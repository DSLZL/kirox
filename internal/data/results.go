package data

import (
	"log"
	"path/filepath"
	"strings"
	"time"

	"reg_go/internal/storage"
)

// ClassifyError 根据错误信息分类结果
func ClassifyError(errorMsg string) string {
	if errorMsg == "" {
		return "failed"
	}
	// 封禁
	if strings.Contains(errorMsg, "suspended") {
		return "banned"
	}
	// 已注册（不可重试）
	if strings.Contains(errorMsg, "邮箱已注册过") || strings.Contains(errorMsg, "临时邮箱不可能已存在") {
		return "registered"
	}
	// 其他都是可重试的错误（网络错误等）
	return "failed"
}

// SaveKiroResult 保存注册结果到加密存储
func SaveKiroResult(result map[string]interface{}, outDir string) {
	emailAddr, _ := result["email"].(string)

	if result["status"] == "success" {
		at, _ := result["aws_token"].(map[string]interface{})
		if at == nil {
			at = map[string]interface{}{}
		}
		verify, _ := result["verify"].(map[string]interface{})
		item := map[string]interface{}{
			"refreshToken": at["refreshToken"],
			"provider":     "BuilderId",
			"clientId":     result["client_id"],
			"clientSecret": result["client_secret"],
			"region":       "us-east-1",
			"email":        emailAddr,
			"time":         time.Now().Format("2006-01-02 15:04:05"),
		}
		if verify != nil {
			item["creditUsed"] = verify["credit_used"]
			item["creditLimit"] = verify["credit_limit"]
			item["subscription"] = verify["subscription"]
		}
		storage.AppendEncryptedJSON(filepath.Join(outDir, "results.dat"), item)
		log.Printf("[Kiro] 结果已保存: %s/results.dat", outDir)
	} else {
		errorMsg, _ := result["error"].(string)
		category := ClassifyError(errorMsg)

		failItem := map[string]interface{}{
			"email": emailAddr,
			"error": errorMsg,
			"time":  time.Now().Format("2006-01-02 15:04:05"),
		}

		var targetFile string
		switch category {
		case "banned", "registered":
			if category == "registered" {
				failItem["error"] = "邮箱已注册"
			}
			targetFile = filepath.Join(outDir, "banned.dat")
		default:
			targetFile = filepath.Join(outDir, "failed.dat")
		}

		storage.AppendEncryptedJSON(targetFile, failItem)
	}
}

// BatchDeleteResults 批量删除注册结果
func BatchDeleteResults(emails []string) map[string]interface{} {
	resultsPath := filepath.Join(storage.GetKiroDir(), "results.dat")
	accounts, err := storage.LoadEncryptedJSON(resultsPath)
	if err != nil {
		return map[string]interface{}{"error": "读取数据失败"}
	}

	emailSet := make(map[string]bool, len(emails))
	for _, e := range emails {
		emailSet[e] = true
	}

	var kept []map[string]interface{}
	deleted := 0
	for _, acc := range accounts {
		email, _ := acc["email"].(string)
		if emailSet[email] {
			deleted++
		} else {
			kept = append(kept, acc)
		}
	}

	if deleted == 0 {
		return map[string]interface{}{"error": "未找到匹配的账号"}
	}

	if kept == nil {
		kept = []map[string]interface{}{}
	}
	if err := storage.SaveEncryptedJSON(resultsPath, kept); err != nil {
		return map[string]interface{}{"error": "保存失败: " + err.Error()}
	}

	log.Printf("[结果] 批量删除 %d 个账号", deleted)
	return map[string]interface{}{
		"deleted": deleted,
		"total":   len(kept),
	}
}

// ClearKiroResults 一键清空所有 Kiro 注册结果
func ClearKiroResults() map[string]interface{} {
	resultsPath := filepath.Join(storage.GetKiroDir(), "results.dat")
	accounts, err := storage.LoadEncryptedJSON(resultsPath)
	if err != nil {
		return map[string]interface{}{"error": "读取数据失败"}
	}
	count := len(accounts)
	if count == 0 {
		return map[string]interface{}{"deleted": 0, "total": 0}
	}
	if err := storage.SaveEncryptedJSON(resultsPath, []map[string]interface{}{}); err != nil {
		return map[string]interface{}{"error": "保存失败: " + err.Error()}
	}
	log.Printf("[结果] 已清空 %d 个 Kiro 注册结果", count)
	return map[string]interface{}{
		"deleted": count,
		"total":   0,
	}
}

// GetResults 获取结果列表（分页 + 状态筛选）
// filter: "all" | "normal" | "banned"
func GetResults(page int, pageSize int, filter string) map[string]interface{} {
	accounts, err := storage.LoadEncryptedJSON(filepath.Join(storage.GetKiroDir(), "results.dat"))
	if err != nil {
		return map[string]interface{}{
			"total":    0,
			"page":     1,
			"pageSize": pageSize,
			"accounts": []interface{}{},
		}
	}

	// 按状态筛选
	if filter == "banned" || filter == "normal" {
		var filtered []map[string]interface{}
		for _, acc := range accounts {
			banned, _ := acc["banned"].(bool)
			if filter == "banned" && banned {
				filtered = append(filtered, acc)
			} else if filter == "normal" && !banned {
				filtered = append(filtered, acc)
			}
		}
		accounts = filtered
	}

	if pageSize <= 0 {
		pageSize = 10
	}
	if page < 1 {
		page = 1
	}

	total := len(accounts)
	totalPages := (total + pageSize - 1) / pageSize
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}

	var pageData []map[string]interface{}
	if start < total {
		pageData = accounts[start:end]
	}
	if pageData == nil {
		pageData = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"accounts": pageData,
	}
}
