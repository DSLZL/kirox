package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HMACSign 使用会话密钥派生签名密钥，对请求体进行 HMAC-SHA256 签名
func HMACSign(payload []byte, sessionKey []byte) string {
	// 从会话密钥派生签名专用密钥（避免密钥复用）
	h := sha256.Sum256(append(sessionKey, []byte("hmac-signing-key")...))
	mac := hmac.New(sha256.New, h[:])
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
