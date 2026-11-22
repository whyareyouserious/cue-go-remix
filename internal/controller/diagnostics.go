package controller

import (
	"fmt"
	"os"
	"time"

	"cue-go-remix/internal/model"
)

func (a *App) RecordUIEvent(event, value string) { a.runtime.RecordUIEvent(event, value) }

// GetStatus 返回平台、任务状态及最近最多 200 条转录的快照，不包含 API Key。
func (a *App) GetStatus() model.Status { return a.runtime.GetStatus() }

// GetRecentErrors 返回近期错误与告警列表快照。
func (a *App) GetRecentErrors() []model.RecentError { return a.runtime.GetRecentErrors() }

// ClearRecentErrors 清空近期错误与告警列表。
func (a *App) ClearRecentErrors() { a.runtime.ClearRecentErrors() }

// OpenLogsDir 在系统文件管理器中打开本地日志目录。
func (a *App) OpenLogsDir() error { return a.runtime.OpenLogsDir() }

// ExportDiagnostics 导出白名单脱敏诊断报告；绝不泄露 API Key、转录或个人资料。
func (a *App) ExportDiagnostics() (string, error) { return a.runtime.ExportDiagnostics() }

// SaveDiagnosticsToFile 弹出系统原生保存对话框并确实将脱敏诊断报告写入文件，写盘成功后返回文件路径。
func (a *App) SaveDiagnosticsToFile() (string, error) {
	reportJSON, err := a.runtime.ExportDiagnostics()
	if err != nil {
		return "", err
	}
	defaultName := fmt.Sprintf("cue-diagnostics-%d.json", time.Now().Unix())
	targetPath, err := a.desktop.ExportDiagnosticPath(defaultName)
	if err != nil || targetPath == "" {
		return "", err
	}
	if err := os.WriteFile(targetPath, []byte(reportJSON), 0600); err != nil {
		return "", fmt.Errorf("写盘失败: %w", err)
	}
	return targetPath, nil
}
