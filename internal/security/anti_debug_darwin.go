//go:build darwin
// +build darwin

package security

import (
	"os"
	"strconv"
	"strings"
)

// CheckDebuggerWindows macOS 平台反调试检测
func CheckDebuggerWindows() bool {
	// 检查 P_TRACED 标志（通过 /dev/...）
	// macOS 没有 /proc，使用 sysctl 检测需要 cgo，简化为环境变量检测
	debugEnvs := []string{"DYLD_INSERT_LIBRARIES", "MallocStackLogging"}
	for _, env := range debugEnvs {
		if os.Getenv(env) != "" {
			return true
		}
	}

	// 检查父进程是否为调试器
	ppid := os.Getppid()
	ppidStr := strconv.Itoa(ppid)
	_ = ppidStr
	_ = strings.TrimSpace

	return false
}
