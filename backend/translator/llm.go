package translator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LLMClient LLM 客户端
type LLMClient struct {
	APIKey        string
	APIURL        string
	Model         string
	Temperature   float64
	MaxTokens     int
	RetryTimes    int
	RetryInterval time.Duration
	HTTPClient    *http.Client
	Cache         *Cache
}

// NewLLMClient 创建 LLM 客户端
func NewLLMClient(apiKey, apiURL, model string, temperature float64, maxTokens int) *LLMClient {
	return &LLMClient{
		APIKey:        apiKey,
		APIURL:        apiURL,
		Model:         model,
		Temperature:   temperature,
		MaxTokens:     maxTokens,
		RetryTimes:    5,
		RetryInterval: 2 * time.Second,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// WithCache 设置缓存
func (c *LLMClient) WithCache(cache *Cache) *LLMClient {
	c.Cache = cache
	return c
}

// WithRetry 设置重试参数
func (c *LLMClient) WithRetry(times int, interval time.Duration) *LLMClient {
	c.RetryTimes = times
	c.RetryInterval = interval
	return c
}

// OpenAI API 请求结构
type openAIRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Translate 翻译文本（带缓存和重试）
func (c *LLMClient) Translate(text, targetLanguage, userPrompt string) (string, error) {
	// 检查缓存
	if c.Cache != nil {
		cacheKey := CacheKey(text, targetLanguage, userPrompt)
		if cached, ok := c.Cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// 执行翻译（带重试）
	var lastErr error
	for attempt := 0; attempt <= c.RetryTimes; attempt++ {
		if attempt > 0 {
			time.Sleep(c.RetryInterval)
		}

		result, err := c.translateOnce(text, targetLanguage, userPrompt)
		if err == nil {
			// 保存到缓存
			if c.Cache != nil {
				cacheKey := CacheKey(text, targetLanguage, userPrompt)
				c.Cache.Set(cacheKey, result)
			}
			return result, nil
		}

		lastErr = err
	}

	return "", fmt.Errorf("翻译失败（重试 %d 次后）: %w", c.RetryTimes, lastErr)
}

// translateOnce 执行一次翻译请求
func (c *LLMClient) translateOnce(text, targetLanguage, userPrompt string) (string, error) {
	systemPrompt := fmt.Sprintf("You are a professional translator. Translate the following text to %s. Keep the original meaning and style. Only return the translated text without any explanations.", targetLanguage)

	if userPrompt != "" {
		systemPrompt += " " + userPrompt
	}

	req := openAIRequest{
		Model:       c.Model,
		Temperature: c.Temperature,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
	}

	if c.MaxTokens > 0 {
		req.MaxTokens = c.MaxTokens
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest("POST", c.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 返回错误 (状态码 %d): %s", resp.StatusCode, string(body))
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("API 错误: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("API 未返回翻译结果")
	}

	return apiResp.Choices[0].Message.Content, nil
}

// TranslateBatch 批量翻译
func (c *LLMClient) TranslateBatch(texts []string, targetLanguage, userPrompt string) ([]string, error) {
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
