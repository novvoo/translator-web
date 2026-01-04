package translator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Cache 翻译缓存
type Cache struct {
	dir      string
	mutex    sync.RWMutex
	disabled bool // 是否禁用缓存
}

// NewCache 创建缓存
func NewCache(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Cache{dir: dir, disabled: false}, nil
}

// DisableCache 禁用缓存（用于强制重新翻译）
func (c *Cache) DisableCache() {
	c.disabled = true
}

// EnableCache 启用缓存
func (c *Cache) EnableCache() {
	c.disabled = false
}

// Get 获取缓存
func (c *Cache) Get(key string) (string, bool) {
	if c.disabled {
		return "", false
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	hash := c.hashKey(key)
	path := filepath.Join(c.dir, hash+".txt")

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	return string(data), true
}

// Set 设置缓存
func (c *Cache) Set(key, value string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	hash := c.hashKey(key)
	path := filepath.Join(c.dir, hash+".txt")

	return os.WriteFile(path, []byte(value), 0644)
}

// hashKey 计算缓存键的哈希
func (c *Cache) hashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// CacheKey 生成缓存键
func CacheKey(text, targetLanguage, userPrompt string) string {
	data := map[string]string{
		"text":           text,
		"targetLanguage": targetLanguage,
		"userPrompt":     userPrompt,
	}
	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}
