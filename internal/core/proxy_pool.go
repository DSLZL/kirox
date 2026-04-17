package core

import (
	"strings"
	"sync"
	"sync/atomic"
)

// ProxyPool 代理池管理器
type ProxyPool struct {
	proxies []string
	index   uint32
	mu      sync.RWMutex
}

// NewProxyPool 创建代理池
// proxyStr 格式: "socks5://127.0.0.1:1080,socks5://127.0.0.1:1081" 或单个代理
func NewProxyPool(proxyStr string) *ProxyPool {
	pool := &ProxyPool{
		proxies: make([]string, 0),
		index:   0,
	}

	if proxyStr == "" {
		return pool
	}

	// 解析代理列表（支持逗号、分号、换行符分隔）
	proxyStr = strings.ReplaceAll(proxyStr, ";", ",")
	proxyStr = strings.ReplaceAll(proxyStr, "\n", ",")
	proxyStr = strings.ReplaceAll(proxyStr, "\r", "")

	parts := strings.Split(proxyStr, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			pool.proxies = append(pool.proxies, p)
		}
	}

	return pool
}

// Next 获取下一个代理（轮询）
func (p *ProxyPool) Next() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.proxies) == 0 {
		return ""
	}

	if len(p.proxies) == 1 {
		return p.proxies[0]
	}

	// 原子递增索引
	idx := atomic.AddUint32(&p.index, 1)
	return p.proxies[int(idx-1)%len(p.proxies)]
}

// Count 返回代理数量
func (p *ProxyPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.proxies)
}

// IsEmpty 检查代理池是否为空
func (p *ProxyPool) IsEmpty() bool {
	return p.Count() == 0
}
