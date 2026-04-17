package network

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	httputil "reg_go/internal/http"
)

// CheckIPInfo 通过代理查询出口 IP 的归属地和类型
func CheckIPInfo(proxy string) {
	if proxy == "" {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get("http://ip-api.com/json/?fields=query,country,regionName,city,isp,org,as,hosting")
		parseIPInfo(resp, err)
		return
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil {
		log.Printf("[Kiro] IP 检测失败: 代理地址解析错误: %v", err)
		return
	}

	scheme := strings.ToLower(proxyURL.Scheme)
	var resp *http.Response
	var reqErr error

	if scheme == "http" || scheme == "https" || scheme == "socks5" {
		client := &http.Client{
			Timeout: 12 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		}
		resp, reqErr = client.Get("http://ip-api.com/json/?fields=query,country,regionName,city,isp,org,as,hosting")
	} else {
		host := proxyURL.Hostname()
		if host == "" {
			log.Printf("[Kiro] IP 检测失败: 无法提取主机名")
			return
		}
		client := &http.Client{Timeout: 10 * time.Second}
		reqURL := fmt.Sprintf("http://ip-api.com/json/%s?fields=query,country,regionName,city,isp,org,as,hosting", host)
		resp, reqErr = client.Get(reqURL)
	}

	parseIPInfo(resp, reqErr)
}

func parseIPInfo(resp *http.Response, err error) {
	if err != nil {
		log.Printf("[Kiro] IP 检测失败: %v", err)
		return
	}
	defer resp.Body.Close()

	var info struct {
		IP      string `json:"query"`
		Country string `json:"country"`
		Region  string `json:"regionName"`
		City    string `json:"city"`
		ISP     string `json:"isp"`
		Org     string `json:"org"`
		AS      string `json:"as"`
		Hosting bool   `json:"hosting"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		log.Printf("[Kiro] IP 信息解析失败: %v", err)
		return
	}

	ipType := "住宅IP (Residential)"
	if info.Hosting {
		ipType = "机房IP (Datacenter)"
	}

	log.Printf("[Kiro] ═══════════════════════════════")
	log.Printf("[Kiro] 出口 IP: %s", info.IP)
	log.Printf("[Kiro] 归属地: %s %s %s", info.Country, info.Region, info.City)
	log.Printf("[Kiro] IP 类型: %s", ipType)
	log.Printf("[Kiro] ISP: %s", info.ISP)
	if info.Org != "" && info.Org != info.ISP {
		log.Printf("[Kiro] 机构: %s", info.Org)
	}
	log.Printf("[Kiro] ═══════════════════════════════")

	if info.Hosting {
		log.Printf("[Kiro] ⚠ 警告: 当前使用机房IP，注册成功率可能较低，建议使用住宅代理")
	}
}

// TestProxy 测试代理连通性和延迟
func TestProxy(proxyStr string) map[string]interface{} {
	if proxyStr == "" {
		return map[string]interface{}{
			"error": "请输入代理地址",
		}
	}

	// 解析代理列表
	proxyStr = strings.ReplaceAll(proxyStr, ";", ",")
	proxyStr = strings.ReplaceAll(proxyStr, "\n", ",")
	proxyStr = strings.ReplaceAll(proxyStr, "\r", "")

	parts := strings.Split(proxyStr, ",")
	var proxies []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			proxies = append(proxies, p)
		}
	}

	if len(proxies) == 0 {
		return map[string]interface{}{
			"error": "未解析到有效代理",
		}
	}

	// 测试每个代理
	var results []map[string]interface{}
	successCount := 0

	for _, proxy := range proxies {
		result := testSingleProxy(proxy)
		results = append(results, result)
		if result["success"] == true {
			successCount++
		}
	}

	return map[string]interface{}{
		"total":   len(proxies),
		"success": successCount,
		"failed":  len(proxies) - successCount,
		"results": results,
	}
}

// testSingleProxy 测试单个代理
func testSingleProxy(proxy string) map[string]interface{} {
	// 使用实际业务相关的 URL 测试（AWS 服务端点）
	testURLs := []string{
		"https://oidc.us-east-1.amazonaws.com",
		"https://us-east-1.signin.aws",
		"https://www.google.com/generate_204",
	}

	for _, testURL := range testURLs {
		start := time.Now()
		client := httputil.NewTLSClient(proxy, true)

		resp, err := client.Get(testURL)
		latency := time.Since(start).Milliseconds()

		if err != nil {
			// 如果是最后一个 URL 也失败，返回错误
			if testURL == testURLs[len(testURLs)-1] {
				errMsg := err.Error()
				// 简化错误信息
				if strings.Contains(errMsg, "wsarecv") || strings.Contains(errMsg, "forcibly closed") {
					errMsg = "连接被拒绝"
				} else if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "Timeout") {
					errMsg = "连接超时"
				} else if strings.Contains(errMsg, "refused") {
					errMsg = "代理拒绝连接"
				} else if strings.Contains(errMsg, "no such host") {
					errMsg = "代理地址无法解析"
				}
				log.Printf("代理测试失败 [%s]: %v", proxy, err)
				return map[string]interface{}{
					"proxy":   proxy,
					"success": false,
					"error":   errMsg,
					"latency": 0,
				}
			}
			continue // 尝试下一个 URL
		}

		// 任何非错误响应都算成功（包括 403、302 等）
		log.Printf("代理测试成功 [%s]: %dms (HTTP %d via %s)", proxy, latency, resp.StatusCode, testURL)
		return map[string]interface{}{
			"proxy":   proxy,
			"success": true,
			"latency": latency,
		}
	}

	return map[string]interface{}{
		"proxy":   proxy,
		"success": false,
		"error":   "所有测试端点均不可达",
		"latency": 0,
	}
}
