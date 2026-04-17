package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"reg_go/internal/crypto"
	"reg_go/internal/device"
	"reg_go/internal/license"
	"reg_go/internal/session"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// VersionInfo 版本信息
type VersionInfo struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Sha256      string `json:"sha256"`
	ReleaseDate string `json:"release_date"`
	Changelog   string `json:"changelog"`
	Required    bool   `json:"required"`
}

// cachedVersionInfo 缓存服务端返回的版本信息（防止前端伪造下载地址）
var (
	cachedVersionInfo *VersionInfo
	cachedVersionMu   sync.Mutex
)

// isNewerVersion 比较语义化版本号，返回 true 当 remote 严格大于 local
func isNewerVersion(remote, local string) bool {
	parse := func(v string) []int {
		v = strings.TrimPrefix(v, "v")
		parts := strings.Split(v, ".")
		nums := make([]int, 3)
		for i := 0; i < len(parts) && i < 3; i++ {
			nums[i], _ = strconv.Atoi(parts[i])
		}
		return nums
	}
	r := parse(remote)
	l := parse(local)
	for i := 0; i < 3; i++ {
		if r[i] > l[i] {
			return true
		}
		if r[i] < l[i] {
			return false
		}
	}
	return false
}

// GetCurrentVersion 获取当前版本号
func GetCurrentVersion() string {
	return "v2.3.0"
}

// CleanupTemp 清理更新遗留的临时文件
func CleanupTemp() {
	// 清理 kiro-update 临时目录
	tempDir := filepath.Join(os.TempDir(), "kiro-update")
	os.RemoveAll(tempDir)

	// 清理 .backup 文件
	if exePath, err := os.Executable(); err == nil {
		backupPath := exePath + ".backup"
		os.Remove(backupPath)
	}
}

// CheckForUpdatesInBackground 后台延迟检查更新（启动时调用）
func CheckForUpdatesInBackground(ctx context.Context) {
	serverURL := license.GetServerURL()

	// 延迟 2 秒后检查，避免阻塞启动
	time.Sleep(2 * time.Second)

	// 读取卡密配置
	encryptedData, err := license.LoadFromRegistry()
	if err != nil {
		log.Printf("检查更新跳过: 未激活卡密")
		return
	}

	// 解密配置
	decryptedData, err := crypto.DecryptLocal(encryptedData)
	if err != nil {
		log.Printf("检查更新跳过: 卡密配置异常")
		return
	}

	var cfg license.Config
	if err := json.Unmarshal([]byte(decryptedData), &cfg); err != nil {
		log.Printf("检查更新跳过: 卡密配置解析失败")
		return
	}

	deviceID := device.GetDeviceID()

	// 构造请求数据
	reqData := map[string]string{
		"key":       cfg.LicenseKey,
		"device_id": deviceID,
	}
	reqJSON, _ := json.Marshal(reqData)

	// 加密请求
	encrypted, err := crypto.Encrypt(string(reqJSON))
	if err != nil {
		log.Printf("检查更新跳过: 安全会话未建立")
		return
	}

	// 发送加密请求
	payload := map[string]string{"data": encrypted}
	payloadJSON, _ := json.Marshal(payload)

	httpReq, _ := http.NewRequest("POST", serverURL+"/api/version", bytes.NewReader(payloadJSON))
	httpReq.Header.Set("Content-Type", "application/json")
	if sid := session.Manager.GetSessionID(); sid != "" {
		httpReq.Header.Set("X-Session-ID", sid)
	}
	if sk := session.Manager.GetSessionKey(); sk != nil {
		sig := crypto.HMACSign(payloadJSON, sk)
		httpReq.Header.Set("X-Signature", sig)
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		log.Printf("检查更新失败: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			log.Printf("检查更新失败: 授权无效，卡密可能已过期或被封禁")
		} else {
			var errResp map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp["error"] != "" {
				log.Printf("检查更新失败: %s", errResp["error"])
			} else {
				log.Printf("检查更新失败: HTTP %d", resp.StatusCode)
			}
		}
		return
	}

	var encryptedResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&encryptedResp); err != nil {
		log.Printf("检查更新失败: 解析响应失败")
		return
	}

	// 解密响应
	decrypted, err := crypto.Decrypt(encryptedResp["data"])
	if err != nil {
		log.Printf("检查更新失败: 解密响应失败")
		return
	}

	var versionInfo VersionInfo
	if err := json.Unmarshal([]byte(decrypted), &versionInfo); err != nil {
		log.Printf("检查更新失败: 解析版本信息失败")
		return
	}

	currentVersion := GetCurrentVersion()
	if isNewerVersion(versionInfo.Version, currentVersion) {
		log.Printf("发现新版本: %s (当前: %s)", versionInfo.Version, currentVersion)

		// 缓存版本信息（包含安全下载地址和哈希）
		cachedVersionMu.Lock()
		cachedVersionInfo = &versionInfo
		cachedVersionMu.Unlock()

		// 通知前端有新版本（注意：不向前端暴露 downloadURL）
		if ctx != nil {
			wailsRuntime.EventsEmit(ctx, "update-available", map[string]interface{}{
				"currentVersion": currentVersion,
				"latestVersion":  versionInfo.Version,
				"version":        versionInfo.Version,
				"releaseDate":    versionInfo.ReleaseDate,
				"changelog":      versionInfo.Changelog,
				"required":       versionInfo.Required,
			})
		}
	} else {
		log.Printf("当前已是最新版本: %s", currentVersion)
	}
}

// CheckUpdate 手动检查更新
func CheckUpdate() map[string]interface{} {
	serverURL := license.GetServerURL()

	// 读取卡密配置
	encryptedData, err := license.LoadFromRegistry()
	if err != nil {
		return map[string]interface{}{
			"error": "请先激活卡密后再检查更新",
		}
	}

	// 解密配置
	decryptedData, err := crypto.DecryptLocal(encryptedData)
	if err != nil {
		return map[string]interface{}{
			"error": "卡密配置异常，请退出卡密后重新激活",
		}
	}

	var cfg license.Config
	if err := json.Unmarshal([]byte(decryptedData), &cfg); err != nil {
		return map[string]interface{}{
			"error": "卡密配置解析失败，请退出卡密后重新激活",
		}
	}

	deviceID := device.GetDeviceID()

	// 构造请求数据
	reqData := map[string]string{
		"key":       cfg.LicenseKey,
		"device_id": deviceID,
	}
	reqJSON, _ := json.Marshal(reqData)

	// 加密请求
	encrypted, err := crypto.Encrypt(string(reqJSON))
	if err != nil {
		return map[string]interface{}{
			"error": "请先激活卡密后再检查更新",
		}
	}

	// 发送加密请求
	payload := map[string]string{"data": encrypted}
	payloadJSON, _ := json.Marshal(payload)

	httpReq, _ := http.NewRequest("POST", serverURL+"/api/version", bytes.NewReader(payloadJSON))
	httpReq.Header.Set("Content-Type", "application/json")
	if sid := session.Manager.GetSessionID(); sid != "" {
		httpReq.Header.Set("X-Session-ID", sid)
	}
	if sk := session.Manager.GetSessionKey(); sk != nil {
		sig := crypto.HMACSign(payloadJSON, sk)
		httpReq.Header.Set("X-Signature", sig)
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return map[string]interface{}{
			"error": "检查更新失败: " + err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return map[string]interface{}{
				"error": "授权无效，请先验证卡密",
			}
		}
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp["error"] != "" {
			return map[string]interface{}{
				"error": "检查更新失败: " + errResp["error"],
			}
		}
		return map[string]interface{}{
			"error": fmt.Sprintf("检查更新失败: HTTP %d", resp.StatusCode),
		}
	}

	var encryptedResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&encryptedResp); err != nil {
		return map[string]interface{}{
			"error": "解析响应失败",
		}
	}

	// 解密响应
	decrypted, err := crypto.Decrypt(encryptedResp["data"])
	if err != nil {
		return map[string]interface{}{
			"error": "解密响应失败",
		}
	}

	var versionInfo VersionInfo
	if err := json.Unmarshal([]byte(decrypted), &versionInfo); err != nil {
		return map[string]interface{}{
			"error": "解析版本信息失败: " + err.Error(),
		}
	}

	currentVersion := GetCurrentVersion()
	hasUpdate := isNewerVersion(versionInfo.Version, currentVersion)

	// 缓存版本信息（包含安全下载地址和哈希）
	if hasUpdate {
		cachedVersionMu.Lock()
		cachedVersionInfo = &versionInfo
		cachedVersionMu.Unlock()
	}

	// 注意：不向前端暴露 downloadURL 和 sha256
	return map[string]interface{}{
		"hasUpdate":      hasUpdate,
		"currentVersion": currentVersion,
		"latestVersion":  versionInfo.Version,
		"releaseDate":    versionInfo.ReleaseDate,
		"changelog":      versionInfo.Changelog,
		"required":       versionInfo.Required,
	}
}

var (
	downloadCancel context.CancelFunc
	downloadMutex  sync.Mutex
)

// CancelUpdate 取消正在进行的更新下载
func CancelUpdate() map[string]interface{} {
	downloadMutex.Lock()
	defer downloadMutex.Unlock()
	if downloadCancel != nil {
		downloadCancel()
		downloadCancel = nil
		return map[string]interface{}{"success": true, "message": "更新已取消"}
	}
	return map[string]interface{}{"success": false, "message": "没有正在进行的更新"}
}

// DownloadUpdate 下载并安装更新（不接受前端 URL，从缓存的服务端版本信息中获取下载地址）
func DownloadUpdate(ctx context.Context) map[string]interface{} {
	// 从缓存中获取版本信息（不信任前端传入的 URL）
	cachedVersionMu.Lock()
	vInfo := cachedVersionInfo
	cachedVersionMu.Unlock()

	if vInfo == nil || vInfo.DownloadURL == "" {
		return map[string]interface{}{
			"error": "请先检查更新",
		}
	}

	downloadURL := vInfo.DownloadURL
	expectedHash := vInfo.Sha256

	log.Printf("开始下载更新: %s", downloadURL)

	downloadMutex.Lock()
	// Create cancellable context
	dlCtx, cancel := context.WithCancel(ctx)
	if downloadCancel != nil {
		downloadCancel() // 结束旧的
	}
	downloadCancel = cancel
	downloadMutex.Unlock()

	defer func() {
		downloadMutex.Lock()
		if downloadCancel != nil {
			downloadCancel()
			downloadCancel = nil
		}
		downloadMutex.Unlock()
	}()

	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "kiro-update")
	os.MkdirAll(tempDir, 0755)

	// 确保错误时清理临时目录
	updateSuccess := false
	defer func() {
		if !updateSuccess {
			os.RemoveAll(tempDir)
		}
	}()

	// 使用支持 Context 取消的请求
	req, err := http.NewRequestWithContext(dlCtx, "GET", downloadURL, nil)
	if err != nil {
		return map[string]interface{}{"error": "构建请求失败: " + err.Error()}
	}

	// 强制使用 HTTP/1.1，避免 Go HTTP/2 协议下因 Nginx/Cloudflare 大文件分片导致的 stream INTERNAL_ERROR bug
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			ForceAttemptHTTP2: false,
		},
	}

	// 下载文件
	resp, err := client.Do(req)
	if err != nil {
		// 检查是否是被用户取消的
		if errors.Is(err, context.Canceled) {
			return map[string]interface{}{"error": "更新被用户取消"}
		}
		return map[string]interface{}{
			"error": "下载失败: " + err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{
			"error": fmt.Sprintf("下载失败: HTTP %d", resp.StatusCode),
		}
	}

	// 保存到临时文件
	newExeName := "kiro-reg-new"
	if runtime.GOOS == "windows" {
		newExeName += ".exe"
	}
	newExePath := filepath.Join(tempDir, newExeName)
	out, err := os.Create(newExePath)
	if err != nil {
		return map[string]interface{}{
			"error": "创建临时文件失败: " + err.Error(),
		}
	}

	// 下载并显示进度
	totalSize := resp.ContentLength
	downloaded := int64(0)
	buffer := make([]byte, 32*1024) // 32KB 缓冲区
	lastEventTime := time.Now()

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			out.Write(buffer[:n])
			downloaded += int64(n)

			// 发送进度事件（节流，每 100ms 发送一次，避免阻塞前端）
			now := time.Now()
			if ctx != nil && (now.Sub(lastEventTime) > 100*time.Millisecond || downloaded == totalSize) {
				lastEventTime = now
				progress := float64(0)
				if totalSize > 0 {
					progress = float64(downloaded) / float64(totalSize) * 100
				}
				// 注意：前端 task.js 监听的是 update-progress 并且接收三个独立参数
				wailsRuntime.EventsEmit(ctx, "update-progress", progress, downloaded, totalSize)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			out.Close()
			os.Remove(newExePath)
			return map[string]interface{}{
				"error": "下载中断: " + err.Error(),
			}
		}
	}
	out.Close()

	log.Printf("下载完成: %s (%d bytes)", newExePath, downloaded)

	// 验证下载完整性
	if totalSize > 0 && downloaded != totalSize {
		return map[string]interface{}{
			"error": fmt.Sprintf("下载不完整: 期望 %d 字节，实际 %d 字节", totalSize, downloaded),
		}
	}

	// SHA256 完整性校验（防止中间人替换文件）
	if expectedHash != "" {
		fileData, err := os.ReadFile(newExePath)
		if err != nil {
			return map[string]interface{}{
				"error": "读取下载文件失败: " + err.Error(),
			}
		}
		actualHash := sha256.Sum256(fileData)
		actualHashStr := hex.EncodeToString(actualHash[:])
		if !strings.EqualFold(actualHashStr, expectedHash) {
			log.Printf("SHA256 校验失败: 期望 %s, 实际 %s", expectedHash, actualHashStr)
			return map[string]interface{}{
				"error": "文件完整性校验失败，可能已被篡改",
			}
		}
		log.Printf("SHA256 校验通过: %s", actualHashStr)
	}

	// 验证 PE 可执行文件头（MZ 头，作为二次防线）
	if f, err := os.Open(newExePath); err == nil {
		header := make([]byte, 2)
		f.Read(header)
		f.Close()
		if len(header) < 2 || header[0] != 'M' || header[1] != 'Z' {
			return map[string]interface{}{
				"error": "下载的文件不是有效的可执行文件",
			}
		}
	}

	// 获取当前可执行文件路径
	currentExePath, err := os.Executable()
	if err != nil {
		return map[string]interface{}{
			"error": "获取当前程序路径失败: " + err.Error(),
		}
	}
	// 解析符号链接，获取真实路径
	if resolved, err := filepath.EvalSymlinks(currentExePath); err == nil {
		currentExePath = resolved
	}

	log.Printf("当前程序路径: %s", currentExePath)

	// Windows: 使用 bat 脚本在进程退出后替换 exe 并重启
	// 这是 Windows 下唯一可靠的自更新方式
	batContent := fmt.Sprintf(`@echo off
@chcp 65001 >NUL
setlocal
set "TARGET=%s"
set "SOURCE=%s"
set "BACKUP=%s"
set "TEMPDIR=%s"
set "LOGFILE=%%TEMPDIR%%\update.log"

echo [%%TIME%%] Start update >> "%%LOGFILE%%"
echo TARGET=%%TARGET%% >> "%%LOGFILE%%"
echo SOURCE=%%SOURCE%% >> "%%LOGFILE%%"

:: Wait for process to exit (max 30 seconds)
set /a count=0
:waitloop
tasklist /FI "PID eq %d" /NH 2>NUL | find "%d" >NUL
if %%ERRORLEVEL%% == 0 (
    set /a count+=1
    echo [%%TIME%%] Process is still running, waiting... >> "%%LOGFILE%%"
    if %%count%% GEQ 30 goto :forcekill
    timeout /t 1 /nobreak >NUL
    goto :waitloop
)
goto :doupdate

:forcekill
echo [%%TIME%%] Timeout reached, force killing PID %d >> "%%LOGFILE%%"
taskkill /F /PID %d >NUL 2>&1
timeout /t 1 /nobreak >NUL

:doupdate
:: Wait 1 extra second to ensure file handles are released
timeout /t 1 /nobreak >NUL

echo [%%TIME%%] Proceeding to move/copy >> "%%LOGFILE%%"
:: Backup current file
if exist "%%TARGET%%" (
    move /Y "%%TARGET%%" "%%BACKUP%%" >> "%%LOGFILE%%" 2>&1
    echo [%%TIME%%] Move ERRORLEVEL: %%ERRORLEVEL%% >> "%%LOGFILE%%"
)

:: Copy new file
copy /Y "%%SOURCE%%" "%%TARGET%%" >> "%%LOGFILE%%" 2>&1
if %%ERRORLEVEL%% NEQ 0 (
    echo [%%TIME%%] Copy failed with ERRORLEVEL %%ERRORLEVEL%%, restoring backup >> "%%LOGFILE%%"
    :: Restore backup if copy failed
    if exist "%%BACKUP%%" (
        move /Y "%%BACKUP%%" "%%TARGET%%" >> "%%LOGFILE%%" 2>&1
    )
    goto :cleanup
)

echo [%%TIME%%] Copy success, launching target >> "%%LOGFILE%%"
:: Start new program
start "" "%%TARGET%%"

:cleanup
:: Clean up temp files (not TEMPDIR so log is preserved for now)
if exist "%%BACKUP%%" del /F "%%BACKUP%%" >NUL 2>&1
echo [%%TIME%%] Update script finished >> "%%LOGFILE%%"
:: Delete self
(goto) 2>NUL & del "%%~f0"
exit
`,
		currentExePath,
		newExePath,
		currentExePath+".backup",
		tempDir,
		os.Getpid(),
		os.Getpid(),
		os.Getpid(),
		os.Getpid(),
	)

	batPath := filepath.Join(tempDir, "update.bat")
	
	// CMD.exe 对仅含有 LF (\n) 的批处理文件解析存在臭名昭著的吞字 Bug (吞掉行首字符)
	// 必须强制将换行符替换为 Windows 标准的 CRLF (\r\n)
	batContent = strings.ReplaceAll(batContent, "\r", "")
	batContent = strings.ReplaceAll(batContent, "\n", "\r\n")

	if err := os.WriteFile(batPath, []byte(batContent), 0755); err != nil {
		return map[string]interface{}{
			"error": "创建更新脚本失败: " + err.Error(),
		}
	}

	// 启动 bat 脚本（完全隐藏窗口运行，通过 SysProcAttr.HideWindow）
	cmd := exec.Command("cmd.exe", "/C", batPath)
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return map[string]interface{}{
			"error": "启动更新脚本失败: " + err.Error(),
		}
	}

	updateSuccess = true
	log.Printf("更新脚本已启动，程序即将退出")

	// 延迟退出当前程序，让前端有时间显示提示
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()

	return map[string]interface{}{
		"success": true,
		"message": "更新成功，程序即将重启",
	}
}

// CopyFile 复制文件（带 sync 和 close 错误检查）
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		destFile.Close()
		os.Remove(dst)
		return err
	}

	// 确保数据写入磁盘
	if err := destFile.Sync(); err != nil {
		destFile.Close()
		os.Remove(dst)
		return err
	}

	return destFile.Close()
}
