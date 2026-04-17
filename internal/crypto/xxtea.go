package crypto

import (
	"io"
	"log"
	"regexp"
	"sync"

	fhttp "github.com/bogdanfinn/fhttp"
	httputil "reg_go/internal/http"
)

const (
	fallbackVer = "4.0.0"
)

var (
	cacheMu       sync.Mutex
	cachedVersion string
)

// RefreshAppJSConfig 从 app.js 获取 TES 版本号
func RefreshAppJSConfig(proxy string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cachedVersion != "" {
		return
	}

	js := fetchAppJS(proxy)
	if js != "" {
		ver := extractVersionFromAppJS(js)
		if ver != "" {
			cachedVersion = ver
		}
	}
	if cachedVersion == "" {
		cachedVersion = fallbackVer
	}
}

// GetTESVersion 获取当前 TES 版本
func GetTESVersion() string {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cachedVersion != "" {
		return cachedVersion
	}
	return fallbackVer
}

// fetchAppJS 下载 signin.aws app.js
func fetchAppJS(proxy string) string {
	client := httputil.NewTLSClient(proxy, true)
	req, _ := fhttp.NewRequest("GET", "https://us-east-1.signin.aws/assets/js/app.js", nil)
	httputil.SetHeaders(req, map[string]string{
		"User-Agent":      httputil.DefaultUA,
		"Accept":          "*/*",
		"Accept-Language":  "en-US,en;q=0.9",
		"Referer":         "https://us-east-1.signin.aws/",
		"sec-ch-ua":       httputil.DefaultSecUA,
		"sec-fetch-dest":  "script",
		"sec-fetch-mode":  "no-cors",
		"sec-fetch-site":  "same-origin",
	})
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[tes] 下载 app.js 失败: %v", err)
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// extractVersionFromAppJS 从 app.js 提取 TES 版本号
func extractVersionFromAppJS(js string) string {
	reVer := regexp.MustCompile(`FWCIM_VERSION\s*=\s*"(\d+\.\d+\.\d+)"`)
	vm := reVer.FindStringSubmatch(js)
	if len(vm) == 2 {
		return vm[1]
	}
	return ""
}
