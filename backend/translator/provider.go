package translator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ProviderType AI 提供商类型
type ProviderType string

const (
	ProviderOpenAI         ProviderType = "openai"
	ProviderClaude         ProviderType = "claude"
	ProviderGemini         ProviderType = "gemini"
	ProviderCustom         ProviderType = "custom"
	ProviderOllama         ProviderType = "ollama"
	ProviderDeepSeek       ProviderType = "deepseek"
	ProviderNLTranslate    ProviderType = "nltranslator"   // macOS NaturalLanguage 翻译
	ProviderLibreTranslate ProviderType = "libretranslate" // LibreTranslate 翻译
)

// Provider AI 提供商接口
type Provider interface {
	Translate(text, targetLanguage, userPrompt string) (string, error)
	GetName() string
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
	Type        ProviderType      `json:"type"`
	APIKey      string            `json:"apiKey"`
	APIURL      string            `json:"apiUrl"`
	Model       string            `json:"model"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"maxTokens"`
	Extra       map[string]string `json:"extra,omitempty"` // 额外参数
}

// BaseProvider 基础提供商实现
type BaseProvider struct {
	Config     ProviderConfig
	HTTPClient *http.Client
	Cache      *Cache
}

// NewProvider 创建提供商实例
func NewProvider(config ProviderConfig, cache *Cache) (Provider, error) {
	base := &BaseProvider{
		Config: config,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		Cache: cache,
	}

	switch config.Type {
	case ProviderOpenAI, ProviderDeepSeek:
		return &OpenAIProvider{BaseProvider: base}, nil
	case ProviderClaude:
		return &ClaudeProvider{BaseProvider: base}, nil
	case ProviderGemini:
		return &GeminiProvider{BaseProvider: base}, nil
	case ProviderOllama:
		return &OllamaProvider{BaseProvider: base}, nil
	case ProviderNLTranslate:
		return &NLTranslateProvider{BaseProvider: base}, nil
	case ProviderLibreTranslate:
		return &LibreTranslateProvider{BaseProvider: base}, nil
	case ProviderCustom:
		return &CustomProvider{BaseProvider: base}, nil
	default:
		return nil, fmt.Errorf("不支持的提供商类型: %s", config.Type)
	}
}

// doRequest 执行 HTTP 请求
func (b *BaseProvider) doRequest(req *http.Request) ([]byte, error) {
	resp, err := b.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误 (状态码 %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// checkCache 检查缓存
func (b *BaseProvider) checkCache(text, targetLanguage, userPrompt string) (string, bool) {
	if b.Cache != nil {
		cacheKey := CacheKey(text, targetLanguage, userPrompt)
		if cached, ok := b.Cache.Get(cacheKey); ok {
			return cached, true
		}
	}
	return "", false
}

// saveCache 保存到缓存
func (b *BaseProvider) saveCache(text, targetLanguage, userPrompt, result string) {
	if b.Cache != nil {
		cacheKey := CacheKey(text, targetLanguage, userPrompt)
		b.Cache.Set(cacheKey, result)
	}
}

// OpenAIProvider OpenAI 兼容的提供商（包括 OpenAI、DeepSeek 等）
type OpenAIProvider struct {
	*BaseProvider
}

func (p *OpenAIProvider) GetName() string {
	return string(p.Config.Type)
}

func (p *OpenAIProvider) Translate(text, targetLanguage, userPrompt string) (string, error) {
	// 检查缓存
	if cached, ok := p.checkCache(text, targetLanguage, userPrompt); ok {
		return cached, nil
	}

	systemPrompt := fmt.Sprintf("You are a professional translator. Translate the following text to %s. Keep the original meaning and style. Only return the translated text without any explanations.", targetLanguage)
	if userPrompt != "" {
		systemPrompt += " " + userPrompt
	}

	reqBody := map[string]interface{}{
		"model":       p.Config.Model,
		"temperature": p.Config.Temperature,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": text},
		},
	}

	if p.Config.MaxTokens > 0 {
		reqBody["max_tokens"] = p.Config.MaxTokens
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", p.Config.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.Config.APIKey)

	body, err := p.doRequest(req)
	if err != nil {
		return "", err
	}

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("API 错误: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("API 未返回翻译结果")
	}

	result := resp.Choices[0].Message.Content
	p.saveCache(text, targetLanguage, userPrompt, result)
	return result, nil
}

// NLTranslateProvider macOS NaturalLanguage 翻译提供商
type NLTranslateProvider struct {
	*BaseProvider
}

func (p *NLTranslateProvider) GetName() string {
	return "nltranslator"
}

func (p *NLTranslateProvider) Translate(text, targetLanguage, userPrompt string) (string, error) {
	// 检查缓存
	if cached, ok := p.checkCache(text, targetLanguage, userPrompt); ok {
		return cached, nil
	}

	// 映射目标语言到 NaturalLanguage 语言代码
	targetLangCode := mapToNLLanguageCode(targetLanguage)

	// 获取源语言，优先使用配置中的源语言
	var sourceLangCode string
	if p.Config.Extra != nil && p.Config.Extra["sourceLanguage"] != "" {
		sourceLangCode = mapToNLLanguageCode(p.Config.Extra["sourceLanguage"])
	} else {
		// 自动检测源语言或使用默认值
		sourceLangCode = detectSourceLanguage(text)
	}

	// 调用 NLTranslator Proxy API
	reqBody := map[string]interface{}{
		"text":           text,
		"sourceLanguage": sourceLangCode,
		"targetLanguage": targetLangCode,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", p.Config.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := p.doRequest(req)
	if err != nil {
		return "", err
	}

	var resp struct {
		TranslatedText string `json:"translatedText"`
		Error          string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("翻译错误: %s", resp.Error)
	}

	if resp.TranslatedText == "" {
		return "", fmt.Errorf("API 未返回翻译结果")
	}

	result := resp.TranslatedText
	p.saveCache(text, targetLanguage, userPrompt, result)
	return result, nil
}

// mapToNLLanguageCode 将常见语言名称映射到 NaturalLanguage 语言代码
func mapToNLLanguageCode(language string) string {
	languageMap := map[string]string{
		"Chinese":             "zh-Hans",
		"Simplified Chinese":  "zh-Hans",
		"简体中文":                "zh-Hans",
		"中文":                  "zh-Hans",
		"Traditional Chinese": "zh-Hant",
		"繁体中文":                "zh-Hant",
		"繁體中文":                "zh-Hant",
		"English":             "en",
		"英语":                  "en",
		"英文":                  "en",
		"Japanese":            "ja",
		"日语":                  "ja",
		"日文":                  "ja",
		"Korean":              "ko",
		"韩语":                  "ko",
		"韓語":                  "ko",
		"Spanish":             "es",
		"西班牙语":                "es",
		"French":              "fr",
		"法语":                  "fr",
		"German":              "de",
		"德语":                  "de",
	}

	if code, ok := languageMap[language]; ok {
		return code
	}

	// 如果已经是语言代码格式，直接返回
	return language
}

// ClaudeProvider Anthropic Claude 提供商
type ClaudeProvider struct {
	*BaseProvider
}

func (p *ClaudeProvider) GetName() string {
	return "claude"
}

func (p *ClaudeProvider) Translate(text, targetLanguage, userPrompt string) (string, error) {
	// 检查缓存
	if cached, ok := p.checkCache(text, targetLanguage, userPrompt); ok {
		return cached, nil
	}

	systemPrompt := fmt.Sprintf("You are a professional translator. Translate the following text to %s. Keep the original meaning and style. Only return the translated text without any explanations.", targetLanguage)
	if userPrompt != "" {
		systemPrompt += " " + userPrompt
	}

	reqBody := map[string]interface{}{
		"model":       p.Config.Model,
		"max_tokens":  p.Config.MaxTokens,
		"temperature": p.Config.Temperature,
		"system":      systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": text},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", p.Config.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.Config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	body, err := p.doRequest(req)
	if err != nil {
		return "", err
	}

	var resp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("API 错误: %s", resp.Error.Message)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("API 未返回翻译结果")
	}

	result := resp.Content[0].Text
	p.saveCache(text, targetLanguage, userPrompt, result)
	return result, nil
}

// GeminiProvider Google Gemini 提供商
type GeminiProvider struct {
	*BaseProvider
}

func (p *GeminiProvider) GetName() string {
	return "gemini"
}

func (p *GeminiProvider) Translate(text, targetLanguage, userPrompt string) (string, error) {
	// 检查缓存
	if cached, ok := p.checkCache(text, targetLanguage, userPrompt); ok {
		return cached, nil
	}

	systemPrompt := fmt.Sprintf("You are a professional translator. Translate the following text to %s. Keep the original meaning and style. Only return the translated text without any explanations.", targetLanguage)
	if userPrompt != "" {
		systemPrompt += " " + userPrompt
	}

	fullPrompt := systemPrompt + "\n\n" + text

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": fullPrompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": p.Config.Temperature,
		},
	}

	if p.Config.MaxTokens > 0 {
		reqBody["generationConfig"].(map[string]interface{})["maxOutputTokens"] = p.Config.MaxTokens
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Gemini API URL 格式: https://generativelanguage.googleapis.com/v1/models/{model}:generateContent?key={apiKey}
	apiURL := fmt.Sprintf("%s?key=%s", p.Config.APIURL, p.Config.APIKey)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := p.doRequest(req)
	if err != nil {
		return "", err
	}

	var resp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("API 错误: %s", resp.Error.Message)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("API 未返回翻译结果")
	}

	result := resp.Candidates[0].Content.Parts[0].Text
	p.saveCache(text, targetLanguage, userPrompt, result)
	return result, nil
}

// OllamaProvider Ollama 本地模型提供商
type OllamaProvider struct {
	*BaseProvider
}

func (p *OllamaProvider) GetName() string {
	return "ollama"
}

func (p *OllamaProvider) Translate(text, targetLanguage, userPrompt string) (string, error) {
	// 检查缓存
	if cached, ok := p.checkCache(text, targetLanguage, userPrompt); ok {
		return cached, nil
	}

	systemPrompt := fmt.Sprintf("You are a professional translator. Translate the following text to %s. Keep the original meaning and style. Only return the translated text without any explanations.", targetLanguage)
	if userPrompt != "" {
		systemPrompt += " " + userPrompt
	}

	reqBody := map[string]interface{}{
		"model":  p.Config.Model,
		"prompt": systemPrompt + "\n\n" + text,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": p.Config.Temperature,
		},
	}

	if p.Config.MaxTokens > 0 {
		reqBody["options"].(map[string]interface{})["num_predict"] = p.Config.MaxTokens
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", p.Config.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := p.doRequest(req)
	if err != nil {
		return "", err
	}

	var resp struct {
		Response string `json:"response"`
		Error    string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("API 错误: %s", resp.Error)
	}

	if resp.Response == "" {
		return "", fmt.Errorf("API 未返回翻译结果")
	}

	result := resp.Response
	p.saveCache(text, targetLanguage, userPrompt, result)
	return result, nil
}

// CustomProvider 自定义 API 提供商
type CustomProvider struct {
	*BaseProvider
}

func (p *CustomProvider) GetName() string {
	return "custom"
}

func (p *CustomProvider) Translate(text, targetLanguage, userPrompt string) (string, error) {
	// 检查缓存
	if cached, ok := p.checkCache(text, targetLanguage, userPrompt); ok {
		return cached, nil
	}

	systemPrompt := fmt.Sprintf("You are a professional translator. Translate the following text to %s. Keep the original meaning and style. Only return the translated text without any explanations.", targetLanguage)
	if userPrompt != "" {
		systemPrompt += " " + userPrompt
	}

	// 自定义提供商使用 OpenAI 兼容格式作为默认
	reqBody := map[string]interface{}{
		"model":       p.Config.Model,
		"temperature": p.Config.Temperature,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": text},
		},
	}

	if p.Config.MaxTokens > 0 {
		reqBody["max_tokens"] = p.Config.MaxTokens
	}

	// 添加额外参数
	for k, v := range p.Config.Extra {
		reqBody[k] = v
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", p.Config.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if p.Config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.Config.APIKey)
	}

	body, err := p.doRequest(req)
	if err != nil {
		return "", err
	}

	// 尝试解析 OpenAI 格式
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("API 错误: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("API 未返回翻译结果")
	}

	result := resp.Choices[0].Message.Content
	p.saveCache(text, targetLanguage, userPrompt, result)
	return result, nil
}

// detectSourceLanguage 简单的源语言检测
func detectSourceLanguage(text string) string {
	// 简单的语言检测逻辑
	// 检测中文字符
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			return "zh-Hans" // 中文
		}
	}

	// 检测日文字符
	for _, r := range text {
		if (r >= 0x3040 && r <= 0x309f) || (r >= 0x30a0 && r <= 0x30ff) {
			return "ja" // 日文
		}
	}

	// 检测韩文字符
	for _, r := range text {
		if r >= 0xac00 && r <= 0xd7af {
			return "ko" // 韩文
		}
	}

	// 默认为英文
	return "en"
}

// LibreTranslateProvider LibreTranslate 提供商
type LibreTranslateProvider struct {
	*BaseProvider
}

func (p *LibreTranslateProvider) GetName() string {
	return "libretranslate"
}

func (p *LibreTranslateProvider) Translate(text, targetLanguage, userPrompt string) (string, error) {
	// 检查缓存
	if cached, ok := p.checkCache(text, targetLanguage, userPrompt); ok {
		return cached, nil
	}

	// 映射目标语言到 LibreTranslate 语言代码
	targetLangCode := mapToLibreTranslateLanguageCode(targetLanguage)

	// 获取源语言，优先使用配置中的源语言
	var sourceLangCode string
	if p.Config.Extra != nil && p.Config.Extra["sourceLanguage"] != "" {
		sourceLangCode = mapToLibreTranslateLanguageCode(p.Config.Extra["sourceLanguage"])
	} else {
		// 自动检测源语言
		sourceLangCode = "auto"
	}

	// 构建请求体
	reqBody := map[string]interface{}{
		"q":      text,
		"source": sourceLangCode,
		"target": targetLangCode,
		"format": "text",
	}

	// 如果配置了 API Key，添加到请求中
	if p.Config.APIKey != "" {
		reqBody["api_key"] = p.Config.APIKey
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", p.Config.APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := p.doRequest(req)
	if err != nil {
		return "", err
	}

	var resp struct {
		TranslatedText string `json:"translatedText"`
		Error          string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("翻译错误: %s", resp.Error)
	}

	if resp.TranslatedText == "" {
		return "", fmt.Errorf("API 未返回翻译结果")
	}

	result := resp.TranslatedText
	p.saveCache(text, targetLanguage, userPrompt, result)
	return result, nil
}

// mapToLibreTranslateLanguageCode 将常见语言名称映射到 LibreTranslate 语言代码
func mapToLibreTranslateLanguageCode(language string) string {
	languageMap := map[string]string{
		"Chinese":             "zh",
		"Simplified Chinese":  "zh",
		"简体中文":                "zh",
		"中文":                  "zh",
		"Traditional Chinese": "zh",
		"繁体中文":                "zh",
		"繁體中文":                "zh",
		"English":             "en",
		"英语":                  "en",
		"英文":                  "en",
		"Japanese":            "ja",
		"日语":                  "ja",
		"日文":                  "ja",
		"Korean":              "ko",
		"韩语":                  "ko",
		"韓語":                  "ko",
		"Spanish":             "es",
		"西班牙语":                "es",
		"French":              "fr",
		"法语":                  "fr",
		"German":              "de",
		"德语":                  "de",
		"Italian":             "it",
		"意大利语":                "it",
		"Portuguese":          "pt",
		"葡萄牙语":                "pt",
		"Russian":             "ru",
		"俄语":                  "ru",
		"Arabic":              "ar",
		"阿拉伯语":                "ar",
		"Hindi":               "hi",
		"印地语":                 "hi",
	}

	if code, ok := languageMap[language]; ok {
		return code
	}

	// 如果已经是语言代码格式，直接返回
	return language
}
