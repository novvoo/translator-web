package translator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// PDFMathTranslator PDF数学翻译器（Go原生实现）- 使用文本替换保留样式
type PDFMathTranslator struct {
	Parser      *PDFParser
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
		// 检查是否是PDF格式问题，提供更友好的错误信息
		if strings.Contains(err.Error(), "stream not present") || strings.Contains(err.Error(), "PDF文件格式不受支持") {
			return nil, fmt.Errorf("PDF文件格式不兼容。此PDF可能使用了特殊编码、加密或压缩方式。建议：\n1. 使用其他PDF工具（如Adobe Acrobat、PDFtk等）重新保存该文件\n2. 确保PDF未加密且可以正常复制文本\n3. 尝试将PDF转换为标准格式后再上传")
		}
		return nil, fmt.Errorf("解析PDF失败: %w", err)
	}

	// 2. 提取需要翻译的文本
	if progressCallback != nil {
		progressCallback(0.2)
	}

	texts := pmt.Parser.GetTextForTranslation(content)
	if len(texts) == 0 {
		return nil, fmt.Errorf("PDF中没有可翻译的文本内容。可能原因：\n1. PDF是扫描版图片，需要先进行OCR识别\n2. PDF文本被加密或使用特殊编码\n3. PDF主要包含图片或图表内容")
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

	// 5. 生成输出文件 - 使用文本替换保留样式
	if progressCallback != nil {
		progressCallback(0.8)
	}

	filename := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))

	// 构建翻译映射
	translationMap := make(map[string]string)
	for _, block := range content.TextBlocks {
		originalText := strings.TrimSpace(block.Text)
		if originalText == "" {
			continue
		}

		// 查找对应的翻译文本
		for _, translatedBlock := range translatedContent.TextBlocks {
			if translatedBlock.PageNum == block.PageNum {
				translatedText := strings.TrimSpace(translatedBlock.Text)
				if translatedText != "" {
					translationMap[originalText] = translatedText
					break
				}
			}
		}
	}

	// 创建PDF文档对象用于样式保留替换
	pdfDoc := &PDFDocument{
		Path: inputPath,
		Metadata: PDFMetadata{
			Title:  content.Metadata["title"],
			Author: content.Metadata["author"],
			Pages:  len(content.TextBlocks),
		},
	}

	// 根据生成模式决定生成哪些文件
	var monoFile, dualFile string

	if config.GenerateMode == "monolingual" {
		// 单语模式：只生成单语PDF - 使用文本替换保留样式
		monoFile = filepath.Join(outputDir, filename+"-mono.pdf")
		if err := pdfDoc.SaveMonolingualPDFWithReplacement(monoFile, translationMap); err != nil {
			return nil, fmt.Errorf("生成单语PDF失败: %w", err)
		}
		log.Printf("单语模式：生成单语PDF: %s", monoFile)
	} else {
		// 双语模式（默认）：生成双语PDF，可选生成单语PDF - 使用文本替换保留样式
		dualFile = filepath.Join(outputDir, filename+"-dual.pdf")
		if err := pdfDoc.SaveBilingualPDFWithReplacement(dualFile, translationMap, BilingualLayoutTopBottom); err != nil {
			return nil, fmt.Errorf("生成双语PDF失败: %w", err)
		}

		// 也生成单语版本作为备选
		monoFile = filepath.Join(outputDir, filename+"-mono.pdf")
		if err := pdfDoc.SaveMonolingualPDFWithReplacement(monoFile, translationMap); err != nil {
			log.Printf("警告：生成单语PDF失败: %v", err)
			// 双语模式下，单语PDF失败不应该导致整个任务失败
		}
		log.Printf("双语模式：生成双语PDF: %s 和单语PDF: %s", dualFile, monoFile)
	}

	if progressCallback != nil {
		progressCallback(1.0)
	}

	// 验证生成的文件是否存在
	if config.GenerateMode == "monolingual" {
		if _, err := os.Stat(monoFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("单语PDF文件未生成: %s", monoFile)
		}
	} else {
		if _, err := os.Stat(dualFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("双语PDF文件未生成: %s", dualFile)
		}
	}

	result := &PDFMathResult{
		MonoFile: monoFile,
		DualFile: dualFile,
		Success:  true,
	}

	log.Printf("PDF翻译完成: mono=%s, dual=%s", result.MonoFile, result.DualFile)
	return result, nil
}

// setupFont 设置字体路径 - 保留用于兼容性，现在使用样式保留替换器自动处理字体
func (pmt *PDFMathTranslator) setupFont(langOut string) {
	// 使用系统字体检测器
	detector := NewSystemFontDetector()

	// 根据目标语言自动检测系统字体
	systemFontPath := detector.GetSystemFontPath(langOut)
	if systemFontPath != "" {
		pmt.FontPath = systemFontPath
		log.Printf("为语言 %s 选择系统字体: %s", langOut, systemFontPath)
	} else {
		log.Printf("警告：未找到语言 %s 的合适字体", langOut)
		log.Printf("提示：请确保系统已安装对应语言的字体")

		// 清空字体路径，使用默认处理
		pmt.FontPath = ""
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
