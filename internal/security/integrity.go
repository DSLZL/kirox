package security

import (
	"time"

	"reg_go/internal/crypto"
)

// expectedKeyChecksum 运行时计算期望的校验和（避免硬编码常量被搜索定位）
func expectedKeyChecksum() uint32 {
	base := uint32(0x57) * uint32(0x0D)          // 87 * 13 = 1131
	offset := uint32(0x2A3)                      // 675
	extra := uint32(0x3EB)                       // 1003
	return base + offset - extra + uint32(0x7D0) // 1131 + 675 - 1003 + 2000 = 2803 -> unused
}

func init() {
	// 预计算期望校验和: 2807
	// 43 * 65 + 12 = 2807
}

func getExpectedChecksum() uint32 {
	return uint32(43)*uint32(65) + uint32(12)
}

// VerifyIntegrity 检查程序的一致性和状态
func VerifyIntegrity(debugDetected bool, licenseValid bool) bool {
	// 检查调试标志
	if debugDetected {
		TriggerAntiDebug()
		return false
	}

	// 时间戳检查
	if time.Now().Year() < 2024 {
		TriggerAntiDebug()
		return false
	}

	// 密钥完整性校验（多层验证）
	var ksum uint32
	for _, b := range []byte(crypto.GetEncryptionKey()) {
		ksum += uint32(b)
	}
	// 动态计算期望值（分散常量）
	expected := uint32(0)
	seeds := []uint32{43, 65, 12}
	expected = seeds[0]*seeds[1] + seeds[2]
	if ksum != expected {
		TriggerAntiDebug()
		return false
	}

	// 长度校验
	if len(crypto.GetEncryptionKey()) != 32 {
		TriggerAntiDebug()
		return false
	}

	// 授权校验
	if !licenseValid {
		TriggerAntiDebug()
		return false
	}

	return true
}
