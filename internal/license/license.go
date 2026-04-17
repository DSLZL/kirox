package license

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"reg_go/internal/crypto"
	"reg_go/internal/device"
	"reg_go/internal/session"
)

type Config struct {
	ServerURL     string `json:"server_url"`
	DeviceID      string `json:"device_id"`
	LicenseKey    string `json:"license_key"`
	LastValidated int64  `json:"last_validated"`
}

var (
	Manager *LicenseManager
)

func init() {
	Manager = &LicenseManager{}
}

type LicenseManager struct {
	IsValid       bool
	OnValidated   func() // 验证通过后的回调（例如触发更新检查）
	cachedServerURL string
}

func (m *LicenseManager) SetValid(valid bool) {
	m.IsValid = valid
}

// GetServerURL -> M.GetServerURL()
func GetServerURL() string {
	if Manager.cachedServerURL != "" {
		return Manager.cachedServerURL
	}

	if url := resolveServerFromDNS(); url != "" {
		Manager.cachedServerURL = url
		return url
	}

	enc := []byte{
		0x32, 0x2e, 0x2e, 0x2a, 0x29, 0x60, 0x75, 0x75,
		0x28, 0x3f, 0x3d, 0x74, 0x23, 0x33, 0x34,
		0x22, 0x32, 0x74, 0x3c, 0x2f, 0x34,
	}
	out := make([]byte, len(enc))
	for i, b := range enc {
		out[i] = b ^ 0x5A
	}
	Manager.cachedServerURL = string(out)
	return Manager.cachedServerURL
}

func resolveServerFromDNS() string {
	dnsEnc := []byte{
		0x81, 0xb0, 0xb6, 0xad, 0xbe, 0x84, 0xad, 0xb2,
		0xb4, 0x8c, 0xae, 0xb6, 0xb7, 0xb5, 0xb5, 0x8c,
		0xb3, 0xa2, 0xb7,
	}
	dnsName := make([]byte, len(dnsEnc))
	for i, b := range dnsEnc {
		dnsName[i] = b ^ byte(0xC0+i%16)
	}

	records, err := net.LookupTXT(string(dnsName))
	if err != nil {
		return ""
	}
	for _, r := range records {
		if strings.HasPrefix(r, "server=") {
			return strings.TrimPrefix(r, "server=")
		}
	}
	return ""
}

// CheckServerHealth 检查连通性
func (m *LicenseManager) CheckServerHealth() map[string]interface{} {
	serverURL := GetServerURL()
	client := &http.Client{Timeout: 5 * time.Second}

	start := time.Now()
	resp, err := client.Get(serverURL + "/api/health")
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return map[string]interface{}{
			"online":  false,
			"latency": 0,
			"message": "无法连接验证服务器，请检查网络连接",
		}
	}
	defer resp.Body.Close()

	return map[string]interface{}{
		"online":  true,
		"latency": latency,
		"message": "",
	}
}

func (m *LicenseManager) ValidateLicense(licenseKey string) map[string]interface{} {
	serverURL := GetServerURL()

	if !session.Manager.HasSession() {
		if err := session.Manager.Handshake(serverURL); err != nil {
			log.Printf("[安全] ECDH 握手失败: %v", err)
			return map[string]interface{}{"success": false, "message": "无法建立安全会话: " + err.Error()}
		}
	}

	deviceID := device.GetDeviceID()

	reqData := map[string]string{
		"key":       licenseKey,
		"device_id": deviceID,
	}
	reqJSON, _ := json.Marshal(reqData)

	encrypted, err := crypto.Encrypt(string(reqJSON))
	if err != nil {
		return map[string]interface{}{"success": false, "message": "加密失败: " + err.Error()}
	}

	payload := map[string]string{"data": encrypted}
	payloadJSON, _ := json.Marshal(payload)

	httpReq, _ := http.NewRequest("POST", serverURL+"/api/validate", bytes.NewReader(payloadJSON))
	httpReq.Header.Set("Content-Type", "application/json")
	if sid := session.Manager.GetSessionID(); sid != "" {
		httpReq.Header.Set("X-Session-ID", sid)
	}
	if sk := session.Manager.GetSessionKey(); sk != nil {
		sig := crypto.HMACSign(payloadJSON, sk)
		httpReq.Header.Set("X-Signature", sig)
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "连接服务器失败: " + err.Error()}
	}
	defer resp.Body.Close()

	var encryptedResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&encryptedResp); err != nil {
		return map[string]interface{}{"success": false, "message": "解析响应失败"}
	}

	decrypted, err := crypto.Decrypt(encryptedResp["data"])
	if err != nil {
		return map[string]interface{}{"success": false, "message": "解密失败: " + err.Error()}
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(decrypted), &result); err != nil {
		return map[string]interface{}{"success": false, "message": "解析结果失败"}
	}

	if success, _ := result["success"].(bool); success {
		m.IsValid = true
	}

	return result
}

func (m *LicenseManager) VerifyLicense(licenseKey string) map[string]interface{} {
	serverURL := GetServerURL()

	if !session.Manager.HasSession() {
		if err := session.Manager.Handshake(serverURL); err != nil {
			log.Printf("[安全] ECDH 握手失败: %v", err)
			return map[string]interface{}{"success": false, "message": "无法建立安全会话: " + err.Error()}
		}
	}

	deviceID := device.GetDeviceID()

	reqData := map[string]string{
		"key":       licenseKey,
		"device_id": deviceID,
	}
	reqJSON, _ := json.Marshal(reqData)

	encrypted, err := crypto.Encrypt(string(reqJSON))
	if err != nil {
		return map[string]interface{}{"success": false, "message": "加密失败: " + err.Error()}
	}

	payload := map[string]string{"data": encrypted}
	payloadJSON, _ := json.Marshal(payload)

	httpReq, _ := http.NewRequest("POST", serverURL+"/api/activate", bytes.NewReader(payloadJSON))
	httpReq.Header.Set("Content-Type", "application/json")
	if sid := session.Manager.GetSessionID(); sid != "" {
		httpReq.Header.Set("X-Session-ID", sid)
	}
	if sk := session.Manager.GetSessionKey(); sk != nil {
		sig := crypto.HMACSign(payloadJSON, sk)
		httpReq.Header.Set("X-Signature", sig)
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return map[string]interface{}{"success": false, "message": "连接服务器失败: " + err.Error()}
	}
	defer resp.Body.Close()

	var encryptedResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&encryptedResp); err != nil {
		return map[string]interface{}{"success": false, "message": "解析响应失败"}
	}

	decrypted, err := crypto.Decrypt(encryptedResp["data"])
	if err != nil {
		return map[string]interface{}{"success": false, "message": "解密失败: " + err.Error()}
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(decrypted), &result); err != nil {
		return map[string]interface{}{"success": false, "message": "解析结果失败"}
	}

	if success, _ := result["success"].(bool); success {
		cfg := Config{
			ServerURL:     "",
			DeviceID:      deviceID,
			LicenseKey:    licenseKey,
			LastValidated: time.Now().Unix(),
		}

		cfgJSON, _ := json.Marshal(cfg)
		if encryptedCfg, err := crypto.EncryptLocal(string(cfgJSON)); err == nil {
			SaveToRegistry(encryptedCfg)
		}

		m.IsValid = true

		if m.OnValidated != nil {
			go m.OnValidated()
		}
	}

	return result
}

func (m *LicenseManager) CheckLicense() map[string]interface{} {
	encryptedData, err := LoadFromRegistry()
	if err != nil {
		return map[string]interface{}{"valid": false}
	}

	decryptedData, err := crypto.DecryptLocal(encryptedData)
	if err != nil {
		DeleteFromRegistry()
		return map[string]interface{}{"valid": false}
	}

	var cfg Config
	if err := json.Unmarshal([]byte(decryptedData), &cfg); err != nil {
		DeleteFromRegistry()
		return map[string]interface{}{"valid": false}
	}

	result := m.ValidateLicense(cfg.LicenseKey)
	success, _ := result["success"].(bool)

	if !success {
		message, _ := result["message"].(string)
		if strings.Contains(message, "连接服务器失败") ||
			strings.Contains(message, "解析响应失败") ||
			strings.Contains(message, "解密失败") {
			if cfg.LastValidated > 0 && time.Now().Unix()-cfg.LastValidated < 60 {
				log.Printf("服务器不可达，离线模式（距上次验证 %d 秒）", time.Now().Unix()-cfg.LastValidated)
				m.IsValid = true
				return map[string]interface{}{"valid": true}
			}
			log.Printf("服务器不可达，离线授权已过期")
			return map[string]interface{}{
				"valid":   false,
				"message": "离线授权已过期，请连接网络后重试",
			}
		}
	}

	if success {
		cfg.LastValidated = time.Now().Unix()
		cfgJSON, _ := json.Marshal(cfg)
		if encCfg, err := crypto.EncryptLocal(string(cfgJSON)); err == nil {
			SaveToRegistry(encCfg)
		}
	} else {
		message, _ := result["message"].(string)
		if message == "" {
			message = "卡密已过期或无效"
		}
		return map[string]interface{}{
			"valid":   false,
			"message": message,
		}
	}

	return map[string]interface{}{"valid": true}
}

func (m *LicenseManager) GetLicenseInfo() map[string]interface{} {
	encryptedData, err := LoadFromRegistry()
	if err != nil {
		return map[string]interface{}{"valid": false, "key": ""}
	}

	decryptedData, err := crypto.DecryptLocal(encryptedData)
	if err != nil {
		return map[string]interface{}{"valid": false, "key": ""}
	}

	var cfg Config
	if err := json.Unmarshal([]byte(decryptedData), &cfg); err != nil {
		return map[string]interface{}{"valid": false, "key": ""}
	}

	result := m.ValidateLicense(cfg.LicenseKey)
	result["key"] = cfg.LicenseKey
	return result
}

func (m *LicenseManager) LogoutLicense() map[string]interface{} {
	encryptedData, err := LoadFromRegistry()
	if err != nil {
		return map[string]interface{}{"success": false, "message": "未找到本地卡密信息"}
	}

	decryptedData, err := crypto.DecryptLocal(encryptedData)
	if err != nil {
		DeleteFromRegistry()
		return map[string]interface{}{"success": true, "message": "已清理本地配置"}
	}

	var cfg Config
	if err := json.Unmarshal([]byte(decryptedData), &cfg); err != nil {
		DeleteFromRegistry()
		return map[string]interface{}{"success": true, "message": "已清理本地配置"}
	}

	serverURL := GetServerURL()
	deviceID := device.GetDeviceID()

	reqData := map[string]string{
		"key":       cfg.LicenseKey,
		"device_id": deviceID,
	}
	reqJSON, _ := json.Marshal(reqData)

	encrypted, err := crypto.Encrypt(string(reqJSON))
	if err != nil {
		DeleteFromRegistry()
		m.IsValid = false
		return map[string]interface{}{"success": true, "message": "已清理本地配置"}
	}

	payload := map[string]string{"data": encrypted}
	payloadJSON, _ := json.Marshal(payload)

	httpReq, _ := http.NewRequest("POST", serverURL+"/api/self-unbind", bytes.NewReader(payloadJSON))
	httpReq.Header.Set("Content-Type", "application/json")
	if sid := session.Manager.GetSessionID(); sid != "" {
		httpReq.Header.Set("X-Session-ID", sid)
	}
	if sk := session.Manager.GetSessionKey(); sk != nil {
		sig := crypto.HMACSign(payloadJSON, sk)
		httpReq.Header.Set("X-Signature", sig)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		DeleteFromRegistry()
		m.IsValid = false
		return map[string]interface{}{"success": true, "message": "已清理本地配置（服务器不可达）"}
	}
	defer resp.Body.Close()

	var encryptedResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&encryptedResp); err != nil {
		DeleteFromRegistry()
		m.IsValid = false
		return map[string]interface{}{"success": true, "message": "已清理本地配置"}
	}

	decrypted, err := crypto.Decrypt(encryptedResp["data"])
	if err != nil {
		DeleteFromRegistry()
		m.IsValid = false
		return map[string]interface{}{"success": true, "message": "已清理本地配置"}
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(decrypted), &result); err != nil {
		DeleteFromRegistry()
		m.IsValid = false
		return map[string]interface{}{"success": true, "message": "已清理本地配置"}
	}

	DeleteFromRegistry()
	m.IsValid = false

	success, _ := result["success"].(bool)
	message, _ := result["message"].(string)

	if success {
		return map[string]interface{}{"success": true, "message": message}
	} else {
		return map[string]interface{}{"success": true, "message": "已清理本地配置，但服务器解绑失败: " + message}
	}
}
