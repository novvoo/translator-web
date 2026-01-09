package translator

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// TOCItem 目录项
type TOCItem struct {
	Title    string
	Href     string
	Children []*TOCItem
}

// ParseTOC 解析 EPUB 目录（NCX 或 NAV）
func ParseTOC(epub *EPUBFile) ([]*TOCItem, error) {
	// 尝试查找 NCX 文件（EPUB 2.0）
	for name, content := range epub.Files {
		if strings.HasSuffix(name, ".ncx") {
			return parseNCX(content)
		}
	}

	// 尝试查找 NAV 文件（EPUB 3.0）
	for name, content := range epub.Files {
		if strings.Contains(strings.ToLower(name), "nav") &&
			(strings.HasSuffix(name, ".xhtml") || strings.HasSuffix(name, ".html")) {
			return parseNAV(content)
		}
	}

	return nil, nil // 没有找到目录
}

// parseNCX 解析 NCX 格式目录
func parseNCX(content []byte) ([]*TOCItem, error) {
	type NavPoint struct {
		XMLName  xml.Name `xml:"navPoint"`
		ID       string   `xml:"id,attr"`
		NavLabel struct {
			Text string `xml:"text"`
		} `xml:"navLabel"`
		Content struct {
			Src string `xml:"src,attr"`
		} `xml:"content"`
		NavPoints []NavPoint `xml:"navPoint"`
	}

	type NCX struct {
		XMLName xml.Name `xml:"ncx"`
		NavMap  struct {
			NavPoints []NavPoint `xml:"navPoint"`
		} `xml:"navMap"`
	}

	var ncx NCX
	if err := xml.Unmarshal(content, &ncx); err != nil {
		return nil, err
	}

	var items []*TOCItem
	for _, np := range ncx.NavMap.NavPoints {
		items = append(items, convertNavPoint(np))
	}

	return items, nil
}

func convertNavPoint(np any) *TOCItem {
	// 简化实现：直接返回基本结构
	item := &TOCItem{
		Title: "Chapter",
		Href:  "",
	}
	return item
}

// parseNAV 解析 NAV 格式目录（EPUB 3.0）
func parseNAV(content []byte) ([]*TOCItem, error) {
	// 简化实现：提取 nav 标签中的 ol/li 结构
	str := string(content)

	// 查找 <nav> 标签
	navStart := strings.Index(str, "<nav")
	if navStart == -1 {
		return nil, nil
	}

	navEnd := strings.Index(str[navStart:], "</nav>")
	if navEnd == -1 {
		return nil, nil
	}

	// 这里应该用完整的 HTML 解析器，简化版本仅提取文本
	return nil, nil
}

// TranslateTOC 翻译目录
func TranslateTOC(items []*TOCItem, client any, targetLanguage, userPrompt string, cache *Cache) error {
	if len(items) == 0 {
		return nil
	}

	// 收集所有标题
	var titles []string
	collectTitles(items, &titles)

	// 批量翻译
	translated, err := translateTitlesWithCache(titles, client, targetLanguage, userPrompt, cache)
	if err != nil {
		return err
	}

	// 填充翻译结果
	index := 0
	fillTitles(items, translated, &index)

	return nil
}

func collectTitles(items []*TOCItem, titles *[]string) {
	for _, item := range items {
		*titles = append(*titles, item.Title)
		if len(item.Children) > 0 {
			collectTitles(item.Children, titles)
		}
	}
}

func fillTitles(items []*TOCItem, translated []string, index *int) {
	for _, item := range items {
		if *index < len(translated) {
			item.Title = translated[*index]
			*index++
		}
		if len(item.Children) > 0 {
			fillTitles(item.Children, translated, index)
		}
	}
}

func translateTitlesWithCache(titles []string, client any, targetLanguage, userPrompt string, cache *Cache) ([]string, error) {
	results := make([]string, len(titles))

	for i, title := range titles {
		if title == "" {
			results[i] = ""
			continue
		}

		// 检查缓存
		if cache != nil {
			cacheKey := CacheKey(title, targetLanguage, userPrompt)
			if cached, ok := cache.Get(cacheKey); ok {
				results[i] = cached
				continue
			}
		}

		// 翻译 - 支持新旧两种客户端
		var translated string
		var err error

		switch c := client.(type) {
		case *TranslatorClient:
			translated, err = c.Translate(title, targetLanguage, userPrompt)
		case *LLMClient:
			translated, err = c.Translate(title, targetLanguage, userPrompt)
		default:
			return nil, fmt.Errorf("不支持的客户端类型")
		}

		if err != nil {
			return nil, fmt.Errorf("翻译标题失败: %w", err)
		}

		results[i] = translated

		// 保存到缓存
		if cache != nil {
			cacheKey := CacheKey(title, targetLanguage, userPrompt)
			cache.Set(cacheKey, translated)
		}
	}

	return results, nil
}

// WriteTOC 写回目录到 EPUB
func WriteTOC(epub *EPUBFile, items []*TOCItem) error {
	// 查找并更新 NCX 或 NAV 文件
	for name, content := range epub.Files {
		if strings.HasSuffix(name, ".ncx") {
			updated, err := updateNCX(content, items)
			if err != nil {
				return err
			}
			epub.Files[name] = updated
			return nil
		}
	}

	// 更新 NAV 文件
	for name, content := range epub.Files {
		if strings.Contains(strings.ToLower(name), "nav") {
			updated, err := updateNAV(content, items)
			if err != nil {
				return err
			}
			epub.Files[name] = updated
			return nil
		}
	}

	return nil
}

func updateNCX(content []byte, items []*TOCItem) ([]byte, error) {
	// 简化实现：直接返回原内容
	// 实际应该解析 XML 并更新标题
	_ = items // 使用参数避免未使用警告
	return content, nil
}

func updateNAV(content []byte, items []*TOCItem) ([]byte, error) {
	// 简化实现：直接返回原内容
	_ = items // 使用参数避免未使用警告
	return content, nil
}
