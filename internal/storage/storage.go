package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"reg_go/internal/crypto"
)

var (
	_dataDir     string
	_dataDirOnce sync.Once
	_customDir   string
)

// GetDefaultDataDir 获取默认应用数据目录
func GetDefaultDataDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	return filepath.Join(configDir, "kiro-reg")
}

// getConfigFilePath 获取配置文件路径（始终在默认目录下）
func getConfigFilePath() string {
	return filepath.Join(GetDefaultDataDir(), "storage.conf")
}

// loadCustomDir 从配置文件读取自定义目录
func loadCustomDir() string {
	data, err := os.ReadFile(getConfigFilePath())
	if err != nil {
		return ""
	}
	dir := strings.TrimSpace(string(data))
	if dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}

// GetDataDir 获取应用数据目录（优先使用自定义目录）
func GetDataDir() string {
	_dataDirOnce.Do(func() {
		_customDir = loadCustomDir()
		if _customDir != "" {
			_dataDir = _customDir
		} else {
			_dataDir = GetDefaultDataDir()
		}
		os.MkdirAll(_dataDir, 0755)
	})
	return _dataDir
}

// SetDataDirPath 设置自定义存储目录（自动迁移旧数据）
func SetDataDirPath(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("目录不能为空")
	}

	oldDir := _dataDir

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	if oldDir != "" && oldDir != dir {
		migrated, migErr := migrateData(oldDir, dir)
		if migErr != nil {
			return "", fmt.Errorf("数据迁移失败: %w", migErr)
		}
		if migrated > 0 {
			log.Printf("已迁移 %d 个数据文件: %s → %s", migrated, oldDir, dir)
		}
	}

	os.MkdirAll(GetDefaultDataDir(), 0755)
	if err := os.WriteFile(getConfigFilePath(), []byte(dir), 0600); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}

	_customDir = dir
	_dataDir = dir
	_dataDirOnce = sync.Once{}
	_dataDirOnce.Do(func() {})

	return dir, nil
}

// ResetDataDirPath 重置为默认存储目录（自动迁移数据回默认目录）
func ResetDataDirPath() string {
	oldDir := _dataDir
	defaultDir := GetDefaultDataDir()

	if oldDir != "" && oldDir != defaultDir {
		migrated, _ := migrateData(oldDir, defaultDir)
		if migrated > 0 {
			log.Printf("已迁移 %d 个数据文件: %s → %s", migrated, oldDir, defaultDir)
		}
	}

	os.Remove(getConfigFilePath())
	os.MkdirAll(defaultDir, 0755)
	_customDir = ""
	_dataDir = defaultDir
	_dataDirOnce = sync.Once{}
	_dataDirOnce.Do(func() {})

	return defaultDir
}

// migrateData 将旧目录中的数据文件迁移到新目录，返回迁移文件数
func migrateData(oldDir, newDir string) (int, error) {
	migrated := 0

	items := []string{
		"accounts.dat",
		"kiro",
	}

	for _, item := range items {
		src := filepath.Join(oldDir, item)
		dst := filepath.Join(newDir, item)

		info, err := os.Stat(src)
		if err != nil {
			continue
		}

		if info.IsDir() {
			n, err := migrateDir(src, dst)
			if err != nil {
				return migrated, err
			}
			migrated += n
		} else {
			if _, err := os.Stat(dst); err != nil {
				data, err := os.ReadFile(src)
				if err != nil {
					return migrated, err
				}
				os.MkdirAll(filepath.Dir(dst), 0755)
				if err := os.WriteFile(dst, data, 0600); err != nil {
					return migrated, err
				}
				migrated++
			}
		}
	}

	return migrated, nil
}

func migrateDir(srcDir, dstDir string) (int, error) {
	migrated := 0
	os.MkdirAll(dstDir, 0755)

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return 0, err
	}

	for _, e := range entries {
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(dstDir, e.Name())

		if e.IsDir() {
			n, err := migrateDir(src, dst)
			if err != nil {
				return migrated, err
			}
			migrated += n
		} else if strings.HasSuffix(e.Name(), ".dat") {
			if _, err := os.Stat(dst); err != nil {
				data, err := os.ReadFile(src)
				if err != nil {
					return migrated, err
				}
				if err := os.WriteFile(dst, data, 0600); err != nil {
					return migrated, err
				}
				migrated++
			}
		}
	}

	return migrated, nil
}

// GetAccountsPath 获取微软邮箱账号文件路径
func GetAccountsPath() string {
	return filepath.Join(GetDataDir(), "accounts.dat")
}

// ===== Accounts 内存缓存（消除并发文件 I/O 瓶颈）=====

var (
	_accountsCache  []map[string]interface{}
	_accountsMu     sync.RWMutex
	_accountsLoaded bool
	_accountsDirty  bool
	_flushTimer     *time.Timer
)

func loadAccountsCache() {
	if _accountsLoaded {
		return
	}
	data, err := loadEncryptedJSONDirect(GetAccountsPath())
	if err != nil {
		_accountsCache = []map[string]interface{}{}
	} else {
		_accountsCache = data
	}
	_accountsLoaded = true
}

// GetAccountsCached 获取账号列表（从内存缓存）
func GetAccountsCached() []map[string]interface{} {
	_accountsMu.Lock()
	if !_accountsLoaded {
		loadAccountsCache()
	}
	result := make([]map[string]interface{}, len(_accountsCache))
	copy(result, _accountsCache)
	_accountsMu.Unlock()
	return result
}

// SetAccountsCached 替换账号列表并触发异步刷盘
func SetAccountsCached(accounts []map[string]interface{}) {
	_accountsMu.Lock()
	_accountsCache = accounts
	_accountsLoaded = true
	_accountsDirty = true
	scheduleFlush()
	_accountsMu.Unlock()
}

// ModifyAccountsCached 原子修改账号列表（回调在锁内执行，高效无文件 I/O）
func ModifyAccountsCached(fn func([]map[string]interface{}) []map[string]interface{}) {
	_accountsMu.Lock()
	if !_accountsLoaded {
		loadAccountsCache()
	}
	_accountsCache = fn(_accountsCache)
	_accountsDirty = true
	scheduleFlush()
	_accountsMu.Unlock()
}

func scheduleFlush() {
	if _flushTimer != nil {
		_flushTimer.Stop()
	}
	_flushTimer = time.AfterFunc(500*time.Millisecond, flushAccountsToDisk)
}

func flushAccountsToDisk() {
	_accountsMu.RLock()
	if !_accountsDirty {
		_accountsMu.RUnlock()
		return
	}
	data := make([]map[string]interface{}, len(_accountsCache))
	copy(data, _accountsCache)
	_accountsMu.RUnlock()

	err := SaveEncryptedJSON(GetAccountsPath(), data)

	_accountsMu.Lock()
	if err == nil {
		_accountsDirty = false
	}
	_accountsMu.Unlock()
}

// FlushAccountsSync 同步刷盘（程序退出前调用）
func FlushAccountsSync() {
	if _flushTimer != nil {
		_flushTimer.Stop()
	}
	flushAccountsToDisk()
}

// GetKiroDir 获取 Kiro 结果目录
func GetKiroDir() string {
	dir := filepath.Join(GetDataDir(), "kiro")
	os.MkdirAll(dir, 0755)
	return dir
}

// ===== 加密存储读写 =====

var fileMutexes sync.Map

func getFileMutex(filePath string) *sync.Mutex {
	val, _ := fileMutexes.LoadOrStore(filePath, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// LoadEncryptedJSON 从加密文件读取 JSON 数组（线程安全）
func LoadEncryptedJSON(filePath string) ([]map[string]interface{}, error) {
	mu := getFileMutex(filePath)
	mu.Lock()
	defer mu.Unlock()
	return loadEncryptedJSONUnsafe(filePath)
}

// SaveEncryptedJSON 将 JSON 数组加密写入文件（线程安全，原子写入）
func SaveEncryptedJSON(filePath string, items []map[string]interface{}) error {
	mu := getFileMutex(filePath)
	mu.Lock()
	defer mu.Unlock()
	return saveEncryptedJSONUnsafe(filePath, items)
}

// AppendEncryptedJSON 向加密 JSON 数组文件追加一条记录（线程安全）
func AppendEncryptedJSON(filePath string, item map[string]interface{}) error {
	mu := getFileMutex(filePath)
	mu.Lock()
	defer mu.Unlock()

	existing, _ := loadEncryptedJSONUnsafe(filePath)
	existing = append(existing, item)
	return saveEncryptedJSONUnsafe(filePath, existing)
}

// ModifyEncryptedJSON 原子读-改-写
func ModifyEncryptedJSON(filePath string, fn func([]map[string]interface{}) []map[string]interface{}) error {
	mu := getFileMutex(filePath)
	mu.Lock()
	defer mu.Unlock()

	existing, _ := loadEncryptedJSONUnsafe(filePath)
	modified := fn(existing)
	return saveEncryptedJSONUnsafe(filePath, modified)
}

// CountEncryptedJSON 统计加密 JSON 数组文件中的记录数
func CountEncryptedJSON(filePath string) int {
	items, err := LoadEncryptedJSON(filePath)
	if err != nil {
		return 0
	}
	return len(items)
}

// --- 内部无锁版本 ---

func loadEncryptedJSONDirect(filePath string) ([]map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	decrypted, err := crypto.DecryptLocal(string(data))
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(decrypted), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func loadEncryptedJSONUnsafe(filePath string) ([]map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	decrypted, err := crypto.DecryptLocal(string(data))
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(decrypted), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func saveEncryptedJSONUnsafe(filePath string, items []map[string]interface{}) error {
	b, err := json.Marshal(items)
	if err != nil {
		return err
	}

	encrypted, err := crypto.EncryptLocal(string(b))
	if err != nil {
		return err
	}

	os.MkdirAll(filepath.Dir(filePath), 0755)

	tmpFile := filePath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(encrypted), 0600); err != nil {
		return err
	}
	return os.Rename(tmpFile, filePath)
}
