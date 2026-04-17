//go:build windows
// +build windows

package security

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	ntdll                       = syscall.NewLazyDLL("ntdll.dll")
	pIsDebuggerPresent          = kernel32.NewProc("IsDebuggerPresent")
	pCheckRemoteDebuggerPresent = kernel32.NewProc("CheckRemoteDebuggerPresent")
	pOutputDebugStringW         = kernel32.NewProc("OutputDebugStringW")
	pGetTickCount64             = kernel32.NewProc("GetTickCount64")
	pNtQueryInformationProcess  = ntdll.NewProc("NtQueryInformationProcess")
)

// isDebuggerPresentAPI 调用 Windows API IsDebuggerPresent
func isDebuggerPresentAPI() bool {
	ret, _, _ := pIsDebuggerPresent.Call()
	return ret != 0
}

// isRemoteDebuggerPresent 调用 CheckRemoteDebuggerPresent
func isRemoteDebuggerPresent() bool {
	var present bool
	handle, _ := syscall.GetCurrentProcess()
	pCheckRemoteDebuggerPresent.Call(uintptr(handle), uintptr(unsafe.Pointer(&present)))
	return present
}

// isDebugPortSet 通过 NtQueryInformationProcess 检查 DebugPort (ProcessInfoClass=7)
func isDebugPortSet() bool {
	var debugPort uintptr
	handle, _ := syscall.GetCurrentProcess()
	ret, _, _ := pNtQueryInformationProcess.Call(
		uintptr(handle),
		7, // ProcessDebugPort
		uintptr(unsafe.Pointer(&debugPort)),
		unsafe.Sizeof(debugPort),
		0,
	)
	if ret == 0 && debugPort != 0 {
		return true
	}
	return false
}

// timingCheck 通过 GetTickCount64 检测单步调试（执行间隔异常）
func timingCheck() bool {
	var t1, t2 uint64
	r1, _, _ := pGetTickCount64.Call()
	t1 = uint64(r1)
	// 执行一些无害操作消耗时间
	sum := 0
	for i := 0; i < 10000; i++ {
		sum += i
	}
	_ = sum
	r2, _, _ := pGetTickCount64.Call()
	t2 = uint64(r2)
	// 正常执行 < 50ms，单步调试通常 > 500ms
	return (t2 - t1) > 500
}

// CheckDebuggerWindows Windows 平台增强反调试检测
func CheckDebuggerWindows() bool {
	if isDebuggerPresentAPI() {
		return true
	}
	if isRemoteDebuggerPresent() {
		return true
	}
	if isDebugPortSet() {
		return true
	}
	if timingCheck() {
		return true
	}
	return false
}
