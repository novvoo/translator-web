package translator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFDocument 表示一个 PDF 文档
type PDFDocument struct {
	Path      string
	PageTexts []string
	Metadata  PDFMetadata
}

type PDFMetadata struct {
	Title  string
	Author string
	Pages  int
}

// OpenPDF 打开并解析 PDF 文件
func OpenPDF(path string) (*PDFDocument, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("无法打开 PDF 文件: %w", err)
	}
	defer file.Close()

	pageCount := reader.NumPage()
	log.Printf("PDF 总页数: %d", pageCount)

	doc := &PDFDocument{
		Path:      path,
		PageTexts: make([]string, 0, pageCount),
		Metadata: PDFMetadata{
			Pages: pageCount,
		},
	}

	// 提取每页文本
	for i := 1; i <= pageCount; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			log.Printf("警告：第 %d 页为空", i)
			doc.PageTexts = append(doc.PageTexts, "")
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			log.Printf("警告：无法提取第 %d 页的文本: %v", i, err)
			doc.PageTexts = append(doc.PageTexts, "")
			continue
		}

		cleanText := cleanPDFText(text)
		doc.PageTexts = append(doc.PageTexts, cleanText)
	}

	return doc, nil
}

// GetTextBlocks 获取文本块（实现 Document 接口）
func (d *PDFDocument) GetTextBlocks() []string {
	var blocks []string

	for i, pageText := range d.PageTexts {
		if strings.TrimSpace(pageText) == "" {
			continue
		}

		// 按段落分割页面文本
		paragraphs := strings.Split(pageText, "\n\n")
		for _, para := range paragraphs {
			para = strings.TrimSpace(para)
			if para != "" && len(para) > 10 { // 过滤太短的段落
				// 添加页面信息
				blockText := fmt.Sprintf("[第%d页] %s", i+1, para)
				blocks = append(blocks, blockText)
			}
		}
	}

	return blocks
}

// InsertTranslation 插入翻译（实现 Document 接口）
func (d *PDFDocument) InsertTranslation(translations map[string]string) error {
	// PDF 不支持直接编辑，我们将生成文本文件
	// 这个方法主要用于保存翻译映射
	return nil
}

// Save 保存文档（实现 Document 接口）
func (d *PDFDocument) Save(outputPath string) error {
	// 由于 PDF 编辑复杂，我们生成双语文本文件
	return d.SaveAsText(outputPath)
}

// SaveAsText 保存为双语文本文件
func (d *PDFDocument) SaveAsText(outputPath string) error {
	var content strings.Builder

	content.WriteString("# PDF 翻译结果\n")
	content.WriteString("# PDF Translation Result\n\n")
	content.WriteString(fmt.Sprintf("原文件: %s\n", filepath.Base(d.Path)))
	content.WriteString(fmt.Sprintf("总页数: %d\n\n", d.Metadata.Pages))
	content.WriteString("---\n\n")

	for i, pageText := range d.PageTexts {
		if strings.TrimSpace(pageText) == "" {
			continue
		}

		content.WriteString(fmt.Sprintf("## 第 %d 页 / Page %d\n\n", i+1, i+1))
		content.WriteString("**原文 / Original:**\n")
		content.WriteString(pageText)
		content.WriteString("\n\n")
		content.WriteString("**译文 / Translation:**\n")
		content.WriteString("(翻译将在处理完成后显示)\n\n")
		content.WriteString("---\n\n")
	}

	return writeTextFile(outputPath, content.String())
}

// SaveBilingualText 保存双语对照文本
func (d *PDFDocument) SaveBilingualText(outputPath string, originalBlocks, translatedBlocks []string) error {
	var content strings.Builder

	content.WriteString("# PDF 翻译结果\n")
	content.WriteString("# PDF Translation Result\n\n")
	content.WriteString(fmt.Sprintf("原文件: %s\n", filepath.Base(d.Path)))
	content.WriteString(fmt.Sprintf("总页数: %d\n\n", d.Metadata.Pages))
	content.WriteString("---\n\n")

	for i := 0; i < len(originalBlocks) && i < len(translatedBlocks); i++ {
		if strings.TrimSpace(originalBlocks[i]) == "" {
			continue
		}

		content.WriteString(fmt.Sprintf("## 段落 %d / Paragraph %d\n\n", i+1, i+1))
		content.WriteString("**原文 / Original:**\n")
		content.WriteString(originalBlocks[i])
		content.WriteString("\n\n")
		content.WriteString("**译文 / Translation:**\n")
		content.WriteString(translatedBlocks[i])
		content.WriteString("\n\n")
		content.WriteString("---\n\n")
	}

	return writeTextFile(outputPath, content.String())
}

// cleanPDFText 清理 PDF 文本
func cleanPDFText(text string) string {
	// 按行处理，保留换行符
	lines := strings.Split(text, "\n")
	var cleanLines []string

	for _, line := range lines {
		// 移除行内多余的空白字符
		line = regexp.MustCompile(`[ \t]+`).ReplaceAllString(line, " ")
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		// 跳过可能的页码
		if regexp.MustCompile(`^\d+$`).MatchString(line) {
			continue
		}

		// 跳过太短的行（可能是页眉页脚）
		if len(line) < 3 {
			continue
		}

		cleanLines = append(cleanLines, line)
	}

	return strings.Join(cleanLines, "\n")
}

// ValidatePDF 验证是否为有效的 PDF 文件
func ValidatePDF(filePath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".pdf" {
		return fmt.Errorf("文件必须是 PDF 格式")
	}

	// 尝试打开文件验证格式
	file, _, err := pdf.Open(filePath)
	if err != nil {
		return fmt.Errorf("无效的 PDF 文件: %w", err)
	}
	file.Close()

	return nil
}

// GetPDFPageCount 获取 PDF 页数
func GetPDFPageCount(filePath string) (int, error) {
	file, reader, err := pdf.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	return reader.NumPage(), nil
}

// SaveBilingualHTML 保存双语对照 HTML 文件
func (d *PDFDocument) SaveBilingualHTML(outputPath string, originalBlocks, translatedBlocks []string) error {
	var content strings.Builder

	content.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>PDF 翻译结果 / PDF Translation Result</title>
    <style>
        body { 
            font-family: Arial, sans-serif; 
            margin: 20px; 
            line-height: 1.6; 
            background-color: #f5f5f5;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
            background-color: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
            border-bottom: 2px solid #3498db;
            padding-bottom: 20px;
        }
        .header h1 {
            color: #2c3e50;
            margin: 0;
        }
        .meta-info {
            background-color: #ecf0f1;
            padding: 15px;
            border-radius: 5px;
            margin-bottom: 30px;
        }
        .section { 
            margin-bottom: 25px; 
            border: 1px solid #e0e0e0;
            border-radius: 5px;
            overflow: hidden;
        }
        .original { 
            background-color: #f8f9fa; 
            padding: 15px;
            border-bottom: 1px solid #e0e0e0;
        }
        .translation { 
            background-color: #e8f4f8; 
            padding: 15px;
        }
        .label { 
            font-weight: bold; 
            color: #2c3e50; 
            margin-bottom: 8px;
            font-size: 14px;
        }
        .content {
            color: #34495e;
            white-space: pre-wrap;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📄 PDF 翻译结果</h1>
            <h2>PDF Translation Result</h2>
        </div>
        
        <div class="meta-info">
            <strong>原文件:</strong> ` + filepath.Base(d.Path) + `<br>
            <strong>总页数:</strong> ` + fmt.Sprintf("%d", d.Metadata.Pages) + `<br>
            <strong>翻译时间:</strong> <span id="datetime"></span>
        </div>
`)

	for i := 0; i < len(originalBlocks) && i < len(translatedBlocks); i++ {
		if strings.TrimSpace(originalBlocks[i]) == "" {
			continue
		}

		content.WriteString(fmt.Sprintf(`
        <div class="section">
            <div class="original">
                <div class="label">📖 原文 / Original %d:</div>
                <div class="content">%s</div>
            </div>
            <div class="translation">
                <div class="label">🌐 译文 / Translation %d:</div>
                <div class="content">%s</div>
            </div>
        </div>
`, i+1, strings.ReplaceAll(originalBlocks[i], "\n", "<br>"),
			i+1, strings.ReplaceAll(translatedBlocks[i], "\n", "<br>")))
	}

	content.WriteString(`
    </div>
    <script>
        document.getElementById('datetime').textContent = new Date().toLocaleString();
    </script>
</body>
</html>`)

	return writeTextFile(outputPath, content.String())
}

// writeTextFile 写入文本文件
func writeTextFile(filePath, content string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}
