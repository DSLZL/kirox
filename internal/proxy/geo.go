package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// continentMap 国家代码 → 洲代码映射
var continentMap = map[string]string{
	"US": "NA", "CA": "NA", "MX": "NA",
	"BR": "SA", "AR": "SA", "CL": "SA", "CO": "SA",
	"GB": "EU", "DE": "EU", "FR": "EU", "NL": "EU", "IT": "EU", "ES": "EU",
	"SE": "EU", "NO": "EU", "FI": "EU", "DK": "EU", "PL": "EU", "CH": "EU",
	"AT": "EU", "BE": "EU", "IE": "EU", "PT": "EU", "CZ": "EU", "RO": "EU",
	"JP": "AS", "KR": "AS", "CN": "AS", "IN": "AS", "SG": "AS", "HK": "AS",
	"TW": "AS", "TH": "AS", "VN": "AS", "MY": "AS", "ID": "AS", "PH": "AS",
	"AU": "OC", "NZ": "OC",
	"ZA": "AF", "NG": "AF", "EG": "AF", "KE": "AF",
	"RU": "EU", "UA": "EU", "TR": "EU",
	"AE": "AS", "SA": "AS", "IL": "AS",
}

// GeoInfo IP 地理信息查询结果
type GeoInfo struct {
	IP        string `json:"query"`
	Country   string `json:"country"`
	CountryCode string `json:"countryCode"`
	Region    string `json:"regionName"`
	City      string `json:"city"`
	ISP       string `json:"isp"`
	Org       string `json:"org"`
	Hosting   bool   `json:"hosting"`
}

// ResolveGeoAsync 异步批量解析代理地理信息
// 对每个未解析的代理，通过代理本身查询 ip-api.com
func ResolveGeoAsync(pool *SmartProxyPool) {
	entries := pool.GetEntries()

	// 筛选未解析的
	var unresolved []*ProxyEntry
	for _, e := range entries {
		if !e.GeoResolved && e.Country == "" {
			unresolved = append(unresolved, e)
		}
	}

	if len(unresolved) == 0 {
		return
	}

	log.Printf("[代理] 开始解析 %d 个代理的地理信息...", len(unresolved))

	// 限制并发为 3（ip-api 免费版 45 req/min）
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for _, entry := range unresolved {
		wg.Add(1)
		sem <- struct{}{}
		go func(e *ProxyEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			info, err := queryGeoViaProxy(e.Address)
			if err != nil {
				log.Printf("[代理] %s 地理信息查询失败: %v", maskAddress(e.Address), err)
				return
			}

			continent := continentMap[info.CountryCode]
			ipType := "residential"
			if info.Hosting {
				ipType = "datacenter"
			}

			pool.UpdateEntryGeo(
				e.Address,
				info.CountryCode,
				info.Region,
				continent,
				info.City,
				info.ISP,
				ipType,
			)

			log.Printf("[代理] %s → %s %s %s (%s)",
				maskAddress(e.Address), info.CountryCode, info.Region, info.City, ipType)

			// ip-api 限流：每 1.5 秒一个请求
			time.Sleep(1500 * time.Millisecond)
		}(entry)
	}

	wg.Wait()
	log.Printf("[代理] 地理信息解析完成")
}

// queryGeoViaProxy 智能查询代理地理信息
func queryGeoViaProxy(proxyAddr string) (*GeoInfo, error) {
	proxyURL, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, err
	}

	scheme := strings.ToLower(proxyURL.Scheme)
	
	// 如果是 Go 原生支持的协议 (http/socks5)，优先尝试通过代理去查它的真实出网 IP
	if scheme == "http" || scheme == "https" || scheme == "socks5" {
		client := &http.Client{
			Timeout:   12 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		}

		resp, err := client.Get("http://ip-api.com/json/?fields=query,country,countryCode,regionName,city,isp,org,hosting")
		if err == nil {
			defer resp.Body.Close()
			var info GeoInfo
			if err := json.NewDecoder(resp.Body).Decode(&info); err == nil && info.CountryCode != "" {
				return &info, nil
			}
		}
	}

	// 对于不支持的自定义协议 (ss/vmess/trojan)，或者上方连通性测试超时，降级为直连查询节点的接入 IP
	host := proxyURL.Hostname()
	if host == "" {
		return nil, fmt.Errorf("无法从 %s 提取主机名", proxyAddr)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	reqURL := fmt.Sprintf("http://ip-api.com/json/%s?fields=query,country,countryCode,regionName,city,isp,org,hosting", host)
	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info GeoInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

// maskAddress 脱敏代理地址（隐藏密码）
func maskAddress(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		if len(addr) > 30 {
			return addr[:30] + "..."
		}
		return addr
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	return u.String()
}
