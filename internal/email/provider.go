package email

// EmailProvider 统一邮箱接口
type EmailProvider interface {
	// GetAddress 返回邮箱地址
	GetAddress() string
	// WaitForCode 轮询等待验证码
	WaitForCode(timeout, interval int) (string, error)
}
