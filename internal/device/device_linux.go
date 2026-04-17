//go:build linux
// +build linux

package device

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var cachedDeviceID string

// GetDeviceID 获取设备唯一标识（Linux）
func GetDeviceID() string {
	if cachedDeviceID != "" {
		return cachedDeviceID
	}
	cachedDeviceID = GenerateHardwareID()
	return cachedDeviceID
}

// GenerateHardwareID 基于多因子硬件信息生成稳定的设备标识（Linux）
func GenerateHardwareID() string {
	var factors []string

	// 因子1: /etc/machine-id（systemd 机器标识）
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			factors = append(factors, id)
		}
	}

	// 因子2: /sys/class/dmi/id/product_uuid（DMI 产品 UUID，需要 root）
	if data, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		uuid := strings.TrimSpace(string(data))
		if uuid != "" {
			factors = append(factors, uuid)
		}
	}

	// 因子3: /sys/class/dmi/id/board_serial（主板序列号）
	if data, err := os.ReadFile("/sys/class/dmi/id/board_serial"); err == nil {
		serial := strings.TrimSpace(string(data))
		if serial != "" && serial != "To be filled by O.E.M." {
			factors = append(factors, serial)
		}
	}

	// 因子4: CPU 信息
	if out, err := exec.Command("cat", "/proc/cpuinfo").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					factors = append(factors, strings.TrimSpace(parts[1]))
				}
				break
			}
		}
	}

	// 回退: 主机名 + 用户名
	if len(factors) == 0 {
		hostname, _ := os.Hostname()
		username := os.Getenv("USER")
		factors = append(factors, fmt.Sprintf("%s-%s", hostname, username))
	}

	// SHA-256 组合所有因子
	combined := strings.Join(factors, "|")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:16])
}
