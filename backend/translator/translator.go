package translator

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

// DocumentTranslator 统一文档翻译器
type DocumentTranslator struct {
	Client            *TranslatorClient
	PDFMathTranslator *PDFMathTranslator
}

// NewDocumentTranslator 创建文档翻译器
func NewDocumentTranslator(config ProviderConfig, cache *Cache) (*DocumentTranslator, error) {
	client, err := NewTranslatorClient(config, cache)
	if err != nil {
		return nil, err
	}

	return &DocumentTranslator{
		Client:            client,
		PDFMathTranslator: NewPDFMathTranslator(),
	}, nil
}

// TranslateDocument 翻译文档，返回实际的输出路径
func (dt *DocumentTranslator) TranslateDocument(inputPath, outputPath, targetLanguage, userPrompt string, forceRetranslate bool, generateMode string, progressCallback func(float64)) (string, error) {
	log.Printf("开始翻译文档: %s", inputPath)

	// 验证文档
	if err := ValidateDocument(inputPath); err != nil {
		return "", fmt.Errorf("文档验证失败: %w", err)
	}

	// 获取文档类型
	_, docType, err := OpenDocument(inputPath)
	if err != nil {
		return "", fmt.Errorf("打开文档失败: %w", err)
	}

	log.Printf("文档类型: %s", docType)

	// 根据文档类型选择翻译方式
	switch docType {
	case DocumentTypePDF:
		return dt.translatePDF(inputPath, outputPath, targetLanguage, userPrompt, forceRetranslate, generateMode, progressCallback)
	case DocumentTypeEPUB:
		return dt.translateEPUB(inputPath, outputPath, targetLanguage, userPrompt, generateMode, progressCallback)
	default:
		return "", fmt.Errorf("不支持的文档类型: %s", docType)
	}
}

// savePDFTranslation 保存 PDF 翻译结果
func (dt *DocumentTranslator) savePDFTranslation(pdfDoc *PDFDocument, outputPath string, originalBlocks []string, translations map[string]string, generateMode string) (string, error) {
	// 构建翻译后的文本块
	var translatedBlocks []string
	for _, block := range originalBlocks {
		if trans, ok := translations[block]; ok {
			translatedBlocks = append(translatedBlocks, trans)
		} else {
			translatedBlocks = append(translatedBlocks, block)
		}
	}

	// 根据输出路径扩展名决定输出格式
	ext := strings.ToLower(filepath.Ext(outputPath))

	switch ext {
	case ".pdf":
		// 生成 PDF 文件
		if generateMode == "monolingual" {
			return outputPath, pdfDoc.SaveMonolingualPDF(outputPath, translatedBlocks)
		}
		return outputPath, pdfDoc.SaveBilingualPDF(outputPath, originalBlocks, translatedBlocks)
	case ".html":
		// 生成 HTML 文件
		if generateMode == "monolingual" {
			return outputPath, pdfDoc.SaveMonolingualHTML(outputPath, translatedBlocks)
		}
		return outputPath, pdfDoc.SaveBilingualHTML(outputPath, originalBlocks, translatedBlocks)
	case ".txt", ".md":
		// 生成文本文件
		if generateMode == "monolingual" {
			return outputPath, pdfDoc.SaveMonolingualText(outputPath, translatedBlocks)
		}
		return outputPath, pdfDoc.SaveBilingualText(outputPath, originalBlocks, translatedBlocks)
	default:
		// 默认保存为 PDF 文件
		pdfPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".pdf"
		if generateMode == "monolingual" {
			return pdfPath, pdfDoc.SaveMonolingualPDF(pdfPath, translatedBlocks)
		}
		return pdfPath, pdfDoc.SaveBilingualPDF(pdfPath, originalBlocks, translatedBlocks)
	}
}

// translatePDF 翻译PDF文档
func (dt *DocumentTranslator) translatePDF(inputPath, outputPath, targetLanguage, userPrompt string, forceRetranslate bool, generateMode string, progressCallback func(float64)) (string, error) {
	log.Printf("开始翻译PDF: %s", inputPath)

	// 准备输出目录
	outputDir := filepath.Dir(outputPath)

	// 设置翻译客户端
	dt.PDFMathTranslator.SetTranslatorClient(dt.Client)

	// 构建PDF翻译配置
	config := PDFMathConfig{
		LangIn:       "auto", // 自动检测源语言
		LangOut:      dt.mapLanguageCode(targetLanguage),
		Service:      dt.PDFMathTranslator.MapProviderToService(string(dt.Client.Provider.GetConfig().Type)),
		Thread:       4,
		Output:       outputDir,
		IgnoreCache:  forceRetranslate,
		Prompt:       userPrompt,
		GenerateMode: generateMode,
		Envs:         dt.PDFMathTranslator.BuildEnvs(dt.Client.Provider.GetConfig()),
	}

	// 执行翻译
	result, err := dt.PDFMathTranslator.TranslatePDF(inputPath, outputDir, config, progressCallback)
	if err != nil {
		log.Printf("PDF翻译失败，使用简化模式: %v", err)
		return dt.translatePDFSimple(inputPath, outputPath, targetLanguage, userPrompt, generateMode, progressCallback)
	}

	// 返回合适的PDF文件路径
	if generateMode == "monolingual" {
		if result.MonoFile != "" {
			return result.MonoFile, nil
		}
		return result.DualFile, nil
	} else {
		if result.DualFile != "" {
			return result.DualFile, nil
		}
		return result.MonoFile, nil
	}
}

// translatePDFSimple 简化PDF翻译（当PDFMathTranslator失败时使用）
func (dt *DocumentTranslator) translatePDFSimple(inputPath, outputPath, targetLanguage, userPrompt, generateMode string, progressCallback func(float64)) (string, error) {
	log.Printf("使用简化模式翻译PDF")

	// 打开PDF文档
	doc, _, err := OpenDocument(inputPath)
	if err != nil {
		return "", fmt.Errorf("打开PDF文档失败: %w", err)
	}

	// 提取文本块
	textBlocks := doc.GetTextBlocks()
	if len(textBlocks) == 0 {
		return "", fmt.Errorf("PDF中没有可翻译的文本内容")
	}

	log.Printf("提取到 %d 个文本块", len(textBlocks))

	// 翻译文本块
	translations := dt.translateTextBlocks(textBlocks, targetLanguage, userPrompt, progressCallback)

	// 保存PDF翻译结果
	pdfDoc := doc.(*PDFDocument)
	return dt.savePDFTranslation(pdfDoc, outputPath, textBlocks, translations, generateMode)
}

// translateEPUB 翻译EPUB文档
func (dt *DocumentTranslator) translateEPUB(inputPath, outputPath, targetLanguage, userPrompt, generateMode string, progressCallback func(float64)) (string, error) {
	log.Printf("开始翻译EPUB: %s", inputPath)

	// 打开EPUB文档
	doc, _, err := OpenDocument(inputPath)
	if err != nil {
		return "", fmt.Errorf("打开EPUB文档失败: %w", err)
	}

	// 提取文本块
	textBlocks := doc.GetTextBlocks()
	if len(textBlocks) == 0 {
		return "", fmt.Errorf("EPUB中没有可翻译的文本内容")
	}

	log.Printf("提取到 %d 个文本块", len(textBlocks))

	// 翻译文本块
	translations := dt.translateTextBlocks(textBlocks, targetLanguage, userPrompt, progressCallback)

	// 插入翻译到EPUB
	if generateMode == "monolingual" {
		if err := doc.InsertMonolingualTranslation(translations); err != nil {
			return "", fmt.Errorf("插入单语翻译失败: %w", err)
		}
	} else {
		if err := doc.InsertTranslation(translations); err != nil {
			return "", fmt.Errorf("插入双语翻译失败: %w", err)
		}
	}

	// 保存EPUB文档
	if err := doc.Save(outputPath); err != nil {
		return "", fmt.Errorf("保存EPUB文档失败: %w", err)
	}

	log.Printf("EPUB翻译完成: %s", outputPath)
	return outputPath, nil
}

// translateTextBlocks 翻译文本块的通用方法
func (dt *DocumentTranslator) translateTextBlocks(textBlocks []string, targetLanguage, userPrompt string, progressCallback func(float64)) map[string]string {
	translations := make(map[string]string)

	for i, block := range textBlocks {
		if strings.TrimSpace(block) == "" {
			continue
		}

		log.Printf("翻译第 %d/%d 个文本块", i+1, len(textBlocks))

		translated, err := dt.Client.Translate(block, targetLanguage, userPrompt)
		if err != nil {
			log.Printf("警告：翻译第 %d 个文本块失败: %v", i+1, err)
			translations[block] = block // 使用原文
		} else {
			translations[block] = translated
		}

		// 更新进度
		if progressCallback != nil {
			progress := float64(i+1) / float64(len(textBlocks))
			progressCallback(progress)
		}
	}

	return translations
}

// mapLanguageCode 映射语言代码到PDFMathTranslate支持的格式
func (dt *DocumentTranslator) mapLanguageCode(language string) string {
	mapping := map[string]string{
		"Chinese":    "zh",
		"English":    "en",
		"Japanese":   "ja",
		"Korean":     "ko",
		"French":     "fr",
		"German":     "de",
		"Spanish":    "es",
		"Russian":    "ru",
		"Arabic":     "ar",
		"Portuguese": "pt",
	}

	if code, ok := mapping[language]; ok {
		return code
	}
	return "zh" // 默认中文
}
