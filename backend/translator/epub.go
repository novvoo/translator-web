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
	// 预处理HTML，清理字符实体和格式化
	html = strings.ReplaceAll(html, "&#13;", "")
	html = strings.ReplaceAll(html, "\r", "")
	html = strings.ReplaceAll(html, "\n", " ")

	// 将<br/>标签替换为换行符，便于后续分割
	html = strings.ReplaceAll(html, "<br/>", "\n")
	html = strings.ReplaceAll(html, "<br>", "\n")
	html = strings.ReplaceAll(html, "<BR/>", "\n")
	html = strings.ReplaceAll(html, "<BR>", "\n")

	var blocks []string
	decoder := xml.NewDecoder(strings.NewReader(html))
	var currentText strings.Builder
	var inSpan bool
	var inFont bool
	var inBlockElement bool
	var inBold bool

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// 如果 XML 解析失败，直接返回空切片，不再使用simpleTextExtract
			return []string{}
		}

		switch t := token.(type) {
		case xml.StartElement:
			// 检测 span 标签的开始
			if t.Name.Local == "span" {
				inSpan = true
			}
			// 检测 font 标签的开始（通常用于例句）
			if t.Name.Local == "font" {
				inFont = true
				// 如果当前有文本，先保存为一个块
				if currentText.Len() > 0 {
					text := strings.TrimSpace(currentText.String())
					if text != "" && shouldExtractText(text) {
						blocks = append(blocks, text)
					}
					currentText.Reset()
				}
			}
			// 检测 b 标签（粗体，通常是单词）
			if t.Name.Local == "b" {
				inBold = true
				// 如果当前有文本，先保存为一个块
				if currentText.Len() > 0 {
					text := strings.TrimSpace(currentText.String())
					if text != "" && shouldExtractText(text) {
						blocks = append(blocks, text)
					}
					currentText.Reset()
				}
			}
			// 检测块级元素
			if isBlockElement(t.Name.Local) {
				inBlockElement = true
			}

		case xml.CharData:
			text := string(t)
			// 清理多余的空白字符
			text = strings.TrimSpace(text)
			if text != "" {
				if currentText.Len() > 0 {
					currentText.WriteString(" ")
				}
				currentText.WriteString(text)
			}

		case xml.EndElement:
			// span 标签结束时，如果有内容则作为一个独立的文本块
			if t.Name.Local == "span" && inSpan {
				if currentText.Len() > 0 {
					text := strings.TrimSpace(currentText.String())
					if text != "" && shouldExtractText(text) {
						blocks = append(blocks, text)
					}
					currentText.Reset()
				}
				inSpan = false
			} else if t.Name.Local == "font" && inFont {
				// font 标签结束时，保存为独立的文本块（通常是例句）
				if currentText.Len() > 0 {
					text := strings.TrimSpace(currentText.String())
					if text != "" && shouldExtractText(text) {
						blocks = append(blocks, text)
					}
					currentText.Reset()
				}
				inFont = false
			} else if t.Name.Local == "b" && inBold {
				// b 标签结束时，保存为独立的文本块（通常是单词）
				if currentText.Len() > 0 {
					text := strings.TrimSpace(currentText.String())
					if text != "" && shouldExtractText(text) {
						blocks = append(blocks, text)
					}
					currentText.Reset()
				}
				inBold = false
			} else if isBlockElement(t.Name.Local) && inBlockElement {
				// 块级元素结束时，如果还有未处理的文本，也添加进去
				if currentText.Len() > 0 {
					text := strings.TrimSpace(currentText.String())
					if text != "" && shouldExtractText(text) {
						blocks = append(blocks, text)
					}
					currentText.Reset()
				}
				inBlockElement = false
			}
		}
	}

	// 处理剩余的文本
	if currentText.Len() > 0 {
		text := strings.TrimSpace(currentText.String())
		if text != "" && shouldExtractText(text) {
			blocks = append(blocks, text)
		}
	}

	return blocks
}

// shouldExtractText 判断文本是否应该被提取（过滤掉纯标点符号等）
func shouldExtractText(text string) bool {
	// 过滤掉空文本
	if strings.TrimSpace(text) == "" {
		return false
	}

	// 过滤掉只包含标点符号和空格的文本
	hasLetter := false
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r > 127 {
			hasLetter = true
			break
		}
	}

	return hasLetter
}

// isBlockElement 判断是否为块级元素
func isBlockElement(tag string) bool {
	blockTags := map[string]bool{
		"p": true, "div": true, "h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true, "li": true, "blockquote": true,
	}
	return blockTags[tag]
}

// InsertTranslation 插入翻译（双语显示）
func InsertTranslation(html string, translations map[string]string) string {
	var buf bytes.Buffer
	decoder := xml.NewDecoder(strings.NewReader(html))
	var currentText strings.Builder
	var depth int
	var inSpan bool
	var inFont bool

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
			// 检测 font 标签
			if t.Name.Local == "font" {
				inFont = true
				// 如果当前有文本，先插入翻译
				if currentText.Len() > 0 {
					original := strings.TrimSpace(currentText.String())
					if trans, ok := translations[original]; ok && trans != "" {
						buf.WriteString(fmt.Sprintf(`<div class="translation" style="color: #666; font-style: italic; margin-top: 0.5em;">%s</div>`, trans))
					}
					currentText.Reset()
				}
			}
			// br 标签处理
			if t.Name.Local == "br" {
				if currentText.Len() > 0 {
					original := strings.TrimSpace(currentText.String())
					if trans, ok := translations[original]; ok && trans != "" {
						buf.WriteString(fmt.Sprintf(`<div class="translation" style="color: #666; font-style: italic; margin-top: 0.5em;">%s</div>`, trans))
					}
					currentText.Reset()
				}
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
			} else if t.Name.Local == "font" && inFont && currentText.Len() > 0 {
				// 在 font 结束标签后插入翻译
				original := strings.TrimSpace(currentText.String())
				buf.WriteString("</")
				buf.WriteString(t.Name.Local)
				buf.WriteString(">")

				// 在 font 后面添加翻译
				if trans, ok := translations[original]; ok && trans != "" {
					buf.WriteString(fmt.Sprintf(`<div class="translation" style="color: #666; font-style: italic; margin-top: 0.5em;">%s</div>`, trans))
				}
				currentText.Reset()
				inFont = false
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

// GetTextBlocks 获取文本块（实现 Document 接口）
func (e *EPUBFile) GetTextBlocks() []string {
	var allBlocks []string

	htmlFiles := e.GetHTMLFiles()
	for _, filename := range htmlFiles {
		content := e.Files[filename]
		htmlContent, err := ParseHTML(content)
		if err != nil {
			continue
		}

		blocks := ExtractTextBlocks(htmlContent.Body)
		allBlocks = append(allBlocks, blocks...)
	}

	return allBlocks
}

// InsertTranslation 插入翻译（实现 Document 接口）
func (e *EPUBFile) InsertTranslation(translations map[string]string) error {
	htmlFiles := e.GetHTMLFiles()

	for _, filename := range htmlFiles {
		content := e.Files[filename]
		htmlContent, err := ParseHTML(content)
		if err != nil {
			continue
		}

		// 插入翻译
		translatedBody := InsertTranslation(htmlContent.Body, translations)

		// 重新构建完整的 HTML
		originalStr := string(content)
		bodyStart := strings.Index(originalStr, "<body")
		if bodyStart == -1 {
			continue
		}

		bodyStartEnd := strings.Index(originalStr[bodyStart:], ">") + bodyStart + 1
		bodyEnd := strings.Index(originalStr, "</body>")
		if bodyEnd == -1 {
			bodyEnd = len(originalStr)
		}

		newContent := originalStr[:bodyStartEnd] + translatedBody + originalStr[bodyEnd:]
		e.Files[filename] = []byte(newContent)
	}

	return nil
}

// InsertMonolingualTranslation 插入单语翻译（实现 Document 接口）
func (e *EPUBFile) InsertMonolingualTranslation(translations map[string]string) error {
	htmlFiles := e.GetHTMLFiles()

	for _, filename := range htmlFiles {
		content := e.Files[filename]
		htmlContent, err := ParseHTML(content)
		if err != nil {
			continue
		}

		// 插入单语翻译（替换原文）
		translatedBody := InsertMonolingualTranslation(htmlContent.Body, translations)

		// 重新构建完整的 HTML
		originalStr := string(content)
		bodyStart := strings.Index(originalStr, "<body")
		if bodyStart == -1 {
			continue
		}

		bodyStartEnd := strings.Index(originalStr[bodyStart:], ">") + bodyStart + 1
		bodyEnd := strings.Index(originalStr, "</body>")
		if bodyEnd == -1 {
			bodyEnd = len(originalStr)
		}

		newContent := originalStr[:bodyStartEnd] + translatedBody + originalStr[bodyEnd:]
		e.Files[filename] = []byte(newContent)
	}

	return nil
}

// Save 保存文档（实现 Document 接口）
func (e *EPUBFile) Save(outputPath string) error {
	return e.SaveEPUB(outputPath)
}

// ValidateEPUB 验证是否为有效的 EPUB 文件
func ValidateEPUB(filePath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".epub" {
		return fmt.Errorf("文件必须是 EPUB 格式")
	}

	// 尝试打开文件验证格式
	_, err := zip.OpenReader(filePath)
	if err != nil {
		return fmt.Errorf("无效的 EPUB 文件: %w", err)
	}

	return nil
}

// InsertMonolingualTranslation 插入单语翻译（替换原文）
func InsertMonolingualTranslation(html string, translations map[string]string) string {
	var buf bytes.Buffer
	decoder := xml.NewDecoder(strings.NewReader(html))
	var currentText strings.Builder
	var depth int
	var inSpan bool
	var inFont bool

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
			// 检测 font 标签
			if t.Name.Local == "font" {
				inFont = true
			}
			// br 标签处理
			if t.Name.Local == "br" {
				if currentText.Len() > 0 {
					original := strings.TrimSpace(currentText.String())
					if trans, ok := translations[original]; ok && trans != "" {
						// 需要回退并替换之前写入的原文
						// 这里简化处理，直接清空并写入翻译
					}
					currentText.Reset()
				}
			}

		case xml.EndElement:
			// 在 span 结束标签前替换内容
			if t.Name.Local == "span" && inSpan && currentText.Len() > 0 {
				original := strings.TrimSpace(currentText.String())
				// 如果有翻译，使用翻译；否则保留原文
				if trans, ok := translations[original]; ok && trans != "" {
					// 清除之前写入的原文内容，写入翻译
					bufStr := buf.String()
					lastSpanStart := strings.LastIndex(bufStr, "<span")
					if lastSpanStart != -1 {
						// 找到span开始标签的结束位置
						spanTagEnd := strings.Index(bufStr[lastSpanStart:], ">")
						if spanTagEnd != -1 {
							spanTagEnd += lastSpanStart + 1
							// 重构buffer：保留到span标签结束，然后加入翻译内容
							buf.Reset()
							buf.WriteString(bufStr[:spanTagEnd])
							buf.WriteString(trans)
						}
					}
				}
				buf.WriteString("</")
				buf.WriteString(t.Name.Local)
				buf.WriteString(">")
				currentText.Reset()
				inSpan = false
			} else if t.Name.Local == "font" && inFont && currentText.Len() > 0 {
				// 在 font 结束标签前替换内容
				original := strings.TrimSpace(currentText.String())
				if trans, ok := translations[original]; ok && trans != "" {
					bufStr := buf.String()
					lastFontStart := strings.LastIndex(bufStr, "<font")
					if lastFontStart != -1 {
						fontTagEnd := strings.Index(bufStr[lastFontStart:], ">")
						if fontTagEnd != -1 {
							fontTagEnd += lastFontStart + 1
							buf.Reset()
							buf.WriteString(bufStr[:fontTagEnd])
							buf.WriteString(trans)
						}
					}
				}
				buf.WriteString("</")
				buf.WriteString(t.Name.Local)
				buf.WriteString(">")
				currentText.Reset()
				inFont = false
			} else {
				// 在块级元素结束标签前替换内容
				if isBlockElement(t.Name.Local) && currentText.Len() > 0 {
					original := strings.TrimSpace(currentText.String())
					if trans, ok := translations[original]; ok && trans != "" {
						// 对于块级元素，需要更复杂的替换逻辑
						// 这里简化处理，直接在结束标签前添加翻译
						buf.WriteString(fmt.Sprintf(`<div class="translation">%s</div>`, trans))
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
			if inSpan || inFont {
				// 对于span和font内的文本，先收集，在结束标签时处理
				currentText.WriteString(strings.TrimSpace(text))
				currentText.WriteString(" ")
			} else {
				// 对于非span/font文本，检查是否需要替换
				trimmedText := strings.TrimSpace(text)
				if trans, ok := translations[trimmedText]; ok && trans != "" && trimmedText != "" {
					buf.WriteString(trans)
				} else {
					buf.WriteString(text)
				}
			}
		}
	}

	return buf.String()
}
