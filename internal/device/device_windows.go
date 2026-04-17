//go:build windows
// +build windows

package device

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows/registry"
)

var cachedDeviceID string

const registryPath = `SOFTWARE\KiroClient`
const tokenName = "MachineToken"
const obfuscateKey = "KiroDeviceTokenSecureKey2026"

// GetDeviceID 获取设备唯一标识（Windows）
// 从注册表读取混淆后的 Token，防止用户级修改和文件复制
func GetDeviceID() string {
	if cachedDeviceID != "" {
		return cachedDeviceID
	}

	// 1. 尝试从注册表读取（对普通用户不可见）
	if saved := loadFromRegistry(); saved != "" {
		cachedDeviceID = saved
		return cachedDeviceID
	}

	// 2. 如果不存在，使用老算法生成一次
	cachedDeviceID = GenerateHardwareID()

	// 3. 混淆并存入注册表，以后就锁定此凭证
	saveToRegistry(cachedDeviceID)
	return cachedDeviceID
}

func loadFromRegistry() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()

	val, _, err := k.GetStringValue(tokenName)
	if err != nil || val == "" {
		return ""
	}

	// 解除混淆
	decoded, err := hex.DecodeString(val)
	if err != nil {
		return ""
	}
	for i := range decoded {
		decoded[i] ^= obfuscateKey[i%len(obfuscateKey)]
	}
	id := string(decoded)
	if len(id) == 32 && isHex(id) {
		return id
	}
	return ""
}

func saveToRegistry(id string) {
	// 确保键存在
	k, _, err := registry.CreateKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()

	// 混淆: 简单的 XOR 隐藏明文，防止直接篡改
	buf := []byte(id)
	for i := range buf {
		buf[i] ^= obfuscateKey[i%len(obfuscateKey)]
	}
	val := hex.EncodeToString(buf)

	k.SetStringValue(tokenName, val)
}

// isHex 检查字符串是否全为十六进制字符
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// GenerateHardwareID 基于多因子硬件信息生成稳定的设备标识
func GenerateHardwareID() string {
	var factors []string

	// 因子1: MachineGuid 注册表
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE); err == nil {
		if guid, _, err := k.GetStringValue("MachineGuid"); err == nil && guid != "" {
			factors = append(factors, guid)
		}
		k.Close()
	}

	// 因子2: BIOS 序列号 (via WMI)
	if out, err := exec.Command("wmic", "bios", "get", "serialnumber").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) >= 2 {
			serial := strings.TrimSpace(lines[len(lines)-1])
			if serial != "" && serial != "To be filled by O.E.M." {
				factors = append(factors, serial)
			}
		}
	}

	// 因子3: CPU ProcessorId
	if out, err := exec.Command("wmic", "cpu", "get", "processorid").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) >= 2 {
			cpuID := strings.TrimSpace(lines[len(lines)-1])
			if cpuID != "" {
				factors = append(factors, cpuID)
			}
		}
	}

	// 因子4: 主板序列号
	if out, err := exec.Command("wmic", "baseboard", "get", "serialnumber").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) >= 2 {
			boardSerial := strings.TrimSpace(lines[len(lines)-1])
			if boardSerial != "" && boardSerial != "To be filled by O.E.M." {
				factors = append(factors, boardSerial)
			}
		}
	}

	// 回退: 主机名 + 用户名
	if len(factors) == 0 {
		hostname, _ := os.Hostname()
		username := os.Getenv("USERNAME")
		factors = append(factors, fmt.Sprintf("%s-%s", hostname, username))
	}

	// SHA-256 组合所有因子
	combined := strings.Join(factors, "|")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:16]) // 取前 16 字节 = 32 字符 hex
}
