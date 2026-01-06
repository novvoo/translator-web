package translator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// PDFMathTranslator PDF数学翻译器（Go原生实现）
type PDFMathTranslator struct {
	Parser      *PDFParser
	Generator   *PDFGenerator
	FontPath    string
	Integration *PDFTranslatorIntegration
}

// PDFMathConfig PDFMathTranslate配置
type PDFMathConfig struct {
	LangIn          string            `json:"lang_in"`
	LangOut         string            `json:"lang_out"`
	Service         string            `json:"service"`
	Thread          int               `json:"thread"`
	Pages           string            `json:"pages,omitempty"`
	Output          string            `json:"output"`
	SkipSubsetFonts bool              `json:"skip_subset_fonts"`
	IgnoreCache     bool              `json:"ignore_cache"`
	Compatible      bool              `json:"compatible"`
	Prompt          string            `json:"prompt,omitempty"`
	GenerateMode    string            `json:"generate_mode,omitempty"` // 新增：生成模式
	Envs            map[string]string `json:"envs,omitempty"`
}

// PDFMathResult PDFMathTranslate结果
type PDFMathResult struct {
	MonoFile string `json:"mono_file"`
	DualFile string `json:"dual_file"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// NewPDFMathTranslator 创建PDF数学翻译器
func NewPDFMathTranslator() *PDFMathTranslator {
	return &PDFMathTranslator{
		Parser:      NewPDFParser("", ""), // 可配置公式检测规则
		Generator:   NewPDFGenerator(""),  // 可配置字体路径
		FontPath:    "",                   // 将根据目标语言自动选择
		Integration: nil,                  // 将在需要时设置
	}
}

// SetTranslatorClient 设置翻译客户端
func (pmt *PDFMathTranslator) SetTranslatorClient(client *TranslatorClient) {
	pmt.Integration = NewPDFTranslatorIntegration(client)
}

// TranslatePDF 使用Go原生实现翻译PDF
func (pmt *PDFMathTranslator) TranslatePDF(inputPath, outputDir string, config PDFMathConfig, progressCallback func(float64)) (*PDFMathResult, error) {
	log.Printf("开始使用Go原生实现翻译PDF: %s", inputPath)

	// 准备输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 设置字体路径
	pmt.setupFont(config.LangOut)

	// 1. 解析PDF
	if progressCallback != nil {
		progressCallback(0.1)
	}

	content, err := pmt.Parser.ParsePDF(inputPath)
	if err != nil {
		return nil, fmt.Errorf("解析PDF失败: %w", err)
	}

	// 2. 提取需要翻译的文本
	if progressCallback != nil {
		progressCallback(0.2)
	}

	texts := pmt.Parser.GetTextForTranslation(content)
	if len(texts) == 0 {
		return nil, fmt.Errorf("PDF中没有可翻译的文本")
	}

	// 3. 执行翻译
	if progressCallback != nil {
		progressCallback(0.3)
	}

	translations, err := pmt.translateTexts(texts, config)
	if err != nil {
		return nil, fmt.Errorf("翻译失败: %w", err)
	}

	// 4. 应用翻译结果
	if progressCallback != nil {
		progressCallback(0.7)
	}

	translatedContent := *content // 复制原内容
	pmt.Parser.ApplyTranslations(&translatedContent, translations)

	// 5. 生成输出文件
	if progressCallback != nil {
		progressCallback(0.8)
	}

	filename := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))

	// 生成PDF配置
	pdfConfig := BilingualPDFConfig{
		Title:        content.Metadata["title"],
		Author:       content.Metadata["author"],
		Subject:      content.Metadata["subject"],
		Creator:      "PDF Math Translate (Go)",
		SourceLang:   config.LangIn,
		TargetLang:   config.LangOut,
		ShowOriginal: true,
		FontSize:     12,
		LineSpacing:  6,
		Margin:       20,
	}

	// 根据生成模式决定生成哪些文件
	var monoFile, dualFile string

	if config.GenerateMode == "monolingual" {
		// 单语模式：只生成单语PDF
		monoFile = filepath.Join(outputDir, filename+"-mono.pdf")
		if err := pmt.Generator.GenerateMonolingualPDF(&translatedContent, monoFile, pdfConfig); err != nil {
			log.Printf("警告：生成单语PDF失败: %v", err)
		}
		log.Printf("单语模式：生成单语PDF: %s", monoFile)
	} else {
		// 双语模式（默认）：生成双语PDF，可选生成单语PDF
		dualFile = filepath.Join(outputDir, filename+"-dual.pdf")
		if err := pmt.Generator.GenerateBilingualPDF(content, &translatedContent, dualFile, pdfConfig); err != nil {
			log.Printf("警告：生成双语PDF失败: %v", err)
		}

		// 也生成单语版本作为备选
		monoFile = filepath.Join(outputDir, filename+"-mono.pdf")
		if err := pmt.Generator.GenerateMonolingualPDF(&translatedContent, monoFile, pdfConfig); err != nil {
			log.Printf("警告：生成单语PDF失败: %v", err)
		}
		log.Printf("双语模式：生成双语PDF: %s 和单语PDF: %s", dualFile, monoFile)
	}

	if progressCallback != nil {
		progressCallback(1.0)
	}

	result := &PDFMathResult{
		MonoFile: monoFile,
		DualFile: dualFile,
		Success:  true,
	}

	log.Printf("PDF翻译完成: mono=%s, dual=%s", result.MonoFile, result.DualFile)
	return result, nil
}

// setupFont 设置字体路径
func (pmt *PDFMathTranslator) setupFont(langOut string) {
	// 根据目标语言选择合适的字体
	fontMap := map[string]string{
		"zh": "fonts/SourceHanSerif-Regular.ttf",
		"ja": "fonts/SourceHanSerif-Regular.ttf",
		"ko": "fonts/SourceHanSerif-Regular.ttf",
		"ar": "fonts/NotoSansArabic-Regular.ttf",
		"hi": "fonts/NotoSansDevanagari-Regular.ttf",
		"th": "fonts/NotoSansThai-Regular.ttf",
	}

	if fontPath, exists := fontMap[langOut]; exists {
		pmt.FontPath = fontPath
		pmt.Generator.FontPath = fontPath
	}
}

// translateTexts 翻译文本列表
func (pmt *PDFMathTranslator) translateTexts(texts []string, config PDFMathConfig) (map[string]string, error) {
	translations := make(map[string]string)

	// 如果有集成的翻译客户端，使用它进行翻译
	if pmt.Integration != nil && pmt.Integration.Client != nil {
		// 使用集成的翻译客户端
		targetLanguage := pmt.mapLanguageCode(config.LangOut)
		return pmt.Integration.TranslateTexts(texts, targetLanguage, config.Prompt, nil)
	}

	// 否则返回模拟翻译结果
	log.Printf("警告：没有可用的翻译客户端，返回模拟翻译结果")
	for _, text := range texts {
		translations[text] = "[翻译] " + text
	}

	return translations, nil
}

// mapLanguageCode 映射语言代码
func (pmt *PDFMathTranslator) mapLanguageCode(langCode string) string {
	mapping := map[string]string{
		"zh":    "Chinese",
		"zh-cn": "Chinese",
		"zh-tw": "Chinese",
		"en":    "English",
		"ja":    "Japanese",
		"ko":    "Korean",
		"fr":    "French",
		"de":    "German",
		"es":    "Spanish",
		"ru":    "Russian",
		"ar":    "Arabic",
		"pt":    "Portuguese",
	}

	if language, ok := mapping[strings.ToLower(langCode)]; ok {
		return language
	}
	return "Chinese" // 默认中文
}

// GetSupportedServices 获取支持的翻译服务
func (pmt *PDFMathTranslator) GetSupportedServices() []string {
	return []string{
		"openai",
		"claude",
		"gemini",
		"deepseek",
		"ollama",
		"custom",
	}
}

// MapProviderToService 将前端提供商映射到翻译服务
func (pmt *PDFMathTranslator) MapProviderToService(provider string) string {
	mapping := map[string]string{
		"openai":         "openai",
		"claude":         "claude",
		"gemini":         "gemini",
		"deepseek":       "deepseek",
		"ollama":         "ollama",
		"custom":         "custom",
		"nltranslator":   "openai", // 回退到openai
		"libretranslate": "openai", // 回退到openai
	}

	if service, ok := mapping[provider]; ok {
		return service
	}
	return "openai" // 默认服务
}

// BuildEnvs 构建环境变量（保留兼容性）
func (pmt *PDFMathTranslator) BuildEnvs(config ProviderConfig) map[string]string {
	envs := make(map[string]string)

	switch config.Type {
	case ProviderOpenAI:
		envs["OPENAI_API_KEY"] = config.APIKey
		envs["OPENAI_BASE_URL"] = config.APIURL
		envs["OPENAI_MODEL"] = config.Model

	case ProviderClaude:
		envs["ANTHROPIC_API_KEY"] = config.APIKey
		envs["ANTHROPIC_MODEL"] = config.Model

	case ProviderGemini:
		envs["GEMINI_API_KEY"] = config.APIKey
		envs["GEMINI_MODEL"] = config.Model

	case ProviderDeepSeek:
		envs["DEEPSEEK_API_KEY"] = config.APIKey
		envs["DEEPSEEK_MODEL"] = config.Model

	case ProviderOllama:
		envs["OLLAMA_HOST"] = config.APIURL
		envs["OLLAMA_MODEL"] = config.Model

	case ProviderCustom:
		envs["CUSTOM_API_URL"] = config.APIURL
		envs["CUSTOM_API_KEY"] = config.APIKey
		envs["CUSTOM_MODEL"] = config.Model
	}

	return envs
}

// SetEnvironmentVariables 设置环境变量（保留兼容性）
func (pmt *PDFMathTranslator) SetEnvironmentVariables(envs map[string]string) {
	for key, value := range envs {
		os.Setenv(key, value)
	}
}

// ValidatePDFMathConfig 验证PDF翻译配置
func (pmt *PDFMathTranslator) ValidatePDFMathConfig(config PDFMathConfig) error {
	if config.LangIn == "" {
		return fmt.Errorf("源语言不能为空")
	}
	if config.LangOut == "" {
		return fmt.Errorf("目标语言不能为空")
	}
	if config.Service == "" {
		return fmt.Errorf("翻译服务不能为空")
	}
	if config.Thread <= 0 {
		config.Thread = 4 // 默认值
	}
	return nil
}

// IsPDFMathTranslateAvailable 检查PDF翻译功能是否可用
func IsPDFMathTranslateAvailable() bool {
	// Go原生实现总是可用
	return true
}
