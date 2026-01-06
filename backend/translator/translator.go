package translator

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

// DocumentTranslator 统一文档翻译器
type DocumentTranslator struct {
	Client *TranslatorClient
}

// NewDocumentTranslator 创建文档翻译器
func NewDocumentTranslator(config ProviderConfig, cache *Cache) (*DocumentTranslator, error) {
	client, err := NewTranslatorClient(config, cache)
	if err != nil {
		return nil, err
	}

	return &DocumentTranslator{
		Client: client,
	}, nil
}

// TranslateDocument 翻译文档
func (dt *DocumentTranslator) TranslateDocument(inputPath, outputPath, targetLanguage, userPrompt string, progressCallback func(float64)) error {
	log.Printf("开始翻译文档: %s", inputPath)

	// 验证文档
	if err := ValidateDocument(inputPath); err != nil {
		return fmt.Errorf("文档验证失败: %w", err)
	}

	// 打开文档
	doc, docType, err := OpenDocument(inputPath)
	if err != nil {
		return fmt.Errorf("打开文档失败: %w", err)
	}

	log.Printf("文档类型: %s", docType)

	// 提取文本块
	textBlocks := doc.GetTextBlocks()
	if len(textBlocks) == 0 {
		return fmt.Errorf("文档中没有可翻译的文本内容")
	}

	log.Printf("提取到 %d 个文本块", len(textBlocks))

	// 翻译文本块
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
		progress := float64(i+1) / float64(len(textBlocks))
		if progressCallback != nil {
			progressCallback(progress)
		}
	}

	// 根据文档类型处理输出
	switch docType {
	case DocumentTypeEPUB:
		// EPUB 支持直接插入翻译
		if err := doc.InsertTranslation(translations); err != nil {
			return fmt.Errorf("插入翻译失败: %w", err)
		}
		if err := doc.Save(outputPath); err != nil {
			return fmt.Errorf("保存文档失败: %w", err)
		}

	case DocumentTypePDF:
		// PDF 生成双语文本文件
		pdfDoc := doc.(*PDFDocument)
		if err := dt.savePDFTranslation(pdfDoc, outputPath, textBlocks, translations); err != nil {
			return fmt.Errorf("保存 PDF 翻译失败: %w", err)
		}

	default:
		return fmt.Errorf("不支持的文档类型: %s", docType)
	}

	log.Printf("文档翻译完成: %s", outputPath)
	return nil
}

// savePDFTranslation 保存 PDF 翻译结果
func (dt *DocumentTranslator) savePDFTranslation(pdfDoc *PDFDocument, outputPath string, originalBlocks []string, translations map[string]string) error {
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
	case ".html":
		// 生成 HTML 文件
		return pdfDoc.SaveBilingualHTML(outputPath, originalBlocks, translatedBlocks)
	case ".txt", ".md":
		// 生成文本文件
		return pdfDoc.SaveBilingualText(outputPath, originalBlocks, translatedBlocks)
	default:
		// 默认保存为 HTML 文件（更好的格式）
		htmlPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".html"
		return pdfDoc.SaveBilingualHTML(htmlPath, originalBlocks, translatedBlocks)
	}
}
