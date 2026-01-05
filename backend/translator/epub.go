package translator

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// EPUBFile 表示一个 EPUB 文件
type EPUBFile struct {
	Path     string
	Files    map[string][]byte
	Metadata EPUBMetadata
}

type EPUBMetadata struct {
	Title    string
	Author   string
	Language string
}

// OpenEPUB 打开并解析 EPUB 文件
func OpenEPUB(path string) (*EPUBFile, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("打开 EPUB 文件失败: %w", err)
	}
	defer r.Close()

	epub := &EPUBFile{
		Path:  path,
		Files: make(map[string][]byte),
	}

	// 读取所有文件
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}

		epub.Files[f.Name] = content
	}

	// 解析元数据
	if err := epub.parseMetadata(); err != nil {
		return nil, err
	}

	return epub, nil
}

// parseMetadata 解析 EPUB 元数据
func (e *EPUBFile) parseMetadata() error {
	// 查找 content.opf 文件
	var opfPath string
	for name := range e.Files {
		if strings.HasSuffix(name, ".opf") {
			opfPath = name
			break
		}
	}

	if opfPath == "" {
		return fmt.Errorf("未找到 OPF 文件")
	}

	// 简单解析（实际应该用完整的 XML 解析）
	content := string(e.Files[opfPath])
	e.Metadata.Title = extractXMLTag(content, "dc:title")
	e.Metadata.Author = extractXMLTag(content, "dc:creator")
	e.Metadata.Language = extractXMLTag(content, "dc:language")

	return nil
}

// GetHTMLFiles 获取所有 HTML/XHTML 内容文件
func (e *EPUBFile) GetHTMLFiles() []string {
	var htmlFiles []string
	for name := range e.Files {
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".html" || ext == ".xhtml" || ext == ".htm" {
			htmlFiles = append(htmlFiles, name)
		}
	}
	return htmlFiles
}

// SaveEPUB 保存 EPUB 文件
func (e *EPUBFile) SaveEPUB(outputPath string) error {
	// 创建输出目录
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 创建 ZIP 文件
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// 写入所有文件
	for name, content := range e.Files {
		fw, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := fw.Write(content); err != nil {
			return err
		}
	}

	return nil
}

// extractXMLTag 简单提取 XML 标签内容
func extractXMLTag(content, tag string) string {
	start := strings.Index(content, "<"+tag)
	if start == -1 {
		return ""
	}
	start = strings.Index(content[start:], ">")
	if start == -1 {
		return ""
	}
	start += strings.Index(content[:start], "<"+tag) + 1

	end := strings.Index(content[start:], "</"+tag+">")
	if end == -1 {
		return ""
	}

	return strings.TrimSpace(content[start : start+end])
}

// HTMLContent 表示 HTML 内容
type HTMLContent struct {
	Body string
}

// ParseHTML 解析 HTML 内容
func ParseHTML(content []byte) (*HTMLContent, error) {
	// 简单提取 body 内容
	str := string(content)
	bodyStart := strings.Index(str, "<body")
	if bodyStart == -1 {
		return &HTMLContent{Body: str}, nil
	}

	bodyStart = strings.Index(str[bodyStart:], ">") + bodyStart + 1
	bodyEnd := strings.Index(str, "</body>")
	if bodyEnd == -1 {
		bodyEnd = len(str)
	}

	return &HTMLContent{
		Body: str[bodyStart:bodyEnd],
	}, nil
}

// ExtractTextBlocks 提取文本块
func ExtractTextBlocks(html string) []string {
	var blocks []string
	decoder := xml.NewDecoder(strings.NewReader(html))
	var currentText strings.Builder
	var inSpan bool

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// 如果 XML 解析失败，使用简单的文本提取
			return simpleTextExtract(html)
		}

		switch t := token.(type) {
		case xml.StartElement:
			// 检测 span 标签的开始
			if t.Name.Local == "span" {
				inSpan = true
			}
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				currentText.WriteString(text)
				currentText.WriteString(" ")
			}
		case xml.EndElement:
			// span 标签结束时，如果有内容则作为一个独立的文本块
			if t.Name.Local == "span" && inSpan {
				if currentText.Len() > 0 {
					blocks = append(blocks, strings.TrimSpace(currentText.String()))
					currentText.Reset()
				}
				inSpan = false
			} else if isBlockElement(t.Name.Local) {
				// 块级元素结束时，如果还有未处理的文本，也添加进去
				if currentText.Len() > 0 {
					blocks = append(blocks, strings.TrimSpace(currentText.String()))
					currentText.Reset()
				}
			}
		}
	}

	if currentText.Len() > 0 {
		blocks = append(blocks, strings.TrimSpace(currentText.String()))
	}

	return blocks
}

// isBlockElement 判断是否为块级元素
func isBlockElement(tag string) bool {
	blockTags := map[string]bool{
		"p": true, "div": true, "h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true, "li": true, "blockquote": true,
	}
	return blockTags[tag]
}

// simpleTextExtract 简单的文本提取（备用方案）
func simpleTextExtract(html string) []string {
	// 移除标签
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(r)
		}
	}

	// 按段落分割
	text := result.String()
	lines := strings.Split(text, "\n")
	var blocks []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			blocks = append(blocks, line)
		}
	}
	return blocks
}

// InsertTranslation 插入翻译（双语显示）
func InsertTranslation(html string, translations map[string]string) string {
	var buf bytes.Buffer
	decoder := xml.NewDecoder(strings.NewReader(html))
	var currentText strings.Builder
	var depth int
	var inSpan bool

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// 解析失败，返回原文
			return html
		}

		switch t := token.(type) {
		case xml.StartElement:
			buf.WriteString("<")
			buf.WriteString(t.Name.Local)
			for _, attr := range t.Attr {
				buf.WriteString(fmt.Sprintf(` %s="%s"`, attr.Name.Local, attr.Value))
			}
			buf.WriteString(">")
			depth++

			// 检测 span 标签
			if t.Name.Local == "span" {
				inSpan = true
			}

		case xml.EndElement:
			// 在 span 结束标签后插入翻译
			if t.Name.Local == "span" && inSpan && currentText.Len() > 0 {
				original := strings.TrimSpace(currentText.String())
				buf.WriteString("</")
				buf.WriteString(t.Name.Local)
				buf.WriteString(">")

				// 在 span 后面添加翻译（使用 span 标签保持格式一致）
				if trans, ok := translations[original]; ok && trans != "" {
					buf.WriteString(fmt.Sprintf(`<span class="translation" style="color: #666; font-style: italic;"> [%s]</span>`, trans))
				}
				currentText.Reset()
				inSpan = false
			} else {
				// 在块级元素结束标签前插入翻译
				if isBlockElement(t.Name.Local) && currentText.Len() > 0 {
					original := strings.TrimSpace(currentText.String())
					if trans, ok := translations[original]; ok && trans != "" {
						buf.WriteString(fmt.Sprintf(`<div class="translation" style="color: #666; font-style: italic; margin-top: 0.5em;">%s</div>`, trans))
					}
					currentText.Reset()
				}
				buf.WriteString("</")
				buf.WriteString(t.Name.Local)
				buf.WriteString(">")
			}
			depth--

		case xml.CharData:
			text := string(t)
			buf.WriteString(text)
			currentText.WriteString(strings.TrimSpace(text))
			currentText.WriteString(" ")
		}
	}

	return buf.String()
}
