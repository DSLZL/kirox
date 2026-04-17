package session

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/hkdf"
)

// Manager 全局会话管理器
var Manager = &SessionManager{}

// SessionManager 管理 ECDH 会话
type SessionManager struct {
	mu         sync.RWMutex
	sessionID  string
	sessionKey []byte // 32 字节 AES-256 会话密钥
}

// GetSessionID 获取当前会话 ID
func (s *SessionManager) GetSessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionID
}

// GetSessionKey 获取当前会话密钥
func (s *SessionManager) GetSessionKey() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionKey
}

// HasSession 是否已建立会话
func (s *SessionManager) HasSession() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionID != "" && s.sessionKey != nil
}

// Handshake 执行 ECDH 密钥交换
func (s *SessionManager) Handshake(serverURL string) error {
	// 生成客户端临时 ECDH 密钥对
	clientPrivKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("生成密钥对失败: %w", err)
	}
	clientPubKey := clientPrivKey.PublicKey()

	// 发送公钥到服务器
	reqData := map[string]string{
		"client_public_key": base64.StdEncoding.EncodeToString(clientPubKey.Bytes()),
	}
	reqJSON, _ := json.Marshal(reqData)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(serverURL+"/api/handshake", "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		return fmt.Errorf("连接服务器失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("握手失败: HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ServerPublicKey string `json:"server_public_key"`
		SessionID       string `json:"session_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	// 解码服务端公钥
	serverPubBytes, err := base64.StdEncoding.DecodeString(result.ServerPublicKey)
	if err != nil {
		return fmt.Errorf("解码服务端公钥失败: %w", err)
	}

	serverPubKey, err := ecdh.P256().NewPublicKey(serverPubBytes)
	if err != nil {
		return fmt.Errorf("无效的服务端公钥: %w", err)
	}

	// 计算共享密钥
	sharedSecret, err := clientPrivKey.ECDH(serverPubKey)
	if err != nil {
		return fmt.Errorf("密钥交换失败: %w", err)
	}

	// 通过 HKDF 派生 AES-256 会话密钥（与服务端使用相同参数）
	sessionKey := DeriveKey(sharedSecret)

	// 保存会话
	s.mu.Lock()
	s.sessionID = result.SessionID
	s.sessionKey = sessionKey
	s.mu.Unlock()

	log.Printf("[安全] ECDH 会话已建立: %s...", result.SessionID[:8])
	return nil
}

// DeriveKey 使用 HKDF 从共享密钥派生 AES-256 密钥
func DeriveKey(sharedSecret []byte) []byte {
	salt := []byte("kiro-reg-session-v1")
	info := []byte("aes-256-session-key")

	hkdfReader := hkdf.New(sha256.New, sharedSecret, salt, info)
	key := make([]byte, 32)
	io.ReadFull(hkdfReader, key)
	return key
}
