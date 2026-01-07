package translator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// PDFTranslatorIntegration PDF翻译集成
type PDFTranslatorIntegration struct {
	Client *TranslatorClient
}

// NewPDFTranslatorIntegration 创建PDF翻译集成
func NewPDFTranslatorIntegration(client *TranslatorClient) *PDFTranslatorIntegration {
	return &PDFTranslatorIntegration{
		Client: client,
	}
}

// TranslateTexts 翻译文本列表
func (pti *PDFTranslatorIntegration) TranslateTexts(texts []string, targetLanguage, userPrompt string, progressCallback func(float64)) (map[string]string, error) {
	translations := make(map[string]string)
	total := len(texts)

	if total == 0 {
		return translations, nil
	}

	log.Printf("开始翻译 %d 个文本块", total)

	for i, text := range texts {
		// 跳过空文本
		if strings.TrimSpace(text) == "" {
			translations[text] = text
			continue
		}

		// 跳过太短的文本
		if len(strings.TrimSpace(text)) < 3 {
			translations[text] = text
			continue
		}

		// 执行翻译
		translated, err := pti.Client.Translate(text, targetLanguage, userPrompt)
		if err != nil {
			log.Printf("警告：翻译第 %d 个文本块失败: %v", i+1, err)
			translations[text] = text // 使用原文
		} else {
			translations[text] = translated
		}

		// 更新进度
		if progressCallback != nil {
			progress := float64(i+1) / float64(total)
			progressCallback(progress)
		}

		log.Printf("翻译进度: %d/%d", i+1, total)
	}

	log.Printf("翻译完成，成功翻译 %d 个文本块", len(translations))
	return translations, nil
}

// TranslatePDFWithClient 使用翻译客户端翻译PDF
func (pti *PDFTranslatorIntegration) TranslatePDFWithClient(inputPath, outputDir, targetLanguage, userPrompt string, config PDFMathConfig, progressCallback func(float64)) (*PDFMathResult, error) {
	log.Printf("开始使用集成翻译客户端翻译PDF: %s", inputPath)

	// 创建PDF处理器
	parser := NewPDFParser("", "") // 可以根据需要配置公式检测规则

	// 1. 解析PDF (10%)
	if progressCallback != nil {
		progressCallback(0.1)
	}

	content, err := parser.ParsePDF(inputPath)
	if err != nil {
		return nil, fmt.Errorf("解析PDF失败: %w", err)
	}

	// 2. 提取需要翻译的文本 (20%)
	if progressCallback != nil {
		progressCallback(0.2)
	}

	texts := parser.GetTextForTranslation(content)
	if len(texts) == 0 {
		return nil, fmt.Errorf("PDF中没有可翻译的文本")
	}

	log.Printf("提取到 %d 个文本块用于翻译", len(texts))

	// 3. 执行翻译 (20% - 80%)
	translationProgressCallback := func(progress float64) {
		if progressCallback != nil {
			// 翻译占总进度的60%，从20%到80%
			progressCallback(0.2 + progress*0.6)
		}
	}

	translations, err := pti.TranslateTexts(texts, targetLanguage, userPrompt, translationProgressCallback)
	if err != nil {
		return nil, fmt.Errorf("翻译失败: %w", err)
	}

	// 4. 应用翻译结果 (85%)
	if progressCallback != nil {
		progressCallback(0.85)
	}

	translatedContent := *content // 复制原内容
	parser.ApplyTranslations(&translatedContent, translations)

	// 5. 生成输出文件 (90% - 100%)
	if progressCallback != nil {
		progressCallback(0.9)
	}

	result, err := pti.generateOutputFiles(content, &translatedContent, inputPath, outputDir, config)
	if err != nil {
		return nil, fmt.Errorf("生成输出文件失败: %w", err)
	}

	if progressCallback != nil {
		progressCallback(1.0)
	}

	log.Printf("PDF翻译完成: mono=%s, dual=%s", result.MonoFile, result.DualFile)
	return result, nil
}

// generateOutputFiles 生成输出文件 - 使用文本替换保留样式
func (pti *PDFTranslatorIntegration) generateOutputFiles(originalContent, translatedContent *PDFContent, inputPath, outputDir string, config PDFMathConfig) (*PDFMathResult, error) {
	filename := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))

	// 构建翻译映射
	translations := make(map[string]string)
	for _, block := range originalContent.TextBlocks {
		originalText := strings.TrimSpace(block.Text)
		if originalText == "" {
			continue
		}

		// 查找对应的翻译文本
		for _, translatedBlock := range translatedContent.TextBlocks {
			if translatedBlock.PageNum == block.PageNum {
				translatedText := strings.TrimSpace(translatedBlock.Text)
				if translatedText != "" {
					translations[originalText] = translatedText
					break
				}
			}
		}
	}

	// 创建PDF文档对象用于样式保留替换
	pdfDoc := &PDFDocument{
		Path: inputPath,
		Metadata: PDFMetadata{
			Title:  originalContent.Metadata["title"],
			Author: originalContent.Metadata["author"],
			Pages:  len(originalContent.TextBlocks),
		},
	}

	// 生成单语PDF（翻译版）- 使用文本替换保留样式
	monoFile := filepath.Join(outputDir, filename+"-mono.pdf")
	if err := pdfDoc.SaveMonolingualPDFWithReplacement(monoFile, translations); err != nil {
		return nil, fmt.Errorf("生成单语PDF失败: %w", err)
	}

	// 生成双语PDF - 使用文本替换保留样式
	dualFile := filepath.Join(outputDir, filename+"-dual.pdf")
	if err := pdfDoc.SaveBilingualPDFWithReplacement(dualFile, translations, BilingualLayoutTopBottom); err != nil {
		return nil, fmt.Errorf("生成双语PDF失败: %w", err)
	}

	// 验证生成的文件是否存在
	if _, err := os.Stat(monoFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("单语PDF文件未生成: %s", monoFile)
	}
	if _, err := os.Stat(dualFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("双语PDF文件未生成: %s", dualFile)
	}

	result := &PDFMathResult{
		MonoFile: monoFile,
		DualFile: dualFile,
		Success:  true,
	}

	return result, nil
}

// setupFontForLanguage 根据语言设置字体 - 保留用于兼容性，但不再使用PDF生成器
func (pti *PDFTranslatorIntegration) setupFontForLanguage(generator interface{}, langOut string) {
	// 这个方法保留用于兼容性，但现在使用样式保留替换器，不再需要手动设置字体
	log.Printf("使用样式保留替换器，自动处理字体: %s", langOut)
}

// fileExistsInternal 检查文件是否存在（内部使用）
func fileExistsInternal(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
