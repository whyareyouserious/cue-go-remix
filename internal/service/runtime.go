package service

import (
	"context"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"cue-go-remix/internal/applink"
	"cue-go-remix/internal/listening"
	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/repository"
	"cue-go-remix/internal/whisper"
)

// Runtime 统一持有业务任务状态，具体持久化见 repository.Store。
// mu 保护状态，listenMu 串行化音频启停，captureMu 串行化截图，wg 等待后台退出。
type Runtime struct {
	ctx             context.Context
	cancel          context.CancelFunc
	store           *repository.Store
	models          *whisper.Manager
	link            *applink.Server
	mu              sync.Mutex
	listenMu        sync.Mutex
	captureMu       sync.Mutex
	session         *listening.Session
	startCancel     context.CancelFunc
	sessionCancel   context.CancelFunc
	slideCancel     context.CancelFunc
	requestCancel   context.CancelFunc
	busy            bool
	listening       bool
	closing         bool
	slides          []model.Slide
	warnings        []string
	noting          map[string]bool
	wg              sync.WaitGroup
	diagnostics     *logging.Logger
	slideGeneration uint64
	emitEvent       func(model.Event)
	settingsChanged func(model.Settings)
	launchTime      time.Time
	errorMu         sync.Mutex
	recentErrors    []model.RecentError
}

// Options 接受桌面事件出口，业务模块不依赖 Wails 窗口。
type Options struct {
	// DataDir 配置、会议、模型及日志的本机根目录。
	DataDir string
	// Emit 桌面事件回调，可为 nil；业务层不依赖具体窗口。
	Emit func(model.Event)
	// SettingsChanged 设置保存成功后接收脱敏配置的内部桌面回调，可为 nil。
	SettingsChanged func(model.Settings)
}

// New 按 options 创建存储、模型管理和日志；存储不可读时返回错误，桌面事件通过回调传递。
func New(options Options) (*Runtime, error) {
	s, err := repository.Open(options.DataDir)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &Runtime{
		ctx: ctx, cancel: cancel, store: s,
		slides: []model.Slide{}, warnings: []string{}, noting: map[string]bool{},
		emitEvent: options.Emit, settingsChanged: options.SettingsChanged,
		launchTime: time.Now(), recentErrors: []model.RecentError{},
	}
	a.diagnostics, err = logging.Open(options.DataDir)
	if err != nil {
		a.warnings = append(a.warnings, "诊断日志文件无法创建")
	}
	a.trace("event=app-launch platform=%s", runtime.GOOS)
	a.models = whisper.NewManager(filepath.Join(options.DataDir, "models"), a.emit)
	return a, nil
}

// trace 将不含敏感值的 format 和 args 转发到业务诊断日志。
func (a *Runtime) trace(format string, args ...any) { a.diagnostics.Trace(format, args...) }

// Trace 将固定格式的诊断事件写入日志；format 和 args 不得包含用户正文或密钥。
func (a *Runtime) Trace(format string, args ...any) { a.trace(format, args...) }

// Settings 返回内部完整设置，仅供桌面组装使用，不绑定到前端。
func (a *Runtime) Settings() model.Settings { return a.store.Settings() }

// DataDir 返回本机配置与日志主目录。
func (a *Runtime) DataDir() string { return a.store.Dir() }

// UptimeSeconds 返回应用启动以来的运行秒数。
func (a *Runtime) UptimeSeconds() int64 { return int64(time.Since(a.launchTime).Seconds()) }

// SaveShortcuts 持久化快捷键映射并向桌面发送更新通知。
func (a *Runtime) SaveShortcuts(shortcuts map[string]string) (model.Settings, error) {
	v, err := a.store.SaveShortcuts(shortcuts)
	if err != nil {
		return v, err
	}
	if a.settingsChanged != nil {
		a.settingsChanged(v)
	}
	return v, nil
}

// SavePosition 将 x、y 作为窗口逻辑像素坐标持久化；写入失败回滚内存中的位置设置。
func (a *Runtime) SavePosition(x, y int) error { return a.store.SavePosition(x, y) }

// Shutdown 取消初始化、回答、音频及模型任务并等待后台退出，最后关闭诊断日志。
func (a *Runtime) Shutdown() {
	a.trace("event=shutdown-start")
	a.mu.Lock()
	a.closing = true
	if a.startCancel != nil {
		a.startCancel()
	}
	a.mu.Unlock()
	a.Cancel()
	_ = a.StopListening()
	a.cancel()
	if a.link != nil {
		a.link.Close()
	}
	a.models.Close()
	a.wg.Wait()
	a.trace("event=shutdown-done")
	if a.diagnostics != nil {
		_ = a.diagnostics.Close()
	}
}

// Start 启动本地助手和空闲会议巡检，并补齐已结束会议的纪要；不自动开始录音。
func (a *Runtime) Start() {
	a.trace("event=app-ready")
	a.link = applink.New(a.store.Dir(), applink.Dependencies{Emit: a.emit, Status: a.GetStatus, SetCapturing: func(active bool) error {
		if active {
			return a.StartListening()
		}
		return a.StopListening()
	}, Slides: func() []model.Slide {
		return a.GetStatus().Slides
	}})
	if e := a.link.Start(); e != nil {
		a.warn("本地助手连接启动失败：" + e.Error())
	}
	// 启动时补齐已结束会议的纪要，与无卡密基线一致。
	for _, meeting := range a.store.Meetings() {
		if meeting.EndedAt != 0 && (meeting.Notes == "" || meeting.NotesTurns < len(meeting.Turns)) {
			a.makeNotes(meeting)
		}
	}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-t.C:
				if m := a.store.Current(); m != nil && len(m.Turns) > 0 && time.Now().UnixMilli()-m.Turns[len(m.Turns)-1].Time > 30*time.Minute.Milliseconds() {
					closed, e := a.store.End()
					a.report(e)
					if closed != nil {
						a.makeNotes(*closed)
					}
					a.status()
				}
			}
		}
	}()
}
