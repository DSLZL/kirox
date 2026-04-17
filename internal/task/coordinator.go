package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"reg_go/internal/core"
	"reg_go/internal/crypto"
	"reg_go/internal/data"
	"reg_go/internal/email"
	"reg_go/internal/license"
	"reg_go/internal/network"
	"reg_go/internal/proxy"
	"reg_go/internal/security"
	"reg_go/internal/storage"
)

// StartTaskRequest 启动任务请求
type StartTaskRequest struct {
	Count              int                                `json:"count"`
	Concurrency        int                                `json:"concurrency"`
	Delay              int                                `json:"delay"`
	Proxy              string                             `json:"proxy"`
	OutputPath         string                             `json:"outputPath"`
	EmailProvider      string                             `json:"emailProvider"`      // "outlook" 或 "moemail"
	MoeMailDomains     []string                           `json:"moemailDomains"`     // 选中的域名列表
	MoeMailConfigs     map[string][]email.MoeMailConfig   `json:"moemailConfigs"`     // 域名 -> 配置列表映射
	MoeMailRandomMode  bool                               `json:"moemailRandomMode"`  // 是否为随机模式
}

// StartTask 公开方法（包装器）
func StartTask(req StartTaskRequest) map[string]interface{} {
	if !license.Manager.IsValid {
		// 尝试从注册表恢复授权状态（应用重启后 IsValid 会重置为 false）
		result := license.Manager.CheckLicense()
		if valid, _ := result["valid"].(bool); !valid {
			return map[string]interface{}{"error": "授权无效，请先验证卡密"}
		}
	}
	return startTask(req)
}

// startTask 启动注册任务（私有方法）
func startTask(req StartTaskRequest) map[string]interface{} {
	// 多重反调试检测
	if security.CheckDebugger() {
		time.Sleep(200 * time.Millisecond)
		os.Exit(1)
		return nil
	}

	// 验证内部状态完整性
	if !security.VerifyIntegrity(false, license.Manager.IsValid) {
		time.Sleep(200 * time.Millisecond)
		os.Exit(1)
		return nil
	}

	// 先验证卡密（向服务器重新验证）
	encryptedData, err := license.LoadFromRegistry()
	if err != nil {
		Manager.mu.Lock()
		Manager.running = false
		Manager.mu.Unlock()
		return map[string]interface{}{"error": "未找到卡密配置，请先验证卡密"}
	}

	// 解密配置
	decryptedData, err := crypto.DecryptLocal(encryptedData)
	if err != nil {
		// 配置损坏，删除并引导重新验证（不置 licenseValid=false 避免触发反调试）
		license.DeleteFromRegistry()
		Manager.mu.Lock()
		Manager.running = false
		Manager.mu.Unlock()
		return map[string]interface{}{"error": "卡密配置损坏，请退出卡密后重新验证"}
	}

	var cfg license.Config
	if err := json.Unmarshal([]byte(decryptedData), &cfg); err != nil {
		license.DeleteFromRegistry()
		Manager.mu.Lock()
		Manager.running = false
		Manager.mu.Unlock()
		return map[string]interface{}{"error": "卡密配置解析失败，请退出卡密后重新验证"}
	}

	// 向服务器验证卡密（使用 validate 端点，不消耗设备名额）
	result := license.Manager.ValidateLicense(cfg.LicenseKey)
	success, _ := result["success"].(bool)
	if !success {
		// 验证失败，返回错误但不清空注册表
		license.Manager.IsValid = false
		message, _ := result["message"].(string)
		if message == "" {
			message = "卡密验证失败"
		}
		return map[string]interface{}{"error": "卡密验证失败: " + message}
	}

	// 二次验证：检查返回的 success 字段
	if !license.Manager.IsValid && !success {
		return map[string]interface{}{"error": "授权验证失败"}
	}

	// 再次验证卡密（防止内存修改）
	if !license.Manager.IsValid {
		return map[string]interface{}{"error": "授权状态异常"}
	}

	Manager.mu.Lock()
	if Manager.running {
		Manager.mu.Unlock()
		return map[string]interface{}{"error": "任务正在运行中"}
	}

	// 根据邮箱提供商类型处理
	emailProvider := req.EmailProvider
	if emailProvider == "" {
		emailProvider = "outlook" // 默认使用 Outlook
	}

	var outlookAccounts []email.OutlookAccount

	if emailProvider == "moemail" {
		// MoeMail 模式：验证域名和配置
		if len(req.MoeMailDomains) == 0 {
			Manager.mu.Unlock()
			return map[string]interface{}{"error": "请选择至少一个域名"}
		}
		if len(req.MoeMailConfigs) == 0 {
			Manager.mu.Unlock()
			return map[string]interface{}{"error": "MoeMail 配置缺失"}
		}
		// MoeMail 不需要预先加载账号，每次任务动态生成
	} else {
		// Outlook 模式：加载账号列表
		storedAccounts := storage.GetAccountsCached()
		if len(storedAccounts) == 0 {
			Manager.mu.Unlock()
			return map[string]interface{}{"error": "请先添加微软邮箱账号"}
		}

		// 筛选未注册的账号
		for _, acc := range storedAccounts {
			registered, _ := acc["registered"].(bool)
			if !registered {
				emailAddr, _ := acc["email"].(string)
				password, _ := acc["password"].(string)
				clientID, _ := acc["clientId"].(string)
				refreshToken, _ := acc["refreshToken"].(string)

				outlookAccounts = append(outlookAccounts, email.OutlookAccount{
					Email:        emailAddr,
					Password:     password,
					ClientID:     clientID,
					RefreshToken: refreshToken,
				})
			}
		}

		if len(outlookAccounts) == 0 {
			Manager.mu.Unlock()
			return map[string]interface{}{"error": "没有可用的 Outlook 账号（所有账号已注册成功）"}
		}

		if len(outlookAccounts) < req.Count {
			Manager.mu.Unlock()
			return map[string]interface{}{
				"error": fmt.Sprintf("可用 Outlook 账号不足: 需要 %d, 仅有 %d", req.Count, len(outlookAccounts)),
			}
		}
	}

	// 初始化状态
	Manager.running = true
	Manager.stopCh = make(chan struct{})
	Manager.total = req.Count
	Manager.completed = 0
	Manager.success = 0
	Manager.failed = 0
	Manager.results = nil
	Manager.startTime = time.Now()
	Manager.mu.Unlock()

	// 清空日志
	Manager.logsMu.Lock()
	Manager.logs = nil
	Manager.logsMu.Unlock()

	// 后台执行
	go runBatch(req, emailProvider, outlookAccounts)

	return map[string]interface{}{"status": "started"}
}

// StopTask 停止任务（强制取消所有 HTTP 请求）
func StopTask(force bool) map[string]interface{} {
	Manager.mu.Lock()
	if !Manager.running {
		Manager.mu.Unlock()
		return map[string]interface{}{"error": "没有正在运行的任务"}
	}

	select {
	case <-Manager.stopCh:
	default:
		close(Manager.stopCh)
	}

	// 强制取消所有进行中的 HTTP 请求
	if Manager.cancelFunc != nil {
		Manager.cancelFunc()
	}

	Manager.running = false
	log.Println("[Kiro] 任务已强制停止，所有请求已取消")
	Manager.mu.Unlock()
	return map[string]interface{}{"status": "force_stopped"}
}

// runBatch 执行批量注册
func runBatch(req StartTaskRequest, emailProvider string, outlookAccounts []email.OutlookAccount) {
	// 创建可取消的 context，停止时立即中断所有 HTTP 请求
	taskCtx, taskCancel := context.WithCancel(context.Background())
	defer taskCancel()

	Manager.mu.Lock()
	Manager.cancelFunc = taskCancel
	Manager.mu.Unlock()

	defer func() {
		Manager.mu.Lock()
		Manager.running = false
		Manager.cancelFunc = nil
		Manager.mu.Unlock()
	}()

	// 再次验证授权状态（多重检查）
	if !license.Manager.IsValid {
		log.Println("授权验证失败，任务终止")
		return
	}

	// 验证程序完整性
	if !security.VerifyIntegrity(false, license.Manager.IsValid) {
		log.Println("程序完整性验证失败，任务终止")
		return
	}

	// 向服务器重新验证卡密
	encryptedData, err := license.LoadFromRegistry()
	if err != nil {
		log.Println("读取卡密配置失败，任务终止")
		return
	}

	// 解密配置
	decryptedData, err := crypto.DecryptLocal(encryptedData)
	if err != nil {
		log.Println("解密卡密配置失败，任务终止")
		return
	}

	var cfg license.Config
	if err := json.Unmarshal([]byte(decryptedData), &cfg); err != nil {
		log.Println("解析卡密配置失败，任务终止")
		return
	}

	result := license.Manager.ValidateLicense(cfg.LicenseKey)
	success, _ := result["success"].(bool)
	if !success {
		log.Println("卡密验证失败，任务终止")
		return
	}

	// 初始化加密模块（强制要求，无回退）
	remoteCrypto, err := license.SetupRemoteCrypto(cfg.LicenseKey, cfg.DeviceID)
	if err != nil {
		log.Printf("加密模块初始化失败，任务终止: %v", err)
		return
	}

	outDir := req.OutputPath
	if outDir == "" {
		outDir = storage.GetKiroDir()
	}
	os.MkdirAll(outDir, 0755)

	taskConfig := core.NewConfig()
	taskConfig.Proxy = req.Proxy
	taskConfig.EmailProvider = emailProvider

	// 预先准备 MoeMail 域名池
	var moemailDomainPool []string
	var moemailDomainConfigs map[string][]email.MoeMailConfig
	if emailProvider == "moemail" {
		taskConfig.UseMoeMail = true
		moemailDomainPool = req.MoeMailDomains
		moemailDomainConfigs = req.MoeMailConfigs

		if len(moemailDomainPool) == 0 || len(moemailDomainConfigs) == 0 {
			log.Println("[Kiro] MoeMail 域名或配置为空，任务终止")
			Manager.mu.Lock()
			Manager.running = false
			Manager.mu.Unlock()
			return
		}

		log.Printf("[Kiro] MoeMail 域名池: %v (共 %d 个域名)", moemailDomainPool, len(moemailDomainPool))
	} else if emailProvider == "outlook" {
		taskConfig.UseOutlook = true
	}

	// 初始化代理池（智能代理池优先）
	smartPool := GetSmartProxyPool()
	useSmartProxy := smartPool != nil && smartPool.Count() > 0

	if useSmartProxy {
		log.Printf("[Kiro] 智能代理池已启用 (共 %d 个代理, 可用 %d 个)", smartPool.Count(), smartPool.ActiveCount())
		// 取第一个可用代理检测 IP
		firstProxy := smartPool.Next()
		if firstProxy != "" {
			go network.CheckIPInfo(firstProxy)
		}
	} else if req.Proxy != "" {
		taskConfig.ProxyPool = core.NewProxyPool(req.Proxy)
		if taskConfig.ProxyPool.Count() > 1 {
			log.Printf("[Kiro] 代理池已启用，共 %d 个代理", taskConfig.ProxyPool.Count())
		}
		// 检测出口 IP 信息（取第一个代理检测）
		firstProxy := strings.Split(strings.ReplaceAll(strings.ReplaceAll(req.Proxy, ";", ","), "\n", ","), ",")[0]
		go network.CheckIPInfo(strings.TrimSpace(firstProxy))
	} else {
		go network.CheckIPInfo("")
	}

	// 统计计数器
	var statsMu sync.Mutex
	var taskDurations []float64
	var failRegistered, failNetwork, failBanned, failOther int
	taskStartTime := time.Now()

	// 共享账号池（并发安全），goroutine 动态领取账号（仅 Outlook 模式使用）
	var accountPoolMu sync.Mutex
	accountPoolIdx := 0
	nextAccount := func() (email.OutlookAccount, bool) {
		accountPoolMu.Lock()
		defer accountPoolMu.Unlock()
		if accountPoolIdx >= len(outlookAccounts) {
			return email.OutlookAccount{}, false
		}
		acc := outlookAccounts[accountPoolIdx]
		accountPoolIdx++
		return acc, true
	}

	// MoeMail 域名池索引（并发安全）
	var moemailDomainIdx int
	var moemailDomainMu sync.Mutex
	nextMoeMailDomain := func() (string, email.MoeMailConfig) {
		moemailDomainMu.Lock()
		defer moemailDomainMu.Unlock()

		var domain string
		if req.MoeMailRandomMode {
			// 随机模式：每次随机选择一个域名
			randomIdx := time.Now().UnixNano() % int64(len(moemailDomainPool))
			domain = moemailDomainPool[randomIdx]
		} else {
			// 轮询模式：按顺序轮询域名
			domain = moemailDomainPool[moemailDomainIdx%len(moemailDomainPool)]
			moemailDomainIdx++
		}

		// 从该域名的配置列表中随机选择一个
		configs := moemailDomainConfigs[domain]
		configIdx := time.Now().UnixNano() % int64(len(configs))
		return domain, configs[configIdx]
	}

	var otp400KillOnce sync.Once // OTP 400 熔断：确保只触发一次
	doTask := func(i int) {
		select {
		case <-Manager.stopCh:
			return
		default:
		}

		taskCfg := *taskConfig
		taskCfg.Password = core.GenPassword()
		var currentEmail string

		// 智能代理池：每个任务分配一个代理
		var usedProxy string
		if useSmartProxy {
			usedProxy = smartPool.Next()
			if usedProxy != "" {
				taskCfg.Proxy = usedProxy
			} else {
				log.Printf("[Kiro][%d/%d] 智能代理池无可用代理，跳过", i+1, req.Count)
				Manager.mu.Lock()
				Manager.completed++
				Manager.failed++
				Manager.mu.Unlock()
				return
			}
		} else if taskConfig.ProxyPool != nil && taskConfig.ProxyPool.Count() > 1 {
			usedProxy = taskConfig.ProxyPool.Next()
			taskCfg.Proxy = usedProxy
		}

		// 根据邮箱提供商类型获取邮箱
		if emailProvider == "outlook" {
			// Outlook 模式：从共享池领取账号
			acc, ok := nextAccount()
			if !ok {
				log.Printf("[Kiro][%d/%d] 无可用账号，跳过", i+1, req.Count)
				Manager.mu.Lock()
				Manager.completed++
				Manager.failed++
				Manager.mu.Unlock()
				return
			}
			taskCfg.OutlookAccount = &acc
			currentEmail = acc.Email
		} else if emailProvider == "moemail" {
			// MoeMail 模式：动态生成临时邮箱
			// 从域名池中获取域名和配置
			domain, config := nextMoeMailDomain()

			// 生成完全随机的邮箱名
			emailName := email.GenerateEmailName(i)

			// 使用 1 小时有效期
			expiryTime := int64(3600000) // 1 小时（毫秒）

			log.Printf("[Kiro][%d/%d] 创建 MoeMail 邮箱: %s@%s (配置: %s)", i+1, req.Count, emailName, domain, config.Name)

			// 创建 MoeMail 提供商
			provider, err := email.NewMoeMailProvider(config, emailName, expiryTime, domain)
			if err != nil {
				log.Printf("[Kiro][%d/%d] 生成 MoeMail 邮箱失败: %v", i+1, req.Count, err)
				Manager.mu.Lock()
				Manager.completed++
				Manager.failed++
				Manager.mu.Unlock()
				return
			}

			taskCfg.MoeMailProvider = provider
			currentEmail = provider.GetAddress()
		}

		log.Printf("[Kiro][%d/%d] 开始注册", i+1, req.Count)
		itemStart := time.Now()

		// 重试机制：最多重试 2 次
		var result map[string]interface{}
		maxRetries := 2
	retryLoop:
		for attempt := 0; attempt <= maxRetries; attempt++ {
			// 每次重试前检查停止信号
			select {
			case <-Manager.stopCh:
				return
			default:
			}

			if attempt > 0 {
				log.Printf("[Kiro][%d/%d] 第 %d 次重试", i+1, req.Count, attempt)
				// 可中断的延迟：stopCh 关闭时立即退出
				select {
				case <-Manager.stopCh:
					return
				case <-time.After(time.Duration(2+attempt) * time.Second):
				}
			}

			// 再次检查 context 是否已取消
			if taskCtx.Err() != nil {
				return
			}

			reg := core.NewRegistrar(&taskCfg)
			reg.Ctx = taskCtx
			reg.TaskLabel = fmt.Sprintf("%d/%d", i+1, req.Count)
			reg.EncryptFP = remoteCrypto.EncryptFP
			reg.EncryptJWE = remoteCrypto.EncryptJWE
			result = reg.Run()

			if result["status"] == "success" {
				break
			}

			errorMsg, _ := result["error"].(string)

			// 邮箱已注册：标记当前账号，无限尝试下一个直到账号池耗尽
			if taskConfig.UseOutlook && strings.Contains(errorMsg, "邮箱已注册过") {
				log.Printf("[Kiro][%d/%d] %s 已注册，标记并换号", i+1, req.Count, currentEmail)
				email.UpdateAccountStatus(currentEmail, true, false)
				acc, ok := nextAccount()
				if ok {
					taskCfg.OutlookAccount = &acc
					taskCfg.Password = core.GenPassword()
					currentEmail = acc.Email
					attempt = -1 // 重置重试计数
					continue retryLoop
				}
				// 账号池耗尽
				log.Printf("[Kiro][%d/%d] 账号池已耗尽", i+1, req.Count)
				break
			}

			// 不重试的错误类型（含 context 取消）
			noRetryErrors := []string{"suspended", "临时邮箱不可能已存在", "邮箱创建失败", "context canceled", "context deadline exceeded", "IP或浏览器指纹被检测", "注册请求被拦截", "BLOCKED", "send-otp 失败"}
			shouldRetry := true
			for _, noRetry := range noRetryErrors {
				if strings.Contains(errorMsg, noRetry) {
					shouldRetry = false
					break
				}
			}

			if !shouldRetry || attempt >= maxRetries {
				break
			}

			log.Printf("[Kiro][%d/%d] 注册失败: %s，准备重试", i+1, req.Count, errorMsg)
		}

		itemDuration := time.Since(itemStart).Seconds()

		Manager.mu.Lock()
		Manager.results = append(Manager.results, result)
		Manager.completed++

		success := result["status"] == "success"
		if success {
			Manager.success++
		} else {
			Manager.failed++
		}
		completedCount := Manager.completed
		Manager.mu.Unlock()

		// 统计分类
		statsMu.Lock()
		taskDurations = append(taskDurations, itemDuration)
		if !success {
			errorMsg, _ := result["error"].(string)
			errClass := data.ClassifyError(errorMsg)
			switch errClass {
			case "registered":
				failRegistered++
			case "banned":
				failBanned++
			default:
				if strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "网络") || strings.Contains(errorMsg, "connection") || strings.Contains(errorMsg, "TLS") {
					failNetwork++
				} else {
					failOther++
				}
			}

			// 向智能代理池上报结果
			if useSmartProxy && usedProxy != "" {
				latencyMs := int64(itemDuration * 1000)
				switch {
				case strings.Contains(errorMsg, "400") || strings.Contains(errorMsg, "OTP") || strings.Contains(errorMsg, "send-otp"):
					smartPool.ReportResult(usedProxy, proxy.ResultOTP400, latencyMs)
					// ★ 根据策略决定：熔断 or 切换IP重试
					retryMode := smartPool.GetPolicy().OTP400RetryMode
					if retryMode == "switch_ip" {
						log.Printf("[Kiro] OTP 400 → 切换IP重试 (proxy: %s)", usedProxy)
					} else {
						// 默认: 立即熔断
						otp400KillOnce.Do(func() {
							log.Printf("[Kiro] ⚠️ OTP 400 熔断触发！立即终止所有注册线程")
							go StopTask(true)
						})
					}
				case errClass == "banned" || strings.Contains(errorMsg, "suspended") || strings.Contains(errorMsg, "BLOCKED"):
					smartPool.ReportResult(usedProxy, proxy.ResultBanned, latencyMs)
				case strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "connection") || strings.Contains(errorMsg, "TLS"):
					smartPool.ReportResult(usedProxy, proxy.ResultConnFail, latencyMs)
				}
			}
		} else if useSmartProxy && usedProxy != "" {
			// 成功也上报
			latencyMs := int64(itemDuration * 1000)
			smartPool.ReportResult(usedProxy, proxy.ResultSuccess, latencyMs)
		}
		statsMu.Unlock()

		// log.Printf 必须在 state.mu 外调用，否则与 logWriter 死锁
		if !success {
			if errMsg, ok := result["error"].(string); ok {
				log.Printf("[Kiro][%d/%d] 失败: %s (%s)", completedCount, req.Count, errMsg, currentEmail)
			}
		}

		// 只有设置完密码后（passwordSet=true）才标记邮箱为已注册
		// 之前步骤失败的邮箱不标记，等同于归还到邮箱池
		if taskConfig.UseOutlook && currentEmail != "" {
			passwordSet, _ := result["passwordSet"].(bool)
			if passwordSet {
				email.UpdateAccountStatus(currentEmail, true, success)
			}
			// 未设密码的失败邮箱不标记 registered，下次任务可继续使用
		}
		data.SaveKiroResult(result, outDir)
	}

	if req.Concurrency > 1 {
		log.Printf("[Kiro] 启动并发任务: %d 个任务，并发数 %d", req.Count, req.Concurrency)
		sem := make(chan struct{}, req.Concurrency)
		var wg sync.WaitGroup
	loop:
		for i := 0; i < req.Count; i++ {
			select {
			case <-Manager.stopCh:
				break loop
			default:
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int) {
				defer wg.Done()
				defer func() { <-sem }()
				doTask(idx)
			}(i)
		}
		wg.Wait()
	} else {
		log.Printf("[Kiro] 启动串行任务: %d 个任务", req.Count)
		for i := 0; i < req.Count; i++ {
			select {
			case <-Manager.stopCh:
				log.Println("任务已停止")
				return
			default:
			}
			doTask(i)
			if req.Delay > 0 && i < req.Count-1 {
				time.Sleep(time.Duration(req.Delay) * time.Second)
			}
		}
	}

	totalDuration := time.Since(taskStartTime).Seconds()

	Manager.mu.Lock()
	sucCount := Manager.success
	failCount := Manager.failed
	totalCount := Manager.completed
	Manager.mu.Unlock()

	// 计算平均耗时
	var avgDur float64
	if len(taskDurations) > 0 {
		var sum float64
		for _, d := range taskDurations {
			sum += d
		}
		avgDur = sum / float64(len(taskDurations))
	}

	// 统计报告
	log.Println("[Kiro] ═══════════════════════════════")
	log.Printf("[Kiro] 任务完成 — 总计: %d, 成功: %d, 失败: %d", totalCount, sucCount, failCount)
	log.Printf("[Kiro] 总耗时: %.1fs, 平均耗时: %.1fs/个", totalDuration, avgDur)
	if totalCount > 0 {
		log.Printf("[Kiro] 成功率: %.1f%%", float64(sucCount)/float64(totalCount)*100)
	}
	if failCount > 0 {
		log.Printf("[Kiro] 失败明细:")
		if failRegistered > 0 {
			log.Printf("[Kiro]   邮箱已注册: %d (%.0f%%)", failRegistered, float64(failRegistered)/float64(totalCount)*100)
		}
		if failBanned > 0 {
			log.Printf("[Kiro]   账号封禁: %d (%.0f%%)", failBanned, float64(failBanned)/float64(totalCount)*100)
		}
		if failNetwork > 0 {
			log.Printf("[Kiro]   网络问题: %d (%.0f%%)", failNetwork, float64(failNetwork)/float64(totalCount)*100)
		}
		if failOther > 0 {
			log.Printf("[Kiro]   其他错误: %d (%.0f%%)", failOther, float64(failOther)/float64(totalCount)*100)
		}
	}
	if sucCount > 0 {
		log.Printf("[Kiro] 成功结果: %s", outDir)
	}
	log.Println("[Kiro] ═══════════════════════════════")
}
