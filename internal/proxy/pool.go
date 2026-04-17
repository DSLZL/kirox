package proxy

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// ProxyStatus 代理状态
type ProxyStatus int

const (
	StatusActive      ProxyStatus = iota // 可用
	StatusCooldown                       // 冷却中
	StatusBlacklisted                    // 已拉黑
)

func (s ProxyStatus) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusCooldown:
		return "cooldown"
	case StatusBlacklisted:
		return "blacklisted"
	default:
		return "unknown"
	}
}

// ProxyEntry 代理条目
type ProxyEntry struct {
	Address   string `json:"address" yaml:"address"`
	Protocol  string `json:"protocol" yaml:"protocol,omitempty"`
	Country   string `json:"country" yaml:"country,omitempty"`
	Region    string `json:"region" yaml:"region,omitempty"`
	Continent string `json:"continent" yaml:"continent,omitempty"`
	City      string `json:"city" yaml:"city,omitempty"`
	ISP       string `json:"isp" yaml:"isp,omitempty"`
	IPType    string `json:"ip_type" yaml:"ip_type,omitempty"` // residential / datacenter
	Tags      []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Weight    int    `json:"weight" yaml:"weight,omitempty"` // 1-100, 默认50

	// 运行时状态（不序列化到 YAML）
	Status      ProxyStatus   `json:"status" yaml:"-"`
	CooldownAt  time.Time     `json:"cooldown_at,omitempty" yaml:"-"`
	CooldownDur time.Duration `json:"cooldown_dur,omitempty" yaml:"-"`
	BlacklistAt time.Time     `json:"blacklist_at,omitempty" yaml:"-"`

	// 统计
	TotalUses    int64     `json:"total_uses" yaml:"-"`
	SuccessCount int64     `json:"success_count" yaml:"-"`
	FailCount    int64     `json:"fail_count" yaml:"-"`
	OTP400Count  int64     `json:"otp400_count" yaml:"-"`
	BannedCount  int64     `json:"banned_count" yaml:"-"`
	AvgLatencyMs int64     `json:"avg_latency_ms" yaml:"-"`
	LastUsedAt   time.Time `json:"last_used_at,omitempty" yaml:"-"`
	LastError    string    `json:"last_error,omitempty" yaml:"-"`

	// GeoResolved 地理信息是否已解析
	GeoResolved bool `json:"geo_resolved" yaml:"-"`
}

// ResultType 代理使用结果类型
type ResultType int

const (
	ResultSuccess  ResultType = iota
	ResultOTP400
	ResultBanned
	ResultConnFail
)

// SmartProxyPool 智能代理池
type SmartProxyPool struct {
	mu       sync.RWMutex
	entries  []*ProxyEntry
	policy   *ProxyPolicy
	index    uint32
	onChange func() // 状态变更回调（用于通知前端）
}

// NewSmartProxyPool 创建智能代理池
func NewSmartProxyPool() *SmartProxyPool {
	return &SmartProxyPool{
		entries: make([]*ProxyEntry, 0),
		policy:  DefaultPolicy(),
	}
}

// SetPolicy 设置策略
func (p *SmartProxyPool) SetPolicy(policy *ProxyPolicy) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.policy = policy
}

// GetPolicy 获取策略
func (p *SmartProxyPool) GetPolicy() *ProxyPolicy {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.policy
}

// SetOnChange 设置状态变更回调
func (p *SmartProxyPool) SetOnChange(fn func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onChange = fn
}

// AddEntry 添加代理
func (p *SmartProxyPool) AddEntry(entry *ProxyEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if entry.Weight <= 0 {
		entry.Weight = 50
	}
	if entry.Status == 0 {
		entry.Status = StatusActive
	}
	p.entries = append(p.entries, entry)
}

// AddEntries 批量添加
func (p *SmartProxyPool) AddEntries(entries []*ProxyEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range entries {
		if e.Weight <= 0 {
			e.Weight = 50
		}
		if e.Status == 0 {
			e.Status = StatusActive
		}
		p.entries = append(p.entries, e)
	}
}

// RemoveEntry 移除代理
func (p *SmartProxyPool) RemoveEntry(address string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, e := range p.entries {
		if e.Address == address {
			p.entries = append(p.entries[:i], p.entries[i+1:]...)
			return true
		}
	}
	return false
}

// ClearAll 清空所有
func (p *SmartProxyPool) ClearAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = make([]*ProxyEntry, 0)
}

// GetEntries 获取所有代理（只读副本）
func (p *SmartProxyPool) GetEntries() []*ProxyEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*ProxyEntry, len(p.entries))
	copy(result, p.entries)
	return result
}

// Count 总数
func (p *SmartProxyPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.entries)
}

// ActiveCount 可用数
func (p *SmartProxyPool) ActiveCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count := 0
	now := time.Now()
	for _, e := range p.entries {
		if p.isAvailable(e, now) {
			count++
		}
	}
	return count
}

// Stats 统计
func (p *SmartProxyPool) Stats() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	now := time.Now()
	stats := map[string]int{
		"total":       len(p.entries),
		"active":      0,
		"cooldown":    0,
		"blacklisted": 0,
		"residential": 0,
		"datacenter":  0,
	}
	for _, e := range p.entries {
		switch {
		case e.Status == StatusBlacklisted:
			stats["blacklisted"]++
		case e.Status == StatusCooldown && now.Before(e.CooldownAt.Add(e.CooldownDur)):
			stats["cooldown"]++
		default:
			stats["active"]++
		}
		if e.IPType == "residential" {
			stats["residential"]++
		} else if e.IPType == "datacenter" {
			stats["datacenter"]++
		}
	}
	return stats
}

// Next 根据策略获取下一个可用代理
func (p *SmartProxyPool) Next() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	available := p.getAvailable()
	if len(available) == 0 {
		return ""
	}

	var selected *ProxyEntry
	switch p.policy.SelectionMode {
	case "random":
		selected = available[rand.Intn(len(available))]
	case "weighted":
		selected = p.weightedSelect(available)
	case "least_used":
		selected = p.leastUsedSelect(available)
	default: // roundrobin
		idx := atomic.AddUint32(&p.index, 1)
		selected = available[int(idx-1)%len(available)]
	}

	if selected != nil {
		selected.TotalUses++
		selected.LastUsedAt = time.Now()
		return selected.Address
	}
	return ""
}

// ReportResult 上报代理使用结果
func (p *SmartProxyPool) ReportResult(address string, result ResultType, latencyMs int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry := p.findEntry(address)
	if entry == nil {
		return
	}

	// 更新延迟
	if latencyMs > 0 {
		if entry.AvgLatencyMs == 0 {
			entry.AvgLatencyMs = latencyMs
		} else {
			entry.AvgLatencyMs = (entry.AvgLatencyMs + latencyMs) / 2
		}
	}

	switch result {
	case ResultSuccess:
		entry.SuccessCount++
		entry.LastError = ""

	case ResultOTP400:
		entry.OTP400Count++
		entry.FailCount++
		entry.LastError = "OTP 400"
		p.applyAction(entry, p.policy.OTP400Action, p.policy.OTP400CooldownMin, entry.OTP400Count, int64(p.policy.OTP400MaxRetries))

	case ResultBanned:
		entry.BannedCount++
		entry.FailCount++
		entry.LastError = "账号封禁"
		p.applyAction(entry, p.policy.BanAction, p.policy.BanCooldownMin, entry.BannedCount, int64(p.policy.BanMaxCount))

	case ResultConnFail:
		entry.FailCount++
		entry.LastError = "连接失败"
		p.applyAction(entry, p.policy.ConnFailAction, p.policy.ConnFailCooldownMin, entry.FailCount, int64(p.policy.ConnFailMaxRetries))
	}

	if p.onChange != nil {
		go p.onChange()
	}
}

// ResetStatus 重置单个代理状态
func (p *SmartProxyPool) ResetStatus(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry := p.findEntry(address)
	if entry != nil {
		entry.Status = StatusActive
		entry.CooldownAt = time.Time{}
		entry.CooldownDur = 0
		entry.BlacklistAt = time.Time{}
		entry.OTP400Count = 0
		entry.BannedCount = 0
		entry.FailCount = 0
		entry.LastError = ""
	}
}

// ResetAllStatus 重置所有代理状态
func (p *SmartProxyPool) ResetAllStatus() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.entries {
		e.Status = StatusActive
		e.CooldownAt = time.Time{}
		e.CooldownDur = 0
		e.BlacklistAt = time.Time{}
		e.OTP400Count = 0
		e.BannedCount = 0
		e.FailCount = 0
		e.LastError = ""
	}
}

// UpdateEntry 更新代理地理信息
func (p *SmartProxyPool) UpdateEntryGeo(address, country, region, continent, city, isp, ipType string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry := p.findEntry(address)
	if entry != nil {
		entry.Country = country
		entry.Region = region
		entry.Continent = continent
		entry.City = city
		entry.ISP = isp
		entry.IPType = ipType
		entry.GeoResolved = true
	}
}

// --- 内部方法 ---

func (p *SmartProxyPool) isAvailable(e *ProxyEntry, now time.Time) bool {
	if e.Status == StatusBlacklisted {
		return false
	}
	if e.Status == StatusCooldown {
		if now.Before(e.CooldownAt.Add(e.CooldownDur)) {
			return false
		}
		// 冷却到期，自动恢复
		e.Status = StatusActive
	}

	// 地理过滤
	if len(p.policy.AllowCountries) > 0 && e.Country != "" {
		if !contains(p.policy.AllowCountries, e.Country) {
			return false
		}
	}
	if len(p.policy.BlockCountries) > 0 && e.Country != "" {
		if contains(p.policy.BlockCountries, e.Country) {
			return false
		}
	}
	if len(p.policy.AllowContinents) > 0 && e.Continent != "" {
		if !contains(p.policy.AllowContinents, e.Continent) {
			return false
		}
	}
	if len(p.policy.AllowRegions) > 0 && e.Region != "" {
		if !contains(p.policy.AllowRegions, e.Region) {
			return false
		}
	}
	if len(p.policy.AllowIPTypes) > 0 && e.IPType != "" {
		if !contains(p.policy.AllowIPTypes, e.IPType) {
			return false
		}
	}

	return true
}

func (p *SmartProxyPool) getAvailable() []*ProxyEntry {
	now := time.Now()
	var result []*ProxyEntry
	for _, e := range p.entries {
		if p.isAvailable(e, now) {
			result = append(result, e)
		}
	}
	return result
}

func (p *SmartProxyPool) weightedSelect(available []*ProxyEntry) *ProxyEntry {
	totalWeight := 0
	for _, e := range available {
		totalWeight += e.Weight
	}
	if totalWeight == 0 {
		return available[rand.Intn(len(available))]
	}
	r := rand.Intn(totalWeight)
	for _, e := range available {
		r -= e.Weight
		if r < 0 {
			return e
		}
	}
	return available[len(available)-1]
}

func (p *SmartProxyPool) leastUsedSelect(available []*ProxyEntry) *ProxyEntry {
	min := available[0]
	for _, e := range available[1:] {
		if e.TotalUses < min.TotalUses {
			min = e
		}
	}
	return min
}

func (p *SmartProxyPool) findEntry(address string) *ProxyEntry {
	for _, e := range p.entries {
		if e.Address == address {
			return e
		}
	}
	return nil
}

func (p *SmartProxyPool) applyAction(entry *ProxyEntry, action string, cooldownMin int, count, threshold int64) {
	if threshold > 0 && count < threshold {
		return // 未达阈值
	}
	switch action {
	case "cooldown":
		entry.Status = StatusCooldown
		entry.CooldownAt = time.Now()
		if cooldownMin <= 0 {
			cooldownMin = 10
		}
		entry.CooldownDur = time.Duration(cooldownMin) * time.Minute
	case "blacklist":
		entry.Status = StatusBlacklisted
		entry.BlacklistAt = time.Now()
	// "ignore" 不做操作
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
