package translator

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Document 统一文档接口
type Document interface {
	GetTextBlocks() []string
	InsertTranslation(translations map[string]string) error
	InsertMonolingualTranslation(translations map[string]string) error // 新增：单语翻译插入
	Save(outputPath string) error
}

// DocumentType 文档类型
type DocumentType string

const (
	DocumentTypeEPUB DocumentType = "epub"
	DocumentTypePDF  DocumentType = "pdf"
)

// TranslationMode 翻译模式
type TranslationMode string

const (
	TranslationModeBasic    TranslationMode = "basic"    // 基础翻译（当前实现）
	TranslationModeAdvanced TranslationMode = "advanced" // 高级翻译（PDFMathTranslate）
)

// OpenDocument 根据文件类型打开文档
func OpenDocument(filePath string) (Document, DocumentType, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".epub":
		doc, err := OpenEPUB(filePath)
		if err != nil {
			return nil, "", fmt.Errorf("打开 EPUB 文件失败: %w", err)
		}
		return doc, DocumentTypeEPUB, nil

	case ".pdf":
		doc, err := OpenPDF(filePath)
		if err != nil {
			return nil, "", fmt.Errorf("打开 PDF 文件失败: %w", err)
		}
		return doc, DocumentTypePDF, nil

	default:
		return nil, "", fmt.Errorf("不支持的文件格式: %s", ext)
	}
}

// ValidateDocument 验证文档文件
func ValidateDocument(filePath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".epub":
		return ValidateEPUB(filePath)
	case ".pdf":
		return ValidatePDF(filePath)
	default:
		return fmt.Errorf("不支持的文件格式: %s，仅支持 .epub 和 .pdf 文件", ext)
	}
}

// GetRecommendedTranslationMode 获取推荐的翻译模式
func GetRecommendedTranslationMode(docType DocumentType) TranslationMode {
	switch docType {
	case DocumentTypePDF:
		// PDF优先使用PDFMathTranslate（如果可用）
		if IsPDFMathTranslateAvailable() {
			return TranslationModeAdvanced
		}
		return TranslationModeBasic
	case DocumentTypeEPUB:
		// EPUB使用基础翻译
		return TranslationModeBasic
	default:
		return TranslationModeBasic
	}
}

// GetDocumentInfo 获取文档信息
func GetDocumentInfo(filePath string) (map[string]interface{}, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	info := make(map[string]interface{})

	switch ext {
	case ".epub":
		epub, err := OpenEPUB(filePath)
		if err != nil {
			return nil, err
		}
		info["type"] = "EPUB"
		info["title"] = epub.Metadata.Title
		info["author"] = epub.Metadata.Author
		info["language"] = epub.Metadata.Language
		info["textBlocks"] = len(epub.GetTextBlocks())

	case ".pdf":
		pageCount, err := GetPDFPageCount(filePath)
		if err != nil {
			return nil, err
		}
		info["type"] = "PDF"
		info["pages"] = pageCount

		// 尝试获取更多信息
		pdf, err := OpenPDF(filePath)
		if err == nil {
			info["textBlocks"] = len(pdf.GetTextBlocks())
		}

		// 检查是否支持高级翻译
		info["advancedTranslationAvailable"] = IsPDFMathTranslateAvailable()
		info["recommendedMode"] = string(GetRecommendedTranslationMode(DocumentTypePDF))

	default:
		return nil, fmt.Errorf("不支持的文件格式: %s", ext)
	}

	return info, nil
}
