package crypto

import (
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"

	"reg_go/internal/device"
)

// localStorageKey 本地存储专用密钥（HKDF 派生，绑定设备）
var localStorageKey []byte

// DeriveLocalKey 使用 HKDF 从主密钥 + 设备 ID 派生本地存储密钥
func DeriveLocalKey() {
	deviceID := device.GetDeviceID()
	salt := sha256.Sum256([]byte(deviceID))
	info := []byte("kiro-local-storage-v1")
	hkdfReader := hkdf.New(sha256.New, []byte(encryptionKey), salt[:], info)
	localStorageKey = make([]byte, 32)
	io.ReadFull(hkdfReader, localStorageKey)
}

// EncryptLocal AES 加密（本地存储用，使用设备绑定密钥）
func EncryptLocal(plaintext string) (string, error) {
	if localStorageKey == nil {
		DeriveLocalKey()
	}
	return AESEncryptWithKey([]byte(plaintext), localStorageKey)
}

// DecryptLocal AES 解密（本地存储用，先尝试 HKDF 密钥，失败回退到静态密钥以兼容旧数据）
func DecryptLocal(ciphertext string) (string, error) {
	if localStorageKey == nil {
		DeriveLocalKey()
	}
	// 优先使用设备绑定密钥
	if result, err := AESDecryptWithKey(ciphertext, localStorageKey); err == nil {
		return result, nil
	}
	// 回退到静态主密钥（兼容旧版本加密的数据）
	return AESDecrypt(ciphertext)
}
