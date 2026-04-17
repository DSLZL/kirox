//go:build darwin
// +build darwin

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

// GetDeviceID 获取设备唯一标识（macOS）
func GetDeviceID() string {
	if cachedDeviceID != "" {
		return cachedDeviceID
	}
	cachedDeviceID = GenerateHardwareID()
	return cachedDeviceID
}

// GenerateHardwareID 基于多因子硬件信息生成稳定的设备标识（macOS）
func GenerateHardwareID() string {
	var factors []string

	// 因子1: IOPlatformSerialNumber（硬件序列号）
	if out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "IOPlatformSerialNumber") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					serial := strings.Trim(strings.TrimSpace(parts[1]), "\" ")
					if serial != "" {
						factors = append(factors, serial)
					}
				}
			}
		}
	}

	// 因子2: IOPlatformUUID（平台 UUID）
	if out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "IOPlatformUUID") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					uuid := strings.Trim(strings.TrimSpace(parts[1]), "\" ")
					if uuid != "" {
						factors = append(factors, uuid)
					}
				}
			}
		}
	}

	// 因子3: MAC 地址（en0）
	if out, err := exec.Command("ifconfig", "en0").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "ether") {
				mac := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "ether"))
				if mac != "" {
					factors = append(factors, mac)
				}
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
