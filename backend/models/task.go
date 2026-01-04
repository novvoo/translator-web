package models

import "time"

type TranslateTask struct {
	ID             string    `json:"id"`
	SourceFile     string    `json:"sourceFile"`
	TargetLanguage string    `json:"targetLanguage"`
	Status         string    `json:"status"` // pending, processing, completed, failed
	Progress       float64   `json:"progress"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	CompletedAt    time.Time `json:"completedAt,omitempty"`
	OutputPath     string    `json:"outputPath,omitempty"`
}

type LLMConfig struct {
	Provider    string            `json:"provider"` // openai, claude, gemini, ollama, deepseek, custom
	APIKey      string            `json:"apiKey"`
	APIURL      string            `json:"apiUrl"`
	Model       string            `json:"model"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"maxTokens"`
	Extra       map[string]string `json:"extra,omitempty"` // 额外参数，用于自定义提供商
}

type TranslateRequest struct {
	TargetLanguage   string    `json:"targetLanguage"`
	LLMConfig        LLMConfig `json:"llmConfig"`
	UserPrompt       string    `json:"userPrompt,omitempty"`
	ForceRetranslate bool      `json:"forceRetranslate,omitempty"` // 是否强制重新翻译（忽略缓存）
}
