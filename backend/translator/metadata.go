package translator

import (
	"encoding/xml"
	"strings"
)

// TranslateMetadata 翻译 EPUB 元数据
func TranslateMetadata(epub *EPUBFile, client interface{}, targetLanguage, userPrompt string, cache *Cache) error {
	// 查找 OPF 文件
	var opfPath string
	var opfContent []byte
	for name, content := range epub.Files {
		if strings.HasSuffix(name, ".opf") {
			opfPath = name
			opfContent = content
			break
		}
	}

	if opfPath == "" {
		return nil // 没有找到 OPF 文件
	}

	// 解析 OPF
	type Metadata struct {
		XMLName xml.Name `xml:"metadata"`
		Title   []string `xml:"title"`
		Creator []string `xml:"creator"`
		Subject []string `xml:"subject"`
		Desc    []string `xml:"description"`
	}

	type Package struct {
		XMLName  xml.Name `xml:"package"`
		Metadata Metadata `xml:"metadata"`
	}

	var pkg Package
	if err := xml.Unmarshal(opfContent, &pkg); err != nil {
		// 如果解析失败，跳过元数据翻译
		return nil
	}

	// 收集需要翻译的字段
	var fieldsToTranslate []string

	// 标题
	for _, title := range pkg.Metadata.Title {
		if strings.TrimSpace(title) != "" {
			fieldsToTranslate = append(fieldsToTranslate, title)
		}
	}

	// 描述
	for _, desc := range pkg.Metadata.Desc {
		if strings.TrimSpace(desc) != "" {
			fieldsToTranslate = append(fieldsToTranslate, desc)
		}
	}

	// 主题
	for _, subject := range pkg.Metadata.Subject {
		if strings.TrimSpace(subject) != "" {
			fieldsToTranslate = append(fieldsToTranslate, subject)
		}
	}

	if len(fieldsToTranslate) == 0 {
		return nil
	}

	// 批量翻译
	translated, err := translateTitlesWithCache(fieldsToTranslate, client, targetLanguage, userPrompt, cache)
	if err != nil {
		return err
	}

	// 更新 OPF 内容（简化实现：字符串替换）
	updatedContent := string(opfContent)
	for i, original := range fieldsToTranslate {
		if i < len(translated) {
			// 在原文后添加翻译（双语显示）
			bilingual := original + " / " + translated[i]
			updatedContent = strings.Replace(updatedContent, ">"+original+"<", ">"+bilingual+"<", 1)
		}
	}

	epub.Files[opfPath] = []byte(updatedContent)
	return nil
}
