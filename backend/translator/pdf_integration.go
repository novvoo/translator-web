package translator

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

// PDFReplacementIntegration PDF替换集成
type PDFReplacementIntegration struct {
	client                *TranslatorClient
	regenerator           *PDFRegenerator
	styleReplacer         *PDFStylePreservingReplacer
	translatorIntegration *PDFTranslatorIntegration
}

// PDFReplacementMode PDF替换模式
type PDFReplacementMode string

const (
	ReplacementModeMonolingual PDFReplacementMode = "monolingual"
	ReplacementModeBilingual   PDFReplacementMode = "bilingual"
)

// PDFBilingualLayout 双语布局
type PDFBilingualLayout string

const (
	BilingualLayoutSideBySide  PDFBilingualLayout = "side-by-side"
	BilingualLayoutTopBottom   PDFBilingualLayout = "top-bottom"
	BilingualLayoutInterleaved PDFBilingualLayout = "interleaved"
)

// PDFReplacementRequest PDF替换请求
type PDFReplacementRequest struct {
	InputPath       string             `json:"input_path"`
	OutputDir       string             `json:"output_dir"`
	TargetLanguage  string             `json:"target_language"`
	UserPrompt      string             `json:"user_prompt"`
	Mode            PDFReplacementMode `json:"mode"`
	BilingualLayout PDFBilingualLayout `json:"bilingual_layout"`
	PreserveStyle   bool               `json:"preserve_style"`
	FontScale       float64            `json:"font_scale"`
	LineSpacing     float64            `json:"line_spacing"`
}

// PDFReplacementResult PDF替换结果
type PDFReplacementResult struct {
	MonoFile string `json:"mono_file"`
	DualFile string `json:"dual_file"`
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Method   string `json:"method"` // "content_replacement" 或 "style_preserving"
}

// NewPDFReplacementIntegration 创建PDF替换集成
func NewPDFReplacementIntegration(client *TranslatorClient) *PDFReplacementIntegration {
	return &PDFReplacementIntegration{
		client:                client,
		regenerator:           NewPDFRegenerator(),
		styleReplacer:         NewPDFStylePreservingReplacer(),
		translatorIntegration: NewPDFTranslatorIntegration(client),
	}
}

// TranslatePDFWithReplacement 使用内容替换方式翻译PDF
func (pri *PDFReplacementIntegration) TranslatePDFWithReplacement(request PDFReplacementRequest, progressCallback func(float64)) (*PDFReplacementResult, error) {
	log.Printf("开始PDF内容替换翻译: %s", request.InputPath)

	// 验证输入参数
	if err := pri.validateRequest(request); err != nil {
		return nil, fmt.Errorf("请求参数验证失败: %w", err)
	}

	// 1. 解析PDF并提取文本 (20%)
	if progressCallback != nil {
		progressCallback(0.1)
	}

	parser := NewPDFParser("", "")
	content, err := parser.ParsePDF(request.InputPath)
	if err != nil {
		return nil, fmt.Errorf("解析PDF失败: %w", err)
	}

	if progressCallback != nil {
		progressCallback(0.2)
	}

	// 2. 提取需要翻译的文本
	texts := parser.GetTextForTranslation(content)
	if len(texts) == 0 {
		return nil, fmt.Errorf("PDF中没有可翻译的文本")
	}

	log.Printf("提取到 %d 个文本块用于翻译", len(texts))

	// 3. 执行翻译 (20% - 70%)
	translationProgressCallback := func(progress float64) {
		if progressCallback != nil {
			progressCallback(0.2 + progress*0.5)
		}
	}

	translations, err := pri.translatorIntegration.TranslateTexts(texts, request.TargetLanguage, request.UserPrompt, translationProgressCallback)
	if err != nil {
		return nil, fmt.Errorf("翻译失败: %w", err)
	}

	if progressCallback != nil {
		progressCallback(0.7)
	}

	// 4. 执行内容替换 (70% - 100%)
	result, err := pri.performReplacement(request, content, translations, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("内容替换失败: %w", err)
	}

	if progressCallback != nil {
		progressCallback(1.0)
	}

	log.Printf("PDF内容替换翻译完成: %s", result.Message)
	return result, nil
}

// validateRequest 验证请求参数
func (pri *PDFReplacementIntegration) validateRequest(request PDFReplacementRequest) error {
	if request.InputPath == "" {
		return fmt.Errorf("输入文件路径不能为空")
	}

	if request.OutputDir == "" {
		return fmt.Errorf("输出目录不能为空")
	}

	if request.TargetLanguage == "" {
		return fmt.Errorf("目标语言不能为空")
	}

	if request.Mode != ReplacementModeMonolingual && request.Mode != ReplacementModeBilingual {
		return fmt.Errorf("不支持的替换模式: %s", request.Mode)
	}

	if request.Mode == ReplacementModeBilingual {
		if request.BilingualLayout != BilingualLayoutSideBySide &&
			request.BilingualLayout != BilingualLayoutTopBottom &&
			request.BilingualLayout != BilingualLayoutInterleaved {
			return fmt.Errorf("不支持的双语布局: %s", request.BilingualLayout)
		}
	}

	// 设置默认值
	if request.FontScale == 0 {
		request.FontScale = 1.0
	}
	if request.LineSpacing == 0 {
		request.LineSpacing = 1.2
	}

	return nil
}

// performReplacement 执行替换操作
func (pri *PDFReplacementIntegration) performReplacement(request PDFReplacementRequest, content *PDFContent, translations map[string]string, progressCallback func(float64)) (*PDFReplacementResult, error) {
	filename := strings.TrimSuffix(filepath.Base(request.InputPath), filepath.Ext(request.InputPath))

	var result *PDFReplacementResult
	var err error

	if request.PreserveStyle {
		// 使用样式保留替换器
		result, err = pri.performStylePreservingReplacement(request, filename, translations, progressCallback)
	} else {
		// 使用基础内容替换器
		result, err = pri.performBasicContentReplacement(request, filename, translations, progressCallback)
	}

	return result, err
}

// performStylePreservingReplacement 执行样式保留替换
func (pri *PDFReplacementIntegration) performStylePreservingReplacement(request PDFReplacementRequest, filename string, translations map[string]string, progressCallback func(float64)) (*PDFReplacementResult, error) {
	log.Printf("执行样式保留替换")

	// 创建样式保留配置
	config := StylePreservingConfig{
		Mode:               string(request.Mode),
		BilingualLayout:    string(request.BilingualLayout),
		PreserveFormatting: true,
		FontScale:          request.FontScale,
		LineSpacing:        request.LineSpacing,
		MarginAdjustment:   0,
		ColorPreservation:  true,
	}

	result := &PDFReplacementResult{
		Success: true,
		Method:  "style_preserving",
	}

	// 生成输出文件路径
	if request.Mode == ReplacementModeMonolingual {
		result.MonoFile = filepath.Join(request.OutputDir, filename+"-mono-replaced.pdf")

		if progressCallback != nil {
			progressCallback(0.8)
		}

		err := pri.styleReplacer.ReplaceWithStylePreservation(request.InputPath, result.MonoFile, translations, config)
		if err != nil {
			return nil, fmt.Errorf("单语样式保留替换失败: %w", err)
		}

		result.Message = fmt.Sprintf("单语PDF替换完成，保留原始样式: %s", result.MonoFile)
	} else {
		result.DualFile = filepath.Join(request.OutputDir, filename+"-dual-replaced.pdf")

		if progressCallback != nil {
			progressCallback(0.8)
		}

		err := pri.styleReplacer.ReplaceWithStylePreservation(request.InputPath, result.DualFile, translations, config)
		if err != nil {
			return nil, fmt.Errorf("双语样式保留替换失败: %w", err)
		}

		result.Message = fmt.Sprintf("双语PDF替换完成，保留原始样式，布局: %s，文件: %s", request.BilingualLayout, result.DualFile)
	}

	if progressCallback != nil {
		progressCallback(0.95)
	}

	return result, nil
}

// performBasicContentReplacement 执行基础内容替换
func (pri *PDFReplacementIntegration) performBasicContentReplacement(request PDFReplacementRequest, filename string, translations map[string]string, progressCallback func(float64)) (*PDFReplacementResult, error) {
	log.Printf("执行基础内容替换")

	result := &PDFReplacementResult{
		Success: true,
		Method:  "content_replacement",
	}

	// 生成输出文件路径
	if request.Mode == ReplacementModeMonolingual {
		result.MonoFile = filepath.Join(request.OutputDir, filename+"-mono-replaced.pdf")

		if progressCallback != nil {
			progressCallback(0.8)
		}

		err := pri.regenerator.RegeneratePDF(request.InputPath, result.MonoFile, translations)
		if err != nil {
			return nil, fmt.Errorf("单语PDF重新生成失败: %w", err)
		}

		result.Message = fmt.Sprintf("单语PDF重新生成完成: %s", result.MonoFile)
	} else {
		result.DualFile = filepath.Join(request.OutputDir, filename+"-dual-replaced.pdf")

		if progressCallback != nil {
			progressCallback(0.8)
		}

		// 对于双语模式，需要构建双语文本映射
		bilingualMappings := make(map[string]string)
		for original, translation := range translations {
			switch request.BilingualLayout {
			case BilingualLayoutSideBySide:
				bilingualMappings[original] = original + " | " + translation
			case BilingualLayoutInterleaved:
				bilingualMappings[original] = original + "\n" + translation
			default: // BilingualLayoutTopBottom
				bilingualMappings[original] = original + "\n" + translation
			}
		}

		err := pri.regenerator.RegeneratePDF(request.InputPath, result.DualFile, bilingualMappings)
		if err != nil {
			return nil, fmt.Errorf("双语PDF重新生成失败: %w", err)
		}

		result.Message = fmt.Sprintf("双语PDF重新生成完成，布局: %s，文件: %s", request.BilingualLayout, result.DualFile)
	}

	if progressCallback != nil {
		progressCallback(0.95)
	}

	return result, nil
}

// GetSupportedBilingualLayouts 获取支持的双语布局
func (pri *PDFReplacementIntegration) GetSupportedBilingualLayouts() []PDFBilingualLayout {
	return []PDFBilingualLayout{
		BilingualLayoutSideBySide,
		BilingualLayoutTopBottom,
		BilingualLayoutInterleaved,
	}
}

// GetReplacementModes 获取支持的替换模式
func (pri *PDFReplacementIntegration) GetReplacementModes() []PDFReplacementMode {
	return []PDFReplacementMode{
		ReplacementModeMonolingual,
		ReplacementModeBilingual,
	}
}

// CreateDefaultRequest 创建默认请求
func CreateDefaultPDFReplacementRequest(inputPath, outputDir, targetLanguage string) PDFReplacementRequest {
	return PDFReplacementRequest{
		InputPath:       inputPath,
		OutputDir:       outputDir,
		TargetLanguage:  targetLanguage,
		UserPrompt:      "",
		Mode:            ReplacementModeMonolingual,
		BilingualLayout: BilingualLayoutTopBottom,
		PreserveStyle:   true,
		FontScale:       1.0,
		LineSpacing:     1.2,
	}
}

// CreateBilingualRequest 创建双语请求
func CreateBilingualPDFReplacementRequest(inputPath, outputDir, targetLanguage string, layout PDFBilingualLayout) PDFReplacementRequest {
	request := CreateDefaultPDFReplacementRequest(inputPath, outputDir, targetLanguage)
	request.Mode = ReplacementModeBilingual
	request.BilingualLayout = layout
	request.FontScale = 0.9 // 双语模式下稍微缩小字体
	return request
}
