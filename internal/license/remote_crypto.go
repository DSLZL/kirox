package license

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"reg_go/internal/crypto"
	"reg_go/internal/session"
)

// apiPath 运行时构建 API 路径（避免明文字符串）
func apiPath(suffix string) string {
	// 分段构建路径
	p := []byte{0x2f, 0x61, 0x70, 0x69, 0x2f}                   // "/api/"
	e := []byte{0x65, 0x6e, 0x63, 0x72, 0x79, 0x70, 0x74, 0x2d} // "encrypt-"
	return string(p) + string(e) + suffix
}

// RemoteCryptoClient 远程加密客户端
type RemoteCryptoClient struct {
	ServerURL  string
	LicenseKey string
	DeviceID   string
}

// NewRemoteCryptoClient 创建远程加密客户端
func NewRemoteCryptoClient(licenseKey, deviceID string) *RemoteCryptoClient {
	return &RemoteCryptoClient{
		ServerURL:  GetServerURL(),
		LicenseKey: licenseKey,
		DeviceID:   deviceID,
	}
}

// EncryptFP 远程指纹加密
func (c *RemoteCryptoClient) EncryptFP(jsonStr string) (string, error) {
	reqData := map[string]interface{}{
		"key":       c.LicenseKey,
		"device_id": c.DeviceID,
		"plaintext": jsonStr,
	}
	return c.callCryptoAPI(apiPath("fp"), reqData)
}

// EncryptJWE 远程 JWE 密码加密
func (c *RemoteCryptoClient) EncryptJWE(password string, publicKey map[string]string, issuer, audience, region string) (string, error) {
	reqData := map[string]interface{}{
		"key":        c.LicenseKey,
		"device_id":  c.DeviceID,
		"password":   password,
		"public_key": publicKey,
		"issuer":     issuer,
		"audience":   audience,
		"region":     region,
	}
	return c.callCryptoAPI(apiPath("jwe"), reqData)
}

// callCryptoAPI 调用服务器加密 API
func (c *RemoteCryptoClient) callCryptoAPI(path string, reqData interface{}) (string, error) {
	reqJSON, _ := json.Marshal(reqData)

	// 使用网络通信加密（带时间戳）
	encrypted, err := crypto.Encrypt(string(reqJSON))
	if err != nil {
		return "", fmt.Errorf("加密请求失败: %w", err)
	}

	payload := map[string]string{"data": encrypted}
	payloadJSON, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("POST", c.ServerURL+path, bytes.NewReader(payloadJSON))
	req.Header.Set("Content-Type", "application/json")
	if sid := session.Manager.GetSessionID(); sid != "" {
		req.Header.Set("X-Session-ID", sid)
	}
	// HMAC 签名（使用会话密钥派生的签名密钥）
	if sk := session.Manager.GetSessionKey(); sk != nil {
		sig := crypto.HMACSign(payloadJSON, sk)
		req.Header.Set("X-Signature", sig)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接服务器失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errResp map[string]string
		json.Unmarshal(body, &errResp)
		return "", fmt.Errorf("服务器错误 %d: %s", resp.StatusCode, errResp["error"])
	}

	var encryptedResp map[string]string
	if err := json.Unmarshal(body, &encryptedResp); err != nil {
		return "", fmt.Errorf("解析响应失败")
	}

	// 使用网络通信解密（验证时间戳）
	decrypted, err := crypto.Decrypt(encryptedResp["data"])
	if err != nil {
		return "", fmt.Errorf("解密响应失败: %w", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(decrypted), &result); err != nil {
		return "", fmt.Errorf("解析结果失败")
	}

	return result["result"], nil
}

// SetupRemoteCrypto 为注册器设置远程加密回调
func SetupRemoteCrypto(licenseKey, deviceID string) (*RemoteCryptoClient, error) {
	if licenseKey == "" || deviceID == "" {
		return nil, fmt.Errorf("缺少卡密或设备ID")
	}
	client := NewRemoteCryptoClient(licenseKey, deviceID)
	log.Printf("加密模块已就绪")
	return client, nil
}
