package translator

import (
	"fmt"
	"time"
)

// TranslatorClient 翻译客户端（支持多提供商）
type TranslatorClient struct {
	Provider      Provider
	RetryTimes    int
	RetryInterval time.Duration
}

// NewTranslatorClient 创建翻译客户端
func NewTranslatorClient(config ProviderConfig, cache *Cache) (*TranslatorClient, error) {
	provider, err := NewProvider(config, cache)
	if err != nil {
		return nil, err
	}

	return &TranslatorClient{
		Provider:      provider,
		RetryTimes:    5,
		RetryInterval: 2 * time.Second,
	}, nil
}

// WithRetry 设置重试参数
func (c *TranslatorClient) WithRetry(times int, interval time.Duration) *TranslatorClient {
	c.RetryTimes = times
	c.RetryInterval = interval
	return c
}

// Translate 翻译文本（带重试）
func (c *TranslatorClient) Translate(text, targetLanguage, userPrompt string) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= c.RetryTimes; attempt++ {
		if attempt > 0 {
			time.Sleep(c.RetryInterval)
		}

		result, err := c.Provider.Translate(text, targetLanguage, userPrompt)
		if err == nil {
			return result, nil
		}

		lastErr = err
	}

	return "", fmt.Errorf("翻译失败（重试 %d 次后）: %w", c.RetryTimes, lastErr)
}

// TranslateBatch 批量翻译
func (c *TranslatorClient) TranslateBatch(texts []string, targetLanguage, userPrompt string) ([]string, error) {
	results := make([]string, len(texts))

	for i, text := range texts {
		if text == "" {
			results[i] = ""
			continue
		}

		translated, err := c.Translate(text, targetLanguage, userPrompt)
		if err != nil {
			return nil, fmt.Errorf("翻译第 %d 段失败: %w", i+1, err)
		}
		results[i] = translated

		// 避免请求过快
		time.Sleep(100 * time.Millisecond)
	}

	return results, nil
}
