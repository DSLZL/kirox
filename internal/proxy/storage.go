package proxy

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// storageData 持久化数据结构
type storageData struct {
	Policy  *ProxyPolicy   `json:"policy"`
	Entries []*ProxyEntry  `json:"entries"`
}

var storageMu sync.Mutex

// GetStoragePath 获取代理池存储路径
func GetStoragePath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".kiro-reg", "proxy")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "pool.json")
}

// SavePool 保存代理池到磁盘
func SavePool(pool *SmartProxyPool) error {
	storageMu.Lock()
	defer storageMu.Unlock()

	pool.mu.RLock()
	data := storageData{
		Policy:  pool.policy,
		Entries: pool.entries,
	}
	pool.mu.RUnlock()

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	path := GetStoragePath()
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return err
	}

	log.Printf("[代理] 代理池已保存 (%d 个代理)", len(data.Entries))
	return nil
}

// LoadPool 从磁盘加载代理池
func LoadPool() *SmartProxyPool {
	storageMu.Lock()
	defer storageMu.Unlock()

	pool := NewSmartProxyPool()

	path := GetStoragePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[代理] 加载代理池失败: %v", err)
		}
		return pool
	}

	var data storageData
	if err := json.Unmarshal(raw, &data); err != nil {
		log.Printf("[代理] 解析代理池数据失败: %v", err)
		return pool
	}

	if data.Policy != nil {
		pool.policy = data.Policy
	}
	if len(data.Entries) > 0 {
		pool.entries = data.Entries
		log.Printf("[代理] 已加载 %d 个代理", len(data.Entries))
	}

	return pool
}

// SaveYAMLFile 保存 YAML 到指定路径
func SaveYAMLFile(pool *SmartProxyPool, path string) error {
	data, err := ExportYAML(pool)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
