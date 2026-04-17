//go:build linux
// +build linux

package license

import (
	"os"
	"path/filepath"
)

func getLicenseFilePath() string {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "KiroClient")
	return filepath.Join(configDir, "license.dat")
}

// SaveToRegistry 保存卡密到文件（Linux 使用 XDG 配置目录）
func SaveToRegistry(encryptedData string) error {
	path := getLicenseFilePath()
	os.MkdirAll(filepath.Dir(path), 0700)
	return os.WriteFile(path, []byte(encryptedData), 0600)
}

// LoadFromRegistry 从文件加载卡密（Linux）
func LoadFromRegistry() (string, error) {
	data, err := os.ReadFile(getLicenseFilePath())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeleteFromRegistry 删除卡密文件（Linux）
func DeleteFromRegistry() error {
	return os.Remove(getLicenseFilePath())
}
