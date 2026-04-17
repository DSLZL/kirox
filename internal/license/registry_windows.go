//go:build windows
// +build windows

package license

import (
	"golang.org/x/sys/windows/registry"
)

// 注册表路径
func getRegistryPath() string {
	return `SOFTWARE\KiroClient`
}

// 注册表键名（混淆或者普通都行）
func getRegistryKey() string {
	return "LicenseData"
}

// SaveToRegistry 保存卡密到注册表
func SaveToRegistry(encryptedData string) error {
	// 会先尝试打开，如果没有则创建
	k, _, err := registry.CreateKey(registry.CURRENT_USER, getRegistryPath(), registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.SetStringValue(getRegistryKey(), encryptedData)
}

// LoadFromRegistry 从注册表加载卡密
func LoadFromRegistry() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, getRegistryPath(), registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()

	data, _, err := k.GetStringValue(getRegistryKey())
	return data, err
}

// DeleteFromRegistry 从注册表删除卡密
func DeleteFromRegistry() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, getRegistryPath(), registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.DeleteValue(getRegistryKey())
}
