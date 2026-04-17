package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"reg_go/internal/session"
)

// encryptionKey 加密密钥（混淆存储，init 时还原）
var encryptionKey string

func init() {
	// 运行时多层还原密钥（分段存储 + XOR 解码 + 交错重组）
	g0 := []byte{0x51, 0x53, 0x48, 0x55, 0x17, 0x48, 0x5F, 0x5D}
	g1 := []byte{0x48, 0x57, 0x55, 0x57, 0x51, 0x48, 0x16, 0x00}
	g2 := []byte{0xF3, 0xE2, 0xF5, 0xE4, 0xBD, 0xFB, 0xF5, 0xE9}
	g3 := []byte{0x96, 0x88, 0x89, 0xD9, 0xC2, 0xCF, 0xDE, 0xC8}

	xkeys := []byte{0x3A, 0x65, 0x90, 0xBB}
	groups := [][]byte{g0, g1, g2, g3}

	key := make([]byte, 0, 32)
	for idx, g := range groups {
		for _, b := range g {
			key = append(key, b^xkeys[idx])
		}
	}
	encryptionKey = string(key)
}

// GetEncryptionKey 获取加密密钥（供完整性校验使用）
func GetEncryptionKey() string {
	return encryptionKey
}

// Encrypt AES 加密（网络通信用，带时间戳防重放）
// 使用 ECDH 会话密钥
func Encrypt(plaintext string) (string, error) {
	data := fmt.Sprintf("%d|%s", time.Now().Unix(), plaintext)
	sk := session.Manager.GetSessionKey()
	if sk == nil {
		return "", fmt.Errorf("未建立安全会话，请先连接服务器")
	}
	return AESEncryptWithKey([]byte(data), sk)
}

// Decrypt AES 解密（网络通信用，验证时间戳）
// 使用 ECDH 会话密钥
func Decrypt(ciphertext string) (string, error) {
	sk := session.Manager.GetSessionKey()
	if sk == nil {
		return "", fmt.Errorf("未建立安全会话")
	}
	data, err := AESDecryptWithKey(ciphertext, sk)
	if err != nil {
		return "", err
	}

	idx := strings.Index(data, "|")
	if idx == -1 {
		return "", fmt.Errorf("数据格式错误")
	}

	timestampStr := data[:idx]
	plaintext := data[idx+1:]

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("时间戳格式错误")
	}

	if time.Now().Unix()-timestamp > 300 {
		return "", fmt.Errorf("请求已过期")
	}

	return plaintext, nil
}

// AESEncryptWithKey 使用指定密钥 AES-CFB 加密
func AESEncryptWithKey(plaintextBytes []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintextBytes))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintextBytes)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// AESDecryptWithKey 使用指定密钥 AES-CFB 解密
func AESDecryptWithKey(ciphertext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	if len(ciphertextBytes) < aes.BlockSize {
		return "", fmt.Errorf("密文太短")
	}

	iv := ciphertextBytes[:aes.BlockSize]
	ciphertextBytes = ciphertextBytes[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertextBytes, ciphertextBytes)

	return string(ciphertextBytes), nil
}

// AESEncrypt 底层 AES-CFB 加密（使用静态密钥）
func AESEncrypt(plaintextBytes []byte) (string, error) {
	return AESEncryptWithKey(plaintextBytes, []byte(encryptionKey))
}

// AESDecrypt 底层 AES-CFB 解密（使用静态密钥）
func AESDecrypt(ciphertext string) (string, error) {
	return AESDecryptWithKey(ciphertext, []byte(encryptionKey))
}
