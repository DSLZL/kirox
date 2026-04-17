package task

import "reg_go/internal/proxy"

// smartProxyPoolGetter 全局函数，由 main 包设置
var smartProxyPoolGetter func() *proxy.SmartProxyPool

// SetSmartProxyPoolGetter 设置智能代理池获取函数（由 main 包在初始化时调用）
func SetSmartProxyPoolGetter(fn func() *proxy.SmartProxyPool) {
	smartProxyPoolGetter = fn
}

// GetSmartProxyPool 获取智能代理池实例
func GetSmartProxyPool() *proxy.SmartProxyPool {
	if smartProxyPoolGetter != nil {
		return smartProxyPoolGetter()
	}
	return nil
}
