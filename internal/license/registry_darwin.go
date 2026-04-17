//go:build darwin
// +build darwin

package license

import (
	"os"
	"path/filepath"
)

func getLicenseFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "KiroClient", "license.dat")
}

// SaveToRegistry 保存卡密到文件（macOS 使用文件替代注册表）
func SaveToRegistry(encryptedData string) error {
	path := getLicenseFilePath()
	os.MkdirAll(filepath.Dir(path), 0700)
	return os.WriteFile(path, []byte(encryptedData), 0600)
}

// LoadFromRegistry 从文件加载卡密（macOS）
func LoadFromRegistry() (string, error) {
	data, err := os.ReadFile(getLicenseFilePath())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeleteFromRegistry 删除卡密文件（macOS）
func DeleteFromRegistry() error {
	return os.Remove(getLicenseFilePath())
}
