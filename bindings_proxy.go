package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"reg_go/internal/network"
	"reg_go/internal/proxy"
	"reg_go/internal/task"
)

// 全局智能代理池实例
var smartProxyPool *proxy.SmartProxyPool

func init() {
	smartProxyPool = proxy.LoadPool()
	// 注册到 task 包，使 coordinator 可以使用
	task.SetSmartProxyPoolGetter(func() *proxy.SmartProxyPool {
		return smartProxyPool
	})
}

// GetSmartProxyPool 获取全局智能代理池
func GetSmartProxyPool() *proxy.SmartProxyPool {
	return smartProxyPool
}

// --- Wails 绑定方法 ---

// GetProxyPool 获取代理池状态
func (a *App) GetProxyPool() map[string]interface{} {
	entries := smartProxyPool.GetEntries()
	stats := smartProxyPool.Stats()

	// 转换为前端友好的格式
	list := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		item := map[string]interface{}{
			"address":       e.Address,
			"protocol":      e.Protocol,
			"country":       e.Country,
			"region":        e.Region,
			"continent":     e.Continent,
			"city":          e.City,
			"isp":           e.ISP,
			"ip_type":       e.IPType,
			"tags":          e.Tags,
			"weight":        e.Weight,
			"status":        e.Status.String(),
			"total_uses":    e.TotalUses,
			"success_count": e.SuccessCount,
			"fail_count":    e.FailCount,
			"otp400_count":  e.OTP400Count,
			"banned_count":  e.BannedCount,
			"avg_latency_ms": e.AvgLatencyMs,
			"last_error":    e.LastError,
			"geo_resolved":  e.GeoResolved,
		}
		if !e.LastUsedAt.IsZero() {
			item["last_used"] = e.LastUsedAt.Format("15:04:05")
		}
		if !e.CooldownAt.IsZero() && e.Status == proxy.StatusCooldown {
			remaining := e.CooldownAt.Add(e.CooldownDur).Sub(e.LastUsedAt).Minutes()
			item["cooldown_remaining_min"] = int(remaining)
		}
		list = append(list, item)
	}

	return map[string]interface{}{
		"entries": list,
		"stats":   stats,
	}
}

// GetProxyPolicy 获取策略配置
func (a *App) GetProxyPolicy() *proxy.ProxyPolicy {
	return smartProxyPool.GetPolicy()
}

// UpdateProxyPolicy 更新策略配置
func (a *App) UpdateProxyPolicy(policy proxy.ProxyPolicy) map[string]interface{} {
	smartProxyPool.SetPolicy(&policy)
	proxy.SavePool(smartProxyPool)
	return map[string]interface{}{"success": true}
}

// ImportProxyYAML 选择并导入 YAML 配置
func (a *App) ImportProxyYAML() map[string]interface{} {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "导入代理配置",
		Filters: []runtime.FileFilter{
			{DisplayName: "YAML 文件", Pattern: "*.yaml;*.yml"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil || path == "" {
		return map[string]interface{}{"error": "未选择文件"}
	}

	config, err := proxy.ParseYAMLFile(path)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	entries := config.ToEntries()
	policy := config.ToPolicy()

	smartProxyPool.SetPolicy(policy)
	smartProxyPool.AddEntries(entries)
	proxy.SavePool(smartProxyPool)

	// 异步解析地理信息
	go func() {
		proxy.ResolveGeoAsync(smartProxyPool)
		proxy.SavePool(smartProxyPool)
	}()

	log.Printf("[代理] 导入成功: %d 个代理", len(entries))
	return map[string]interface{}{
		"success": true,
		"count":   len(entries),
	}
}

// ImportProxyYAMLFromURL 从 URL 选择并导入 YAML 配置
func (a *App) ImportProxyYAMLFromURL(url string) map[string]interface{} {
	if url == "" {
		return map[string]interface{}{"error": "URL 不能为空"}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("获取配置失败: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{"error": fmt.Sprintf("HTTP 状态异常: %d", resp.StatusCode)}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("读取内容失败: %v", err)}
	}

	config, err := proxy.ParseYAML(data)
	if err != nil {
		// 解析失败，先保存下载的原始文件到应用数据文件夹
		savePath := os.Getenv("USERPROFILE")
		if savePath == "" {
			savePath = os.Getenv("HOME")
		}
		savePath = savePath + "/.kiro-reg/proxy/downloaded_url_proxies.yaml"
		os.MkdirAll(savePath[:len(savePath)-28], 0755) // ~/.kiro-reg/proxy

		// 将抓取到的数据写入文件
		if writeErr := os.WriteFile(savePath, data, 0644); writeErr == nil {
			// 根据用户要求，保存后再尝试以文件方式解析
			config, err = proxy.ParseYAMLFile(savePath)
			if err != nil {
				return map[string]interface{}{"error": fmt.Sprintf("已下载至 %s 但解析失败: %v", savePath, err)}
			}
		} else {
			return map[string]interface{}{"error": fmt.Sprintf("YAML 解析失败且无法保存文件: %v", err)}
		}
	}

	entries := config.ToEntries()
	policy := config.ToPolicy()

	smartProxyPool.SetPolicy(policy)
	smartProxyPool.AddEntries(entries)
	proxy.SavePool(smartProxyPool)

	// 异步解析地理信息
	go func() {
		proxy.ResolveGeoAsync(smartProxyPool)
		proxy.SavePool(smartProxyPool)
	}()

	log.Printf("[代理] 从 URL 导入成功: %d 个代理", len(entries))
	return map[string]interface{}{
		"success": true,
		"count":   len(entries),
	}
}

// AddProxyManual 手动添加代理
func (a *App) AddProxyManual(address string) map[string]interface{} {
	if address == "" {
		return map[string]interface{}{"error": "地址不能为空"}
	}
	entry := &proxy.ProxyEntry{
		Address: address,
		Weight:  50,
		Status:  proxy.StatusActive,
	}
	smartProxyPool.AddEntry(entry)
	proxy.SavePool(smartProxyPool)

	// 异步解析地理信息
	go func() {
		proxy.ResolveGeoAsync(smartProxyPool)
		proxy.SavePool(smartProxyPool)
	}()

	return map[string]interface{}{"success": true}
}

// RemoveProxy 移除代理
func (a *App) RemoveProxy(address string) map[string]interface{} {
	if smartProxyPool.RemoveEntry(address) {
		proxy.SavePool(smartProxyPool)
		return map[string]interface{}{"success": true}
	}
	return map[string]interface{}{"error": "代理不存在"}
}

// ResetProxyStatus 重置单个代理状态
func (a *App) ResetProxyStatus(address string) map[string]interface{} {
	smartProxyPool.ResetStatus(address)
	proxy.SavePool(smartProxyPool)
	return map[string]interface{}{"success": true}
}

// ResetAllProxyStatus 重置所有代理状态
func (a *App) ResetAllProxyStatus() map[string]interface{} {
	smartProxyPool.ResetAllStatus()
	proxy.SavePool(smartProxyPool)
	return map[string]interface{}{"success": true}
}

// ClearProxyPool 清空代理池
func (a *App) ClearProxyPool() map[string]interface{} {
	smartProxyPool.ClearAll()
	proxy.SavePool(smartProxyPool)
	return map[string]interface{}{"success": true}
}

// ExportProxyYAML 导出 YAML 配置
func (a *App) ExportProxyYAML() map[string]interface{} {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出代理配置",
		DefaultFilename: "proxies.yaml",
		Filters: []runtime.FileFilter{
			{DisplayName: "YAML 文件", Pattern: "*.yaml;*.yml"},
		},
	})
	if err != nil || path == "" {
		return map[string]interface{}{"error": "未选择路径"}
	}

	data, err := proxy.ExportYAML(smartProxyPool)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	return map[string]interface{}{"success": true, "path": path}
}

// parseTestResult 从 network.TestProxy 的结果中提取第一个代理的成败和延迟
func parseTestResult(result map[string]interface{}) (ok bool, latencyMs int64) {
	results, _ := result["results"].([]map[string]interface{})
	if len(results) == 0 {
		return false, 0
	}
	r := results[0]
	if s, _ := r["success"].(bool); s {
		lat, _ := r["latency"].(int64)
		return true, lat
	}
	return false, 0
}

// BatchTestProxies 批量测试代理（后台逐个测试并写回结果）
func (a *App) BatchTestProxies() map[string]interface{} {
	entries := smartProxyPool.GetEntries()
	if len(entries) == 0 {
		return map[string]interface{}{"error": "代理池为空"}
	}

	go func() {
		// 先解析地理信息
		proxy.ResolveGeoAsync(smartProxyPool)

		// 逐个测试连通性并写回结果
		for _, e := range entries {
			result := network.TestProxy(e.Address)
			ok, lat := parseTestResult(result)
			if ok {
				smartProxyPool.ReportResult(e.Address, proxy.ResultSuccess, lat)
			} else {
				smartProxyPool.ReportResult(e.Address, proxy.ResultConnFail, 0)
			}
		}

		proxy.SavePool(smartProxyPool)
	}()

	return map[string]interface{}{
		"success": true,
		"message": "正在后台测试...",
		"count":   len(entries),
	}
}

// TestPoolProxy 测试单个代理并将结果写回代理池
func (a *App) TestPoolProxy(address string) map[string]interface{} {
	result := network.TestProxy(address)

	ok, lat := parseTestResult(result)
	if ok {
		smartProxyPool.ReportResult(address, proxy.ResultSuccess, lat)
	} else {
		smartProxyPool.ReportResult(address, proxy.ResultConnFail, 0)
	}

	return result
}


