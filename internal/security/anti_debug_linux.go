//go:build linux
// +build linux

package security

import (
	"os"
	"strings"
)

// CheckDebuggerWindows Linux 平台反调试检测
func CheckDebuggerWindows() bool {
	// 检查 /proc/self/status 中的 TracerPid
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "TracerPid:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] != "0" {
				return true
			}
		}
	}

	return false
}
