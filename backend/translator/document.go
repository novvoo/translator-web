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
	Save(outputPath string) error
}

// DocumentType 文档类型
type DocumentType string

const (
	DocumentTypeEPUB DocumentType = "epub"
	DocumentTypePDF  DocumentType = "pdf"
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

	default:
		return nil, fmt.Errorf("不支持的文件格式: %s", ext)
	}

	return info, nil
}
