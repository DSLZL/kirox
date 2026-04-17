package proxy

// ProxyPolicy 代理策略配置
type ProxyPolicy struct {
	// 选择模式: roundrobin / random / weighted / least_used
	SelectionMode string `json:"selection_mode" yaml:"selection"`

	// 地理过滤
	AllowCountries  []string `json:"allow_countries,omitempty" yaml:"allow_countries,omitempty"`
	BlockCountries  []string `json:"block_countries,omitempty" yaml:"block_countries,omitempty"`
	AllowContinents []string `json:"allow_continents,omitempty" yaml:"allow_continents,omitempty"`
	AllowRegions    []string `json:"allow_regions,omitempty" yaml:"allow_regions,omitempty"`
	AllowIPTypes    []string `json:"allow_ip_types,omitempty" yaml:"allow_ip_types,omitempty"`

	// OTP 400 策略
	OTP400RetryMode   string `json:"otp400_retry_mode" yaml:"otp400_retry_mode"`     // fuse(立即熔断) / switch_ip(切换IP重试)
	OTP400Action      string `json:"otp400_action" yaml:"otp400_action"`             // cooldown / blacklist / ignore
	OTP400CooldownMin int    `json:"otp400_cooldown_min" yaml:"otp400_cooldown_min"` // 冷却时长(分钟)
	OTP400MaxRetries  int    `json:"otp400_max_retries" yaml:"otp400_max_retries"`   // 触发阈值

	// 封号策略
	BanAction      string `json:"ban_action" yaml:"ban_action"`
	BanCooldownMin int    `json:"ban_cooldown_min" yaml:"ban_cooldown_min"`
	BanMaxCount    int    `json:"ban_max_count" yaml:"ban_max_count"`

	// 连接失败策略
	ConnFailAction      string `json:"conn_fail_action" yaml:"conn_fail_action"`
	ConnFailCooldownMin int    `json:"conn_fail_cooldown_min" yaml:"conn_fail_cooldown_min"`
	ConnFailMaxRetries  int    `json:"conn_fail_max_retries" yaml:"conn_fail_max_retries"`

	// 自动恢复
	AutoRecoverEnabled bool `json:"auto_recover" yaml:"auto_recover"`
	BlacklistPermanent bool `json:"blacklist_permanent" yaml:"blacklist_permanent"`
}

// DefaultPolicy 默认策略
func DefaultPolicy() *ProxyPolicy {
	return &ProxyPolicy{
		SelectionMode:       "roundrobin",
		OTP400RetryMode:     "fuse",
		OTP400Action:        "cooldown",
		OTP400CooldownMin:   30,
		OTP400MaxRetries:    2,
		BanAction:           "blacklist",
		BanCooldownMin:      60,
		BanMaxCount:         1,
		ConnFailAction:      "cooldown",
		ConnFailCooldownMin: 5,
		ConnFailMaxRetries:  5,
		AutoRecoverEnabled:  true,
		BlacklistPermanent:  false,
	}
}
