package data

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"reg_go/internal/storage"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportKiroResults 导出 Kiro 注册结果（selectedEmails 非空时只导出勾选的）
func ExportKiroResults(ctx context.Context, format string, selectedEmails []string) map[string]interface{} {
	accounts, err := storage.LoadEncryptedJSON(filepath.Join(storage.GetKiroDir(), "results.dat"))
	if err != nil || len(accounts) == 0 {
		return map[string]interface{}{"error": "没有可导出的结果"}
	}
	if len(selectedEmails) > 0 {
		emailSet := make(map[string]bool, len(selectedEmails))
		for _, e := range selectedEmails {
			emailSet[e] = true
		}
		var filtered []map[string]interface{}
		for _, acc := range accounts {
			if email, _ := acc["email"].(string); emailSet[email] {
				filtered = append(filtered, acc)
			}
		}
		if len(filtered) == 0 {
			return map[string]interface{}{"error": "没有可导出的结果"}
		}
		accounts = filtered
	}
	return exportResults(ctx, accounts, format, "Kiro")
}

func exportResults(ctx context.Context, accounts []map[string]interface{}, format, prefix string) map[string]interface{} {
	var ext, filterName string
	switch format {
	case "json":
		ext = "*.json"
		filterName = "JSON"
	case "csv":
		ext = "*.csv"
		filterName = "CSV"
	case "txt":
		ext = "*.txt"
		filterName = "TXT"
	default:
		return map[string]interface{}{"error": "不支持的格式: " + format}
	}

	savePath, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
		Title:           "导出 " + prefix + " 结果",
		DefaultFilename: prefix + "_results." + format,
		Filters: []runtime.FileFilter{
			{DisplayName: filterName + " Files", Pattern: ext},
		},
	})
	if err != nil || savePath == "" {
		return map[string]interface{}{"cancelled": true}
	}

	var content []byte
	switch format {
	case "json":
		content, err = json.MarshalIndent(accounts, "", "  ")
	case "csv":
		content, err = resultsToCSV(accounts)
	case "txt":
		content, err = resultsToTXT(accounts)
	}
	if err != nil {
		return map[string]interface{}{"error": "格式化失败: " + err.Error()}
	}

	if err := os.WriteFile(savePath, content, 0644); err != nil {
		return map[string]interface{}{"error": "写入失败: " + err.Error()}
	}

	log.Printf("[%s] 已导出 %d 条结果到 %s", prefix, len(accounts), savePath)
	return map[string]interface{}{"success": true, "count": len(accounts), "path": savePath}
}

func resultsToCSV(accounts []map[string]interface{}) ([]byte, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	// BOM for Excel compatibility
	buf.WriteString("\xEF\xBB\xBF")
	w.Write([]string{"email", "refreshToken", "clientId", "clientSecret", "region", "provider", "creditUsed", "creditLimit", "subscription", "time"})
	for _, acc := range accounts {
		w.Write([]string{
			str(acc["email"]), str(acc["refreshToken"]), str(acc["clientId"]),
			str(acc["clientSecret"]), str(acc["region"]), str(acc["provider"]),
			str(acc["creditUsed"]), str(acc["creditLimit"]), str(acc["subscription"]),
			str(acc["time"]),
		})
	}
	w.Flush()
	return []byte(buf.String()), w.Error()
}

func resultsToTXT(accounts []map[string]interface{}) ([]byte, error) {
	var lines []string
	for _, acc := range accounts {
		email := str(acc["email"])
		rt := str(acc["refreshToken"])
		cid := str(acc["clientId"])
		cs := str(acc["clientSecret"])
		if rt != "" {
			lines = append(lines, fmt.Sprintf("%s----refreshToken:%s----clientId:%s----clientSecret:%s", email, rt, cid, cs))
		} else {
			lines = append(lines, email)
		}
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%.2f", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// ImportKiroResults 导入 Kiro 注册结果
func ImportKiroResults(ctx context.Context) map[string]interface{} {
	filePath, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "导入 Kiro 结果",
		Filters: []runtime.FileFilter{
			{DisplayName: "支持的格式", Pattern: "*.json;*.csv;*.txt"},
			{DisplayName: "JSON Files", Pattern: "*.json"},
			{DisplayName: "CSV Files", Pattern: "*.csv"},
			{DisplayName: "TXT Files", Pattern: "*.txt"},
		},
	})
	if err != nil || filePath == "" {
		return map[string]interface{}{"cancelled": true}
	}
	return importResults(filePath, filepath.Join(storage.GetKiroDir(), "results.dat"), "Kiro")
}

func importResults(filePath, targetDat, prefix string) map[string]interface{} {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return map[string]interface{}{"error": "读取文件失败: " + err.Error()}
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	var items []map[string]interface{}

	switch ext {
	case ".json":
		// 先尝试解析为数组
		if err := json.Unmarshal(data, &items); err != nil {
			// 再尝试解析为单个对象
			var single map[string]interface{}
			if err2 := json.Unmarshal(data, &single); err2 != nil {
				return map[string]interface{}{"error": "JSON 解析失败: " + err.Error()}
			}
			items = []map[string]interface{}{single}
		}
	case ".csv":
		items, err = parseCSVResults(string(data))
		if err != nil {
			return map[string]interface{}{"error": "CSV 解析失败: " + err.Error()}
		}
	case ".txt":
		items = parseTXTResults(string(data))
	default:
		return map[string]interface{}{"error": "不支持的文件格式: " + ext}
	}

	if len(items) == 0 {
		return map[string]interface{}{"error": "文件中没有有效数据"}
	}

	// 归一化字段名（兼容 snake_case 和其他变体）
	for i := range items {
		items[i] = normalizeResultFields(items[i])
	}

	// 追加到加密存储
	for _, item := range items {
		storage.AppendEncryptedJSON(targetDat, item)
	}

	log.Printf("[%s] 已导入 %d 条结果从 %s", prefix, len(items), filePath)
	return map[string]interface{}{"success": true, "count": len(items)}
}

func parseCSVResults(data string) ([]map[string]interface{}, error) {
	// Remove BOM
	data = strings.TrimPrefix(data, "\xEF\xBB\xBF")
	r := csv.NewReader(strings.NewReader(data))
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV 至少需要标题行和一行数据")
	}

	headers := records[0]
	var items []map[string]interface{}
	for _, row := range records[1:] {
		item := make(map[string]interface{})
		for j, h := range headers {
			if j < len(row) && row[j] != "" {
				item[h] = row[j]
			}
		}
		if item["email"] != nil || item["refreshToken"] != nil {
			items = append(items, item)
		}
	}
	return items, nil
}

// normalizeResultFields 将非标准字段名映射为标准 camelCase
func normalizeResultFields(item map[string]interface{}) map[string]interface{} {
	// 字段映射: 非标准名 → 标准名
	fieldMap := map[string]string{
		"refresh_token": "refreshToken",
		"RefreshToken":  "refreshToken",
		"client_id":     "clientId",
		"ClientId":      "clientId",
		"ClientID":      "clientId",
		"client_secret": "clientSecret",
		"ClientSecret":  "clientSecret",
		"access_token":  "accessToken",
		"AccessToken":   "accessToken",
		"credit_used":   "creditUsed",
		"credit_limit":  "creditLimit",
		"device_code":   "deviceCode",
		"Email":         "email",
		"Region":        "region",
		"Provider":      "provider",
		"Subscription":  "subscription",
		"id_token":      "idToken",
		"session_token": "sessionToken",
		"account_id":    "accountId",
	}

	normalized := make(map[string]interface{}, len(item))
	for k, v := range item {
		if mapped, ok := fieldMap[k]; ok {
			// 只在标准字段不存在时才映射
			if _, exists := item[mapped]; !exists {
				normalized[mapped] = v
			}
		} else {
			normalized[k] = v
		}
	}
	return normalized
}

func parseTXTResults(data string) []map[string]interface{} {
	var items []map[string]interface{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		item := map[string]interface{}{}
		parts := strings.Split(line, "----")
		if len(parts) >= 1 {
			item["email"] = parts[0]
		}
		for _, p := range parts[1:] {
			kv := strings.SplitN(p, ":", 2)
			if len(kv) == 2 {
				item[kv[0]] = kv[1]
			}
		}
		items = append(items, item)
	}
	return items
}
