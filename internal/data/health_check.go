package data

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"reg_go/internal/storage"
)

// RunHealthCheck 运行账号健康检查
func RunHealthCheck(concurrency int) map[string]interface{} {
	accounts, err := storage.LoadEncryptedJSON(filepath.Join(storage.GetKiroDir(), "results.dat"))
	if err != nil || len(accounts) == 0 {
		return map[string]interface{}{
			"error": "没有可检查的账号",
		}
	}

	if concurrency <= 0 {
		concurrency = 5
	}

	total := len(accounts)
	healthy := 0
	unhealthy := 0

	// 使用信号量控制并发
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := range accounts {
		wg.Add(1)
		go func(acc map[string]interface{}) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 检查账号健康状态
			isHealthy := checkAccountHealth(acc)

			mu.Lock()
			if isHealthy {
				healthy++
				// 如果之前被标记为封号，现在恢复正常
				if banned, _ := acc["banned"].(bool); banned {
					acc["banned"] = false
					delete(acc, "bannedAt")
				}
			} else {
				unhealthy++
				// 标记为封号
				acc["banned"] = true
				acc["bannedAt"] = time.Now().Format("2006-01-02 15:04:05")
			}
			// 记录检查时间
			acc["lastHealthCheck"] = time.Now().Format("2006-01-02 15:04:05")
			mu.Unlock()
		}(accounts[i])
	}

	wg.Wait()

	// 保存更新后的账号数据
	storage.SaveEncryptedJSON(filepath.Join(storage.GetKiroDir(), "results.dat"), accounts)

	return map[string]interface{}{
		"total":     total,
		"healthy":   healthy,
		"unhealthy": unhealthy,
	}
}

// checkAccountHealth 检查单个账号的健康状态
func checkAccountHealth(acc map[string]interface{}) bool {
	clientID, _ := acc["clientId"].(string)
	clientSecret, _ := acc["clientSecret"].(string)
	refreshToken, _ := acc["refreshToken"].(string)

	if clientID == "" || clientSecret == "" || refreshToken == "" {
		return false
	}

	// 使用 refresh token 换取新的 access token
	payload := map[string]interface{}{
		"grant_type":    "refresh_token",
		"client_id":     clientID,
		"refresh_token": refreshToken,
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.amazonalexa.com/auth/o2/token", bytes.NewBuffer(jsonData))
	if err != nil {
		return false
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 检查响应
	if resp.StatusCode == 200 {
		var result map[string]interface{}
		if json.Unmarshal(body, &result) == nil {
			if _, ok := result["access_token"]; ok {
				return true
			}
		}
	}

	return false
}
