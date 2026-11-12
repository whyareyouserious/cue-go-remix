package service

import (
	"context"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"cue-go-remix/internal/applink"
	"cue-go-remix/internal/entitlement"
	"cue-go-remix/internal/listening"
	"cue-go-remix/internal/logging"
	"cue-go-remix/internal/model"
	"cue-go-remix/internal/repository"
	"cue-go-remix/internal/whisper"
)

type Runtime struct {
	license         *entitlement.Manager
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
	sessionLicense  context.Context
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
	notesResumed    bool
}

// Options 接受桌面事件出口，业务模块不依赖 Wails 窗口。
type Options struct {
	DataDir         string
	LicenseURL      string
	AllowHTTP       bool
	Emit            func(model.Event)
	SettingsChanged func(model.Settings)
}

func New(options Options) (*Runtime, error) {
	s, err := repository.Open(options.DataDir)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &Runtime{ctx: ctx, cancel: cancel, store: s, slides: []model.Slide{}, warnings: []string{}, noting: map[string]bool{}, emitEvent: options.Emit, settingsChanged: options.SettingsChanged}
	a.diagnostics, err = logging.Open(options.DataDir)
	if err != nil {
		a.warnings = append(a.warnings, "诊断日志文件无法创建")
	}
	a.trace("event=app-launch platform=%s", runtime.GOOS)
	a.models = whisper.NewManager(filepath.Join(options.DataDir, "models"), a.emit)
	a.license = entitlement.New(ctx, entitlement.Options{BaseURL: options.LicenseURL, AllowHTTP: options.AllowHTTP, Changed: a.licenseChanged, Trace: a.trace})
	return a, nil
}
func (a *Runtime) trace(format string, args ...any) { a.diagnostics.Trace(format, args...) }
func (a *Runtime) Trace(format string, args ...any) { a.trace(format, args...) }
func (a *Runtime) Settings() model.Settings         { return a.store.Settings() }
func (a *Runtime) SavePosition(x, y int) error      { return a.store.SavePosition(x, y) }

func (a *Runtime) Shutdown() {
	a.trace("event=shutdown-start")
	a.mu.Lock()
	a.closing = true
	if a.startCancel != nil {
		a.startCancel()
	}
	a.mu.Unlock()
	if a.license != nil {
		a.license.Close()
	}
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
func (a *Runtime) Start() {
	a.trace("event=app-ready")
	a.license.Start()
	a.link = applink.New(a.store.Dir(), applink.Dependencies{Emit: a.emit, Status: a.GetStatus, SetCapturing: func(active bool) error {
		if active {
			return a.StartListening()
		}
		return a.StopListening()
	}, Slides: func() []model.Slide {
		if _, err := a.authorizedContext(); err != nil {
			return nil
		}
		return a.GetStatus().Slides
	}})
	if e := a.link.Start(); e != nil {
		a.warn("本地助手连接启动失败：" + e.Error())
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
