package main

import (
	"context"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"log"
	"os"
	"path/filepath"
	"reg_go/internal/data"
	"reg_go/internal/email"

	"reg_go/internal/license"
	"reg_go/internal/network"
	"reg_go/internal/security"
	"reg_go/internal/storage"
	"reg_go/internal/task"
	"reg_go/internal/updater"
	"sync/atomic"
	"time"
)

type App struct {
	ctx context.Context

	// 无锁运行状态标记（供 logWriter 使用，避免与 log.mutex 死锁）
	kiroRunning atomic.Bool
}

// NewApp 创建新的 App 实例
func NewApp() *App {
	return &App{}
}

// startup 在应用启动时调用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 重定向日志到内存
	log.SetOutput(&logWriter{app: a})
	log.SetFlags(log.Ltime)

	// 居中显示窗口
	go func() {
		time.Sleep(200 * time.Millisecond)
		runtime.WindowCenter(ctx)
	}()

	// 清理上次更新可能遗留的临时文件
	go updater.CleanupTemp()

	// 不在启动时检查更新，只在卡密验证通过后检查
	security.AntiDebugCallback = func() {
		license.DeleteFromRegistry()
	}
}

// shutdown 在应用关闭时调用
func (a *App) shutdown(ctx context.Context) {
	storage.FlushAccountsSync()
}

// logWriter 自定义日志写入器，根据运行状态路由日志
type logWriter struct {
	app *App
}

func (w *logWriter) Write(p []byte) (int, error) {
	msg := string(p)
	task.Manager.AppendLog(msg)
	return os.Stderr.Write(p)
}

// SetLocalCanvasFingerprint 接收前端 WebView2 采集的本机 canvas 指纹
// 新版本使用纯算法生成指纹，此方法保留向后兼容但不再实际使用
func (a *App) SetLocalCanvasFingerprint(hash int, bins []int) map[string]interface{} {
	return map[string]interface{}{"ok": true}
}

// GetStatus 获取任务状态
func (a *App) GetStatus() map[string]interface{} {
	return task.Manager.GetStatus()
}

// GetLogs 获取日志
func (a *App) GetLogs() []string {
	return task.Manager.GetLogs()
}

// --- UI 绑定接口 (Bindings) ---
// --- bindings_data.go ---

// ExportKiroResults 导出 Kiro 注册结果
func (a *App) ExportKiroResults(format string, selectedEmails []string) map[string]interface{} {
	return data.ExportKiroResults(a.ctx, format, selectedEmails)
}

// ImportKiroResults 导入 Kiro 注册结果
func (a *App) ImportKiroResults() map[string]interface{} {
	return data.ImportKiroResults(a.ctx)
}

// BatchDeleteResults 批量删除注册结果
func (a *App) BatchDeleteResults(emails []string) map[string]interface{} {
	return data.BatchDeleteResults(emails)
}

// ClearKiroResults 一键清空所有 Kiro 注册结果
func (a *App) ClearKiroResults() map[string]interface{} {
	return data.ClearKiroResults()
}

// GetResults 获取结果列表
func (a *App) GetResults(page int, pageSize int, filter string) map[string]interface{} {
	return data.GetResults(page, pageSize, filter)
}

// GetOverview 获取全局概览数据
func (a *App) GetOverview() map[string]interface{} {
	// Kiro 注册结果统计
	kiroTotal, kiroSuccess, kiroFailed, kiroBanned := countKiroResults()

	// Outlook 账号统计
	outlookTotal, outlookRegistered, outlookSuccess, outlookPending := countOutlookAccounts()

	// 当前任务状态
	taskStatus := task.Manager.GetStatus()
	kiroRunning := taskStatus["running"]
	kiroTaskSuccess := taskStatus["success"]
	kiroTaskFailed := taskStatus["failed"]
	kiroTaskCompleted := taskStatus["completed"]
	kiroTaskTotal := taskStatus["total"]

	return map[string]interface{}{
		"version": updater.GetCurrentVersion(),
		"kiro": map[string]interface{}{
			"totalAccounts":   kiroTotal,
			"successAccounts": kiroSuccess,
			"failedAccounts":  kiroFailed,
			"bannedAccounts":  kiroBanned,
			"taskRunning":     kiroRunning,
			"taskSuccess":     kiroTaskSuccess,
			"taskFailed":      kiroTaskFailed,
			"taskCompleted":   kiroTaskCompleted,
			"taskTotal":       kiroTaskTotal,
		},
		"outlook": map[string]interface{}{
			"total":      outlookTotal,
			"registered": outlookRegistered,
			"success":    outlookSuccess,
			"pending":    outlookPending,
		},
	}
}

// GetTaskStatus 获取实时任务状态
func (a *App) GetTaskStatus() map[string]interface{} {
	taskStatus := task.Manager.GetStatus()
	return map[string]interface{}{
		"kiro": map[string]interface{}{
			"taskRunning":   taskStatus["running"],
			"taskSuccess":   taskStatus["success"],
			"taskFailed":    taskStatus["failed"],
			"taskCompleted": taskStatus["completed"],
			"taskTotal":     taskStatus["total"],
		},
	}
}

// countKiroResults 统计 Kiro 注册结果
func countKiroResults() (total, success, failed, banned int) {
	kiroDir := storage.GetKiroDir()

	success = storage.CountEncryptedJSON(filepath.Join(kiroDir, "results.dat"))
	total += success

	failedCount := storage.CountEncryptedJSON(filepath.Join(kiroDir, "failed.dat"))
	failed += failedCount
	total += failedCount

	bannedCount := storage.CountEncryptedJSON(filepath.Join(kiroDir, "banned.dat"))
	banned += bannedCount
	total += bannedCount

	return
}

// countOutlookAccounts 统计 Outlook 账号
func countOutlookAccounts() (total, registered, success, pending int) {
	accounts := storage.GetAccountsCached()
	if len(accounts) == 0 {
		return
	}
	total = len(accounts)
	for _, acc := range accounts {
		reg, _ := acc["registered"].(bool)
		suc, _ := acc["success"].(bool)
		if reg {
			registered++
			if suc {
				success++
			}
		} else {
			pending++
		}
	}
	return
}

// RunHealthCheck 运行账号健康检查
func (a *App) RunHealthCheck(concurrency int) map[string]interface{} {
	return data.RunHealthCheck(concurrency)
}



// --- bindings_license.go ---

// CheckServerHealth 检查验证服务器连通性和延迟
func (a *App) CheckServerHealth() map[string]interface{} {
	return license.Manager.CheckServerHealth()
}

// VerifyLicense 验证卡密
func (a *App) VerifyLicense(licenseKey string) map[string]interface{} {
	return license.Manager.VerifyLicense(licenseKey)
}

// ValidateLicense 仅验证卡密（不消耗设备名额）
func (a *App) ValidateLicense(licenseKey string) map[string]interface{} {
	return license.Manager.ValidateLicense(licenseKey)
}

// CheckLicense 检查本地卡密
func (a *App) CheckLicense() map[string]interface{} {
	return license.Manager.CheckLicense()
}

// GetLicenseInfo 获取卡密详细信息
func (a *App) GetLicenseInfo() map[string]interface{} {
	return license.Manager.GetLicenseInfo()
}

// LogoutLicense 退出卡密（调用服务器自助解绑）
func (a *App) LogoutLicense() map[string]interface{} {
	return license.Manager.LogoutLicense()
}

// --- bindings_mail.go ---

// ---- MoeMail ----

func (a *App) GetMoeMailConfigs() []email.MoeMailConfig {
	return email.GetMoeMailConfigs()
}

func (a *App) SaveMoeMailConfigs(configsJSON string) map[string]interface{} {
	return email.SaveMoeMailConfigs(configsJSON)
}

func (a *App) TestMoeMailConnection(configJSON string) map[string]interface{} {
	return email.TestMoeMailConnection(configJSON)
}

func (a *App) GetMoeMailDomains(configJSON string) map[string]interface{} {
	return email.GetMoeMailDomains(configJSON)
}

// ---- Outlook ----

func (a *App) ParseOutlook(data string) map[string]interface{} {
	return email.ParseOutlook(data)
}

func (a *App) AddOutlookAccounts(data string) map[string]interface{} {
	return email.AddOutlookAccounts(data)
}

func (a *App) GetOutlookAccounts() []map[string]interface{} {
	return email.GetOutlookAccounts()
}

func (a *App) UpdateAccountStatus(em string, registered bool, success bool) map[string]interface{} {
	return email.UpdateAccountStatus(em, registered, success)
}

func (a *App) DeleteOutlookAccount(em string) map[string]interface{} {
	return email.DeleteOutlookAccount(em)
}

func (a *App) ClearOutlookAccounts() map[string]interface{} {
	return email.ClearOutlookAccounts()
}

func (a *App) ImportOutlookFile(filePath string) map[string]interface{} {
	return email.ImportOutlookFile(filePath)
}

// ---- Wails 专用对话框 ----

// SelectDirectory 选择目录 (Wails Dialog)
func (a *App) SelectDirectory() string {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择目录",
	})
	if err != nil {
		log.Printf("选择目录失败: %v", err)
		return ""
	}
	return path
}

// SelectOutlookFile 选择 Outlook 账号文件 (Wails Dialog)
func (a *App) SelectOutlookFile() string {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 Outlook 账号文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "文本文件 (*.txt)",
				Pattern:     "*.txt",
			},
			{
				DisplayName: "CSV 文件 (*.csv)",
				Pattern:     "*.csv",
			},
			{
				DisplayName: "所有文件 (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		log.Printf("选择文件失败: %v", err)
		return ""
	}
	return path
}

// --- bindings_network.go ---

// TestProxy 测试代理连通性和延迟
func (a *App) TestProxy(proxyStr string) map[string]interface{} {
	return network.TestProxy(proxyStr)
}

// --- bindings_storage.go ---

// GetDataDir 前端获取当前存储目录
func (a *App) GetDataDir() string {
	return storage.GetDataDir()
}

// SetDataDir 设置自定义存储目录（自动迁移旧数据）
func (a *App) SetDataDir(dir string) map[string]interface{} {
	path, err := storage.SetDataDirPath(dir)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"success": true, "path": path}
}

// ResetDataDir 重置为默认存储目录
func (a *App) ResetDataDir() map[string]interface{} {
	path := storage.ResetDataDirPath()
	return map[string]interface{}{"success": true, "path": path}
}

// --- bindings_task.go ---

// StartTask 启动注册任务
func (a *App) StartTask(req task.StartTaskRequest) map[string]interface{} {
	return task.StartTask(req)
}

// StopTask 停止注册任务
func (a *App) StopTask() map[string]interface{} {
	return task.StopTask(true)
}

// --- bindings_updater.go ---

// CheckUpdate 手动检查更新
func (a *App) CheckUpdate() map[string]interface{} {
	return updater.CheckUpdate()
}

// DownloadUpdate 下载更新（使用服务端缓存的下载地址，不接受前端参数）
func (a *App) DownloadUpdate() map[string]interface{} {
	return updater.DownloadUpdate(a.ctx)
}

// CancelUpdate 取消正在进行的更新下载
func (a *App) CancelUpdate() map[string]interface{} {
	return updater.CancelUpdate()
}
