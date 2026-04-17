//go:build darwin
// +build darwin

package main

import (
	"github.com/wailsapp/wails/v2/pkg/options"
)

// getPlatformOptions 返回 macOS 平台特定选项
func getPlatformOptions() []func(*options.App) {
	return nil
}
