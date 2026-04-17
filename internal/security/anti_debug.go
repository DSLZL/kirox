package security

import (
	"os"
	"time"
)

// AntiDebugCallback 是在触发反调试时执行的回调（比如清理授权）
var AntiDebugCallback = func() {}

// CheckDebugger 检查是否处于调试模式
func CheckDebugger() bool {
	detected := false

	// Windows API 级检测
	if CheckDebuggerWindows() {
		detected = true
	}

	// 检查调试相关环境变量
	envKeys := []string{"GODEBUG", "GORACE", "GOTRACEBACK"}
	for _, key := range envKeys {
		if os.Getenv(key) != "" {
			detected = true
			break
		}
	}

	// 时序检测: 正常执行 < 5ms, 调试器单步 >> 50ms
	t0 := time.Now()
	dummy := 0
	for i := 0; i < 1000; i++ {
		dummy += i * i
	}
	_ = dummy
	if time.Since(t0) > 50*time.Millisecond {
		detected = true
	}

	if detected {
		TriggerAntiDebug() // 同步调用，不用 goroutine
	}

	return detected
}

// TriggerAntiDebug 触发反调试措施
func TriggerAntiDebug() {
	// 执行外部注册的清理回调（如清理注册表）
	if AntiDebugCallback != nil {
		AntiDebugCallback()
	}

	// 随机延迟后退出（避免被识别为固定模式）
	time.Sleep(time.Duration(50+time.Now().UnixNano()%200) * time.Millisecond)
	os.Exit(1)
}
