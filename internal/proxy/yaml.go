package proxy

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLConfig YAML 配置文件结构
type YAMLConfig struct {
	Policy  *YAMLPolicy  `yaml:"policy,omitempty"`
	Proxies []YAMLProxy  `yaml:"proxies"`
}

// YAMLPolicy YAML 策略节
type YAMLPolicy struct {
	Selection      string   `yaml:"selection,omitempty"`
	AllowCountries []string `yaml:"allow_countries,omitempty"`
	BlockCountries []string `yaml:"block_countries,omitempty"`
	AllowContinents []string `yaml:"allow_continents,omitempty"`
	AllowRegions   []string `yaml:"allow_regions,omitempty"`
	AllowIPTypes   []string `yaml:"allow_ip_types,omitempty"`

	OTP400 *YAMLActionConfig `yaml:"otp400,omitempty"`
	Ban    *YAMLActionConfig `yaml:"ban,omitempty"`
	ConnFail *YAMLActionConfig `yaml:"conn_fail,omitempty"`
}

// YAMLActionConfig 策略动作配置
type YAMLActionConfig struct {
	Action         string `yaml:"action"`           // cooldown / blacklist / ignore
	CooldownMinutes int   `yaml:"cooldown_minutes,omitempty"`
	MaxRetries     int    `yaml:"max_retries,omitempty"`
	MaxCount       int    `yaml:"max_count,omitempty"`
}

// YAMLProxy YAML 代理条目
type YAMLProxy struct {
	Address   string   `yaml:"address"`
	Server    string   `yaml:"server"`   // 支持 Clash 格式
	Port      int      `yaml:"port"`     // 支持 Clash 格式
	Type      string   `yaml:"type"`     // 支持 Clash 格式 (示例: socks5)
	Username  string   `yaml:"username"` // 支持 Clash 格式
	Password  string   `yaml:"password"` // 支持 Clash 格式
	Name      string   `yaml:"name"`     // 支持 Clash 格式
	
	Country   string   `yaml:"country,omitempty"`
	Region    string   `yaml:"region,omitempty"`
	Continent string   `yaml:"continent,omitempty"`
	City      string   `yaml:"city,omitempty"`
	ISP       string   `yaml:"isp,omitempty"`
	IPType    string   `yaml:"ip_type,omitempty"`
	Weight    int      `yaml:"weight,omitempty"`
	Tags      []string `yaml:"tags,omitempty"`
}

// ParseYAMLFile 解析 YAML 代理配置文件
func ParseYAMLFile(path string) (*YAMLConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return ParseYAML(data)
}

func ParseYAML(data []byte) (*YAMLConfig, error) {
	var config YAMLConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		// 兼容回退：尝试按 Base64 订阅链接 或 纯文本换行提取节点
		text := strings.TrimSpace(string(data))
		// 仅当没有空格时猜测是否由于缺少 Padding 导致 Base64 失败
		pad := len(text) % 4
		if pad > 0 && !strings.Contains(text, " ") {
			text += strings.Repeat("=", 4-pad)
		}

		decoded, decErr := base64.StdEncoding.DecodeString(text)
		if decErr != nil {
			decoded, decErr = base64.URLEncoding.DecodeString(text)
		}
		if decErr == nil {
			text = string(decoded)
		}

		lines := strings.Split(text, "\n")
		var fallbackProxies []YAMLProxy
		idx := 1
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fallbackProxies = append(fallbackProxies, YAMLProxy{
				Address: line,
				Name:    fmt.Sprintf("订阅节点-%d", idx),
			})
			idx++
		}

		if len(fallbackProxies) > 0 {
			config = YAMLConfig{
				Proxies: fallbackProxies,
			}
		} else {
			return nil, fmt.Errorf("YAML 解析失败，同时尝试解析为纯文本/Base64订阅也失败: %w", err)
		}
	}

	if len(config.Proxies) == 0 {
		return nil, fmt.Errorf("未找到代理配置")
	}

	// 验证代理地址
	for i, p := range config.Proxies {
		if p.Address == "" && p.Server != "" && p.Port != 0 {
			protocol := p.Type
			if protocol == "" {
				protocol = "socks5"
			}
			addr := fmt.Sprintf("%s://", protocol)
			if p.Username != "" || p.Password != "" {
				addr += fmt.Sprintf("%s:%s@", p.Username, p.Password)
			}
			addr += fmt.Sprintf("%s:%d", p.Server, p.Port)
			config.Proxies[i].Address = addr
			p.Address = addr
		}

		if p.Address == "" {
			nameHint := p.Name
			if nameHint == "" {
				nameHint = "未知节点"
			}
			return nil, fmt.Errorf("第 %d 个代理地址为空 (节点: %s)", i+1, nameHint)
		}
		// 自动补全协议
		if !strings.Contains(p.Address, "://") {
			config.Proxies[i].Address = "socks5://" + p.Address
		}
	}

	return &config, nil
}

// ToEntries 将 YAML 代理转换为 ProxyEntry
func (c *YAMLConfig) ToEntries() []*ProxyEntry {
	entries := make([]*ProxyEntry, 0, len(c.Proxies))
	for _, yp := range c.Proxies {
		entry := &ProxyEntry{
			Address:   yp.Address,
			Country:   yp.Country,
			Region:    yp.Region,
			Continent: yp.Continent,
			City:      yp.City,
			ISP:       yp.ISP,
			IPType:    yp.IPType,
			Tags:      yp.Tags,
			Weight:    yp.Weight,
			Status:    StatusActive,
		}
		// 从 URL 提取协议
		if idx := strings.Index(entry.Address, "://"); idx > 0 {
			entry.Protocol = entry.Address[:idx]
		}
		if entry.Weight <= 0 {
			entry.Weight = 50
		}
		entries = append(entries, entry)
	}
	return entries
}

// ToPolicy 将 YAML 策略转换为 ProxyPolicy
func (c *YAMLConfig) ToPolicy() *ProxyPolicy {
	policy := DefaultPolicy()
	if c.Policy == nil {
		return policy
	}
	yp := c.Policy

	if yp.Selection != "" {
		policy.SelectionMode = yp.Selection
	}
	if len(yp.AllowCountries) > 0 {
		policy.AllowCountries = yp.AllowCountries
	}
	if len(yp.BlockCountries) > 0 {
		policy.BlockCountries = yp.BlockCountries
	}
	if len(yp.AllowContinents) > 0 {
		policy.AllowContinents = yp.AllowContinents
	}
	if len(yp.AllowRegions) > 0 {
		policy.AllowRegions = yp.AllowRegions
	}
	if len(yp.AllowIPTypes) > 0 {
		policy.AllowIPTypes = yp.AllowIPTypes
	}

	if yp.OTP400 != nil {
		policy.OTP400Action = yp.OTP400.Action
		if yp.OTP400.CooldownMinutes > 0 {
			policy.OTP400CooldownMin = yp.OTP400.CooldownMinutes
		}
		if yp.OTP400.MaxRetries > 0 {
			policy.OTP400MaxRetries = yp.OTP400.MaxRetries
		}
	}
	if yp.Ban != nil {
		policy.BanAction = yp.Ban.Action
		if yp.Ban.CooldownMinutes > 0 {
			policy.BanCooldownMin = yp.Ban.CooldownMinutes
		}
		if yp.Ban.MaxCount > 0 {
			policy.BanMaxCount = yp.Ban.MaxCount
		}
	}
	if yp.ConnFail != nil {
		policy.ConnFailAction = yp.ConnFail.Action
		if yp.ConnFail.CooldownMinutes > 0 {
			policy.ConnFailCooldownMin = yp.ConnFail.CooldownMinutes
		}
		if yp.ConnFail.MaxRetries > 0 {
			policy.ConnFailMaxRetries = yp.ConnFail.MaxRetries
		}
	}

	return policy
}

// ExportYAML 导出为 YAML 格式
func ExportYAML(pool *SmartProxyPool) ([]byte, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	config := YAMLConfig{
		Policy: &YAMLPolicy{
			Selection:       pool.policy.SelectionMode,
			AllowCountries:  pool.policy.AllowCountries,
			BlockCountries:  pool.policy.BlockCountries,
			AllowContinents: pool.policy.AllowContinents,
			AllowRegions:    pool.policy.AllowRegions,
			AllowIPTypes:    pool.policy.AllowIPTypes,
			OTP400: &YAMLActionConfig{
				Action:          pool.policy.OTP400Action,
				CooldownMinutes: pool.policy.OTP400CooldownMin,
				MaxRetries:      pool.policy.OTP400MaxRetries,
			},
			Ban: &YAMLActionConfig{
				Action:          pool.policy.BanAction,
				CooldownMinutes: pool.policy.BanCooldownMin,
				MaxCount:        pool.policy.BanMaxCount,
			},
			ConnFail: &YAMLActionConfig{
				Action:          pool.policy.ConnFailAction,
				CooldownMinutes: pool.policy.ConnFailCooldownMin,
				MaxRetries:      pool.policy.ConnFailMaxRetries,
			},
		},
		Proxies: make([]YAMLProxy, 0, len(pool.entries)),
	}

	for _, e := range pool.entries {
		config.Proxies = append(config.Proxies, YAMLProxy{
			Address:   e.Address,
			Country:   e.Country,
			Region:    e.Region,
			Continent: e.Continent,
			City:      e.City,
			ISP:       e.ISP,
			IPType:    e.IPType,
			Weight:    e.Weight,
			Tags:      e.Tags,
		})
	}

	return yaml.Marshal(&config)
}
