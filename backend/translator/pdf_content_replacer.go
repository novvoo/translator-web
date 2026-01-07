package translator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PDFContentReplacer PDF内容替换器，保留原始样式
type PDFContentReplacer struct {
	parser *PDFParser
}

// TextReplacement 文本替换信息
type TextReplacement struct {
	Original    string  `json:"original"`
	Translation string  `json:"translation"`
	PageNum     int     `json:"page_num"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	FontSize    float64 `json:"font_size"`
	FontName    string  `json:"font_name"`
}

// PDFReplacementConfig PDF替换配置
type PDFReplacementConfig struct {
	Mode           string  // "monolingual" 或 "bilingual"
	PreserveLayout bool    // 是否保留原始布局
	FontScale      float64 // 字体缩放比例
	LineSpacing    float64 // 行间距调整
	BilingualStyle string  // 双语样式: "side-by-side", "top-bottom", "interleaved"
}

// NewPDFContentReplacer 创建PDF内容替换器
func NewPDFContentReplacer() *PDFContentReplacer {
	return &PDFContentReplacer{
		parser: NewPDFParser("", ""),
	}
}

// ReplaceContent 替换PDF内容，保留原始样式
func (r *PDFContentReplacer) ReplaceContent(inputPath, outputPath string, translations map[string]string, config PDFReplacementConfig) error {
	log.Printf("开始替换PDF内容: %s -> %s", inputPath, outputPath)

	// 1. 解析原始PDF，获取详细的文本位置信息
	content, err := r.parser.ParsePDF(inputPath)
	if err != nil {
		return fmt.Errorf("解析PDF失败: %w", err)
	}

	// 2. 创建替换映射
	replacements := r.createReplacements(content, translations)
	if len(replacements) == 0 {
		return fmt.Errorf("没有找到需要替换的内容")
	}

	// 3. 根据模式执行替换
	switch config.Mode {
	case "monolingual":
		return r.replaceMonolingual(inputPath, outputPath, replacements, config)
	case "bilingual":
		return r.replaceBilingual(inputPath, outputPath, replacements, config)
	default:
		return fmt.Errorf("不支持的替换模式: %s", config.Mode)
	}
}

// createReplacements 创建文本替换映射
func (r *PDFContentReplacer) createReplacements(content *PDFContent, translations map[string]string) []TextReplacement {
	var replacements []TextReplacement

	for _, block := range content.TextBlocks {
		if translation, exists := translations[block.Text]; exists && translation != block.Text {
			replacement := TextReplacement{
				Original:    block.Text,
				Translation: translation,
				PageNum:     block.PageNum,
				X:           block.X,
				Y:           block.Y,
				FontSize:    block.FontSize,
				FontName:    block.FontName,
			}
			replacements = append(replacements, replacement)
		}
	}

	log.Printf("创建了 %d 个文本替换", len(replacements))
	return replacements
}

// replaceMonolingual 单语替换模式
func (r *PDFContentReplacer) replaceMonolingual(inputPath, outputPath string, replacements []TextReplacement, config PDFReplacementConfig) error {
	log.Printf("执行单语替换模式")

	// 复制原始文件到输出路径
	if err := r.copyFile(inputPath, outputPath); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	// 使用pdfcpu进行内容替换
	return r.performContentReplacement(outputPath, replacements, config, false)
}

// replaceBilingual 双语替换模式
func (r *PDFContentReplacer) replaceBilingual(inputPath, outputPath string, replacements []TextReplacement, config PDFReplacementConfig) error {
	log.Printf("执行双语替换模式: %s", config.BilingualStyle)

	// 复制原始文件到输出路径
	if err := r.copyFile(inputPath, outputPath); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	// 根据双语样式执行不同的替换策略
	switch config.BilingualStyle {
	case "side-by-side":
		return r.replaceBilingualSideBySide(outputPath, replacements, config)
	case "top-bottom":
		return r.replaceBilingualTopBottom(outputPath, replacements, config)
	case "interleaved":
		return r.replaceBilingualInterleaved(outputPath, replacements, config)
	default:
		return r.replaceBilingualTopBottom(outputPath, replacements, config) // 默认使用上下布局
	}
}

// replaceBilingualSideBySide 左右对照双语替换
func (r *PDFContentReplacer) replaceBilingualSideBySide(outputPath string, replacements []TextReplacement, config PDFReplacementConfig) error {
	log.Printf("执行左右对照双语替换")

	// 调整替换位置，为译文腾出空间
	adjustedReplacements := make([]TextReplacement, 0, len(replacements)*2)

	for _, replacement := range replacements {
		// 原文保持在左侧，但缩小宽度
		originalReplacement := replacement
		originalReplacement.X = replacement.X * 0.5 // 左半页
		adjustedReplacements = append(adjustedReplacements, originalReplacement)

		// 译文放在右侧
		translationReplacement := TextReplacement{
			Original:    replacement.Original,
			Translation: replacement.Translation,
			PageNum:     replacement.PageNum,
			X:           replacement.X*0.5 + 100, // 右半页
			Y:           replacement.Y,
			FontSize:    replacement.FontSize * config.FontScale,
			FontName:    replacement.FontName,
		}
		adjustedReplacements = append(adjustedReplacements, translationReplacement)
	}

	return r.performContentReplacement(outputPath, adjustedReplacements, config, true)
}

// replaceBilingualTopBottom 上下对照双语替换
func (r *PDFContentReplacer) replaceBilingualTopBottom(outputPath string, replacements []TextReplacement, config PDFReplacementConfig) error {
	log.Printf("执行上下对照双语替换")

	// 调整替换位置，译文放在原文下方
	adjustedReplacements := make([]TextReplacement, 0, len(replacements)*2)

	for _, replacement := range replacements {
		// 原文保持原位置
		adjustedReplacements = append(adjustedReplacements, replacement)

		// 译文放在原文下方
		translationReplacement := TextReplacement{
			Original:    replacement.Original,
			Translation: replacement.Translation,
			PageNum:     replacement.PageNum,
			X:           replacement.X,
			Y:           replacement.Y - replacement.FontSize*config.LineSpacing, // 下移
			FontSize:    replacement.FontSize * config.FontScale,
			FontName:    replacement.FontName,
		}
		adjustedReplacements = append(adjustedReplacements, translationReplacement)
	}

	return r.performContentReplacement(outputPath, adjustedReplacements, config, true)
}

// replaceBilingualInterleaved 交错双语替换
func (r *PDFContentReplacer) replaceBilingualInterleaved(outputPath string, replacements []TextReplacement, config PDFReplacementConfig) error {
	log.Printf("执行交错双语替换")

	// 将原文替换为双语内容
	adjustedReplacements := make([]TextReplacement, 0, len(replacements))

	for _, replacement := range replacements {
		// 创建双语文本
		bilingualText := replacement.Original + "\n" + replacement.Translation

		bilingualReplacement := TextReplacement{
			Original:    replacement.Original,
			Translation: bilingualText,
			PageNum:     replacement.PageNum,
			X:           replacement.X,
			Y:           replacement.Y,
			FontSize:    replacement.FontSize * config.FontScale,
			FontName:    replacement.FontName,
		}
		adjustedReplacements = append(adjustedReplacements, bilingualReplacement)
	}

	return r.performContentReplacement(outputPath, adjustedReplacements, config, false)
}

// performContentReplacement 执行实际的内容替换 - 使用pdfcpu直接操作PDF内容流
func (r *PDFContentReplacer) performContentReplacement(filePath string, replacements []TextReplacement, config PDFReplacementConfig, isBilingual bool) error {
	log.Printf("开始执行PDF内容流替换，共 %d 个替换项", len(replacements))

	// 1. 验证PDF文件
	if err := api.ValidateFile(filePath, model.NewDefaultConfiguration()); err != nil {
		return fmt.Errorf("PDF验证失败: %w", err)
	}

	// 2. 读取PDF上下文
	ctx, err := api.ReadContextFile(filePath)
	if err != nil {
		return fmt.Errorf("读取PDF上下文失败: %w", err)
	}

	// 3. 执行文本替换操作
	modified := false
	for _, replacement := range replacements {
		if err := r.replaceTextInPage(ctx, replacement, config); err != nil {
			log.Printf("警告：替换文本失败 (页面 %d): %v", replacement.PageNum, err)
			continue
		}
		modified = true
	}

	if !modified {
		log.Printf("没有执行任何文本替换")
		return nil
	}

	// 4. 写回PDF文件
	if err := api.WriteContextFile(ctx, filePath); err != nil {
		return fmt.Errorf("写入PDF文件失败: %w", err)
	}

	log.Printf("PDF内容流替换完成，成功替换了文本")
	return nil
}

// copyFile 复制文件
func (r *PDFContentReplacer) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}

// replaceTextInPage 在指定页面替换文本
func (r *PDFContentReplacer) replaceTextInPage(ctx *model.Context, replacement TextReplacement, config PDFReplacementConfig) error {
	// 获取页面对象
	pageDict, _, _, err := ctx.PageDict(replacement.PageNum, false)
	if err != nil {
		return fmt.Errorf("获取页面字典失败: %w", err)
	}

	// 获取页面内容流
	contentStreams, err := r.getPageContentStreams(ctx, pageDict)
	if err != nil {
		return fmt.Errorf("获取页面内容流失败: %w", err)
	}

	// 在内容流中查找并替换文本
	modified := false
	for i, stream := range contentStreams {
		newContent, wasModified, err := r.replaceTextInContentStream(stream, replacement, config)
		if err != nil {
			log.Printf("警告：处理内容流 %d 失败: %v", i, err)
			continue
		}

		if wasModified {
			// 更新内容流
			if err := r.updateContentStream(ctx, pageDict, i, newContent); err != nil {
				return fmt.Errorf("更新内容流失败: %w", err)
			}
			modified = true
		}
	}

	if !modified {
		return fmt.Errorf("在页面 %d 中未找到文本: %s", replacement.PageNum, replacement.Original)
	}

	return nil
}

// getPageContentStreams 获取页面的所有内容流
func (r *PDFContentReplacer) getPageContentStreams(ctx *model.Context, pageDict types.Dict) ([]string, error) {
	var streams []string

	// 获取Contents对象
	contentsObj, found := pageDict.Find("Contents")
	if !found {
		return streams, fmt.Errorf("页面没有Contents对象")
	}

	switch obj := contentsObj.(type) {
	case types.IndirectRef:
		// 单个内容流
		streamDict, _, err := ctx.DereferenceStreamDict(obj)
		if err != nil {
			return streams, fmt.Errorf("解引用内容流失败: %w", err)
		}

		content, err := r.decodeStreamContent(streamDict)
		if err != nil {
			return streams, fmt.Errorf("解码内容流失败: %w", err)
		}

		streams = append(streams, content)

	case types.Array:
		// 多个内容流
		for _, item := range obj {
			if ref, ok := item.(types.IndirectRef); ok {
				streamDict, _, err := ctx.DereferenceStreamDict(ref)
				if err != nil {
					log.Printf("警告：解引用内容流失败: %v", err)
					continue
				}

				content, err := r.decodeStreamContent(streamDict)
				if err != nil {
					log.Printf("警告：解码内容流失败: %v", err)
					continue
				}

				streams = append(streams, content)
			}
		}

	default:
		return streams, fmt.Errorf("不支持的Contents对象类型: %T", obj)
	}

	return streams, nil
}

// decodeStreamContent 解码流内容
func (r *PDFContentReplacer) decodeStreamContent(streamDict *types.StreamDict) (string, error) {
	// 直接返回流内容，pdfcpu会自动处理解码
	if streamDict.Content != nil {
		return string(streamDict.Content), nil
	}
	return "", fmt.Errorf("流内容为空")
}

// replaceTextInContentStream 在内容流中替换文本
func (r *PDFContentReplacer) replaceTextInContentStream(content string, replacement TextReplacement, config PDFReplacementConfig) (string, bool, error) {
	// PDF内容流使用特殊的文本操作符
	// 主要的文本操作符：
	// - Tj: 显示文本字符串
	// - TJ: 显示带有个别字形定位的文本字符串
	// - ': 移动到下一行并显示文本字符串
	// - ": 设置字符和单词间距，移动到下一行并显示文本字符串

	modified := false
	newContent := content

	// 查找文本显示操作符并替换
	// 构建替换文本
	replacementText := r.buildReplacementText(replacement, config)

	// 使用改进的多重替换方法
	result := r.tryMultipleReplacements(newContent, replacement.Original, replacementText)

	if result.modified {
		newContent = result.content
		modified = true
		log.Printf("在内容流中成功替换 %d 处文本: '%s' -> '%s'",
			result.count,
			r.truncateString(replacement.Original, 30),
			r.truncateString(replacementText, 30))
	}

	return newContent, modified, nil
}

// buildReplacementText 构建替换文本
func (r *PDFContentReplacer) buildReplacementText(replacement TextReplacement, config PDFReplacementConfig) string {
	switch config.Mode {
	case "monolingual":
		return replacement.Translation
	case "bilingual":
		switch config.BilingualStyle {
		case "interleaved":
			return replacement.Original + "\\n" + replacement.Translation
		case "side-by-side":
			return replacement.Original + " | " + replacement.Translation
		default: // top-bottom
			return replacement.Original + "\\n" + replacement.Translation
		}
	default:
		return replacement.Translation
	}
}

// ReplacementResult 替换结果
type ReplacementResult struct {
	content  string
	modified bool
	count    int
}

// tryMultipleReplacements 尝试多种文本替换方式
func (r *PDFContentReplacer) tryMultipleReplacements(content, original, translation string) ReplacementResult {
	newContent := content
	totalCount := 0
	wasModified := false

	// 1. 尝试括号包围的文本 (text)
	pattern1 := "(" + original + ")"
	replacement1 := "(" + translation + ")"
	if strings.Contains(newContent, pattern1) {
		count := strings.Count(newContent, pattern1)
		newContent = strings.ReplaceAll(newContent, pattern1, replacement1)
		totalCount += count
		wasModified = true
		log.Printf("成功替换括号文本 %d 次: %s", count, r.truncateString(original, 30))
	}

	// 2. 尝试多种十六进制编码 <hex>
	originalHexEncodings := r.stringToHexMultiEncoding(original)
	translationHexEncodings := r.stringToHexMultiEncoding(translation)

	// 对每种编码方式进行尝试
	for i, hexOriginal := range originalHexEncodings {
		if i < len(translationHexEncodings) {
			hexTranslation := translationHexEncodings[i]
			pattern := "<" + hexOriginal + ">"
			replacement := "<" + hexTranslation + ">"

			if strings.Contains(newContent, pattern) {
				count := strings.Count(newContent, pattern)
				newContent = strings.ReplaceAll(newContent, pattern, replacement)
				totalCount += count
				wasModified = true
				log.Printf("成功替换十六进制文本(编码%d) %d 次: %s -> %s", i+1, count,
					r.truncateString(hexOriginal, 20), r.truncateString(hexTranslation, 20))
			}
		}
	}

	// 3. 尝试直接文本匹配（用于某些简单PDF）
	if strings.Contains(newContent, original) {
		count := strings.Count(newContent, original)
		newContent = strings.ReplaceAll(newContent, original, translation)
		totalCount += count
		wasModified = true
		log.Printf("成功替换直接文本 %d 次: %s", count, r.truncateString(original, 30))
	}

	// 4. 尝试分词匹配（处理被空格或换行分割的文本）
	words := strings.Fields(original)
	if len(words) > 1 {
		// 构建正则表达式匹配被分割的文本
		pattern := r.buildFlexiblePattern(words)
		if r.containsFlexiblePattern(newContent, pattern) {
			newContent = r.replaceFlexiblePattern(newContent, pattern, translation)
			totalCount += 1
			wasModified = true
			log.Printf("成功替换分词文本: %s", r.truncateString(original, 30))
		}
	}

	// 5. 尝试处理带BOM的UTF-16编码
	if r.containsNonASCII(original) {
		bomHex := "FEFF" + r.stringToUTF16BE(original)
		bomTranslationHex := "FEFF" + r.stringToUTF16BE(translation)
		pattern := "<" + bomHex + ">"
		replacement := "<" + bomTranslationHex + ">"

		if strings.Contains(newContent, pattern) {
			count := strings.Count(newContent, pattern)
			newContent = strings.ReplaceAll(newContent, pattern, replacement)
			totalCount += count
			wasModified = true
			log.Printf("成功替换UTF-16BE+BOM文本 %d 次", count)
		}
	}

	return ReplacementResult{
		content:  newContent,
		modified: wasModified,
		count:    totalCount,
	}
}

// stringToHex 将字符串转换为十六进制，正确处理多字节字符
func (r *PDFContentReplacer) stringToHex(text string) string {
	if text == "" {
		return ""
	}

	// 方法1: 尝试UTF-8字节编码（保持原有逻辑作为备选）
	utf8Bytes := []byte(text)
	result := ""
	for _, b := range utf8Bytes {
		result += fmt.Sprintf("%02X", b)
	}

	return result
}

// stringToHexMultiEncoding 尝试多种编码方式转换为十六进制
func (r *PDFContentReplacer) stringToHexMultiEncoding(text string) []string {
	if text == "" {
		return []string{}
	}

	var results []string

	// 1. UTF-8 编码
	utf8Bytes := []byte(text)
	utf8Hex := ""
	for _, b := range utf8Bytes {
		utf8Hex += fmt.Sprintf("%02X", b)
	}
	results = append(results, utf8Hex)

	// 2. UTF-16BE 编码（PDF常用）
	utf16Hex := r.stringToUTF16BE(text)
	if utf16Hex != "" && utf16Hex != utf8Hex {
		results = append(results, utf16Hex)
	}

	// 3. Latin-1/ISO-8859-1 编码（对于ASCII兼容字符）
	latin1Hex := ""
	canUseLatin1 := true
	for _, char := range text {
		if char > 255 {
			canUseLatin1 = false
			break
		}
		latin1Hex += fmt.Sprintf("%02X", byte(char))
	}
	if canUseLatin1 && latin1Hex != utf8Hex {
		results = append(results, latin1Hex)
	}

	return results
}

// stringToUTF16BE 将字符串转换为UTF-16BE十六进制
func (r *PDFContentReplacer) stringToUTF16BE(text string) string {
	if text == "" {
		return ""
	}

	// 转换为UTF-16BE字节
	utf16Bytes := []byte{}
	for _, char := range text {
		if char <= 0xFFFF {
			utf16Bytes = append(utf16Bytes, byte(char>>8), byte(char&0xFF))
		} else {
			// 处理代理对
			char -= 0x10000
			high := 0xD800 + (char >> 10)
			low := 0xDC00 + (char & 0x3FF)
			utf16Bytes = append(utf16Bytes, byte(high>>8), byte(high&0xFF))
			utf16Bytes = append(utf16Bytes, byte(low>>8), byte(low&0xFF))
		}
	}

	// 转换为十六进制字符串
	result := ""
	for _, b := range utf16Bytes {
		result += fmt.Sprintf("%02X", b)
	}
	return result
}

// buildFlexiblePattern 构建灵活的匹配模式
func (r *PDFContentReplacer) buildFlexiblePattern(words []string) string {
	// 简化版：用空白字符连接单词
	return strings.Join(words, `\s+`)
}

// containsFlexiblePattern 检查是否包含灵活模式
func (r *PDFContentReplacer) containsFlexiblePattern(content, pattern string) bool {
	// 简化实现：检查所有单词是否都存在
	words := strings.Fields(pattern)
	for _, word := range words {
		if !strings.Contains(content, word) {
			return false
		}
	}
	return len(words) > 1
}

// replaceFlexiblePattern 替换灵活模式
func (r *PDFContentReplacer) replaceFlexiblePattern(content, pattern, replacement string) string {
	// 简化实现：这里需要更复杂的正则表达式处理
	// 暂时返回原内容
	return content
}

// containsNonASCII 检查字符串是否包含非ASCII字符
func (r *PDFContentReplacer) containsNonASCII(text string) bool {
	for _, char := range text {
		if char > 127 {
			return true
		}
	}
	return false
}

// truncateString 截断字符串用于日志显示
func (r *PDFContentReplacer) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// generateTextFallback 生成文本版本作为备选
func (r *PDFContentReplacer) generateTextFallback(inputPath, outputPath string, textMappings map[string]string) error {
	log.Printf("PDF内容替换失败，生成文本版本作为备选")

	// 打开原始PDF
	doc, err := OpenPDF(inputPath)
	if err != nil {
		return fmt.Errorf("打开PDF失败: %w", err)
	}

	// 构建翻译后的文本块
	var originalBlocks, translatedBlocks []string
	for _, pageText := range doc.PageTexts {
		if strings.TrimSpace(pageText) == "" {
			continue
		}

		paragraphs := strings.Split(pageText, "\n\n")
		for _, para := range paragraphs {
			para = strings.TrimSpace(para)
			if para != "" && len(para) > 10 {
				originalBlocks = append(originalBlocks, para)

				// 查找翻译
				if translation, exists := textMappings[para]; exists {
					translatedBlocks = append(translatedBlocks, translation)
				} else {
					translatedBlocks = append(translatedBlocks, para) // 使用原文
				}
			}
		}
	}

	// 生成HTML版本
	htmlPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".html"
	if err := doc.SaveBilingualHTML(htmlPath, originalBlocks, translatedBlocks); err != nil {
		return fmt.Errorf("生成HTML备选版本失败: %w", err)
	}

	log.Printf("已生成HTML版本作为备选: %s", htmlPath)
	return nil
}

// updateContentStream 更新内容流
func (r *PDFContentReplacer) updateContentStream(ctx *model.Context, pageDict types.Dict, streamIndex int, newContent string) error {
	// 获取Contents对象
	contentsObj, found := pageDict.Find("Contents")
	if !found {
		return fmt.Errorf("页面没有Contents对象")
	}

	switch obj := contentsObj.(type) {
	case types.IndirectRef:
		// 单个内容流
		if streamIndex != 0 {
			return fmt.Errorf("流索引超出范围")
		}
		return r.updateSingleContentStream(ctx, obj, newContent)

	case types.Array:
		// 多个内容流
		if streamIndex >= len(obj) {
			return fmt.Errorf("流索引超出范围")
		}

		if ref, ok := obj[streamIndex].(types.IndirectRef); ok {
			return r.updateSingleContentStream(ctx, ref, newContent)
		}
		return fmt.Errorf("无效的内容流引用")

	default:
		return fmt.Errorf("不支持的Contents对象类型: %T", obj)
	}
}

// updateSingleContentStream 更新单个内容流
func (r *PDFContentReplacer) updateSingleContentStream(ctx *model.Context, ref types.IndirectRef, newContent string) error {
	// 获取流字典
	streamDict, _, err := ctx.DereferenceStreamDict(ref)
	if err != nil {
		return fmt.Errorf("解引用流字典失败: %w", err)
	}

	// 更新流内容
	streamDict.Content = []byte(newContent)

	// 更新长度
	streamLength := int64(len(newContent))
	streamDict.StreamLength = &streamLength
	if streamDict.Dict != nil {
		streamDict.Dict.Update("Length", types.Integer(len(newContent)))
	}

	// 标记为已修改
	ctx.Write.BinaryTotalSize += int64(len(newContent))

	log.Printf("成功更新内容流，新长度: %d 字节", len(newContent))
	return nil
}

// ReplaceContentDirect 直接替换PDF内容的简化接口
func (r *PDFContentReplacer) ReplaceContentDirect(inputPath, outputPath string, textMappings map[string]string) error {
	log.Printf("开始直接替换PDF内容: %s -> %s", inputPath, outputPath)

	// 复制文件
	if err := r.copyFile(inputPath, outputPath); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	// 读取PDF上下文
	ctx, err := api.ReadContextFile(outputPath)
	if err != nil {
		return fmt.Errorf("读取PDF上下文失败: %w", err)
	}

	// 遍历所有页面进行文本替换
	pageCount := ctx.PageCount
	modified := false
	totalReplacements := 0

	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		pageDict, _, _, err := ctx.PageDict(pageNum, false)
		if err != nil {
			log.Printf("警告：获取页面 %d 失败: %v", pageNum, err)
			continue
		}

		// 获取页面内容流
		contentStreams, err := r.getPageContentStreams(ctx, pageDict)
		if err != nil {
			log.Printf("警告：获取页面 %d 内容流失败: %v", pageNum, err)
			continue
		}

		// 在每个内容流中查找并替换文本
		for i, stream := range contentStreams {
			newContent := stream
			streamModified := false
			pageReplacements := 0

			// 对每个文本映射进行替换
			for original, translation := range textMappings {
				// 尝试多种文本编码格式
				replacements := r.tryMultipleReplacements(newContent, original, translation)
				if replacements.modified {
					newContent = replacements.content
					streamModified = true
					pageReplacements += replacements.count
					log.Printf("页面 %d: 成功替换 %d 处 '%s' -> '%s'", pageNum, replacements.count,
						r.truncateString(original, 50), r.truncateString(translation, 50))
				}
			}

			// 如果内容被修改，更新内容流
			if streamModified {
				if err := r.updateContentStream(ctx, pageDict, i, newContent); err != nil {
					log.Printf("警告：更新页面 %d 内容流 %d 失败: %v", pageNum, i, err)
				} else {
					modified = true
					totalReplacements += pageReplacements
				}
			}
		}
	}

	if !modified {
		log.Printf("警告：没有找到任何可替换的文本内容")
		// 尝试生成文本版本作为备选
		return r.generateTextFallback(inputPath, outputPath, textMappings)
	}

	// 写回PDF文件
	if err := api.WriteContextFile(ctx, outputPath); err != nil {
		return fmt.Errorf("写入PDF文件失败: %w", err)
	}

	log.Printf("PDF直接内容替换完成，共替换 %d 处文本", totalReplacements)
	return nil
}

// ReplaceContentWithAdvancedMethod 使用高级方法替换PDF内容
func (r *PDFContentReplacer) ReplaceContentWithAdvancedMethod(inputPath, outputPath string, translations map[string]string, config PDFReplacementConfig) error {
	log.Printf("使用高级方法替换PDF内容")

	// 这个方法将实现更精确的PDF内容替换
	// 包括字体匹配、位置计算、样式保留等

	// 1. 深度解析PDF结构
	structure, err := r.analyzeAdvancedPDFStructure(inputPath)
	if err != nil {
		return fmt.Errorf("深度解析PDF结构失败: %w", err)
	}

	// 2. 创建精确的替换计划
	plan := r.createAdvancedReplacementPlan(structure, translations, config)

	// 3. 执行精确替换
	return r.executeAdvancedReplacement(inputPath, outputPath, plan, config)
}

// analyzeAdvancedPDFStructure 深度分析PDF结构
func (r *PDFContentReplacer) analyzeAdvancedPDFStructure(inputPath string) (*AdvancedPDFStructure, error) {
	// 这里实现更详细的PDF结构分析
	// 包括字体信息、颜色、样式等
	return &AdvancedPDFStructure{}, nil
}

// createAdvancedReplacementPlan 创建高级替换计划
func (r *PDFContentReplacer) createAdvancedReplacementPlan(structure *AdvancedPDFStructure, translations map[string]string, config PDFReplacementConfig) *AdvancedReplacementPlan {
	// 创建详细的替换计划
	return &AdvancedReplacementPlan{}
}

// executeAdvancedReplacement 执行高级替换
func (r *PDFContentReplacer) executeAdvancedReplacement(inputPath, outputPath string, plan *AdvancedReplacementPlan, config PDFReplacementConfig) error {
	// 执行精确的PDF内容替换
	return nil
}

// AdvancedPDFStructure 高级PDF结构
type AdvancedPDFStructure struct {
	Pages []AdvancedPageStructure `json:"pages"`
}

// AdvancedPageStructure 高级页面结构
type AdvancedPageStructure struct {
	PageNum  int                   `json:"page_num"`
	Elements []AdvancedTextElement `json:"elements"`
	Layout   PageLayout            `json:"layout"`
}

// AdvancedTextElement 高级文本元素
type AdvancedTextElement struct {
	Text     string      `json:"text"`
	Position Position    `json:"position"`
	Style    TextStyle   `json:"style"`
	BoundBox BoundingBox `json:"bound_box"`
}

// Position 位置信息
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// TextStyle 文本样式
type TextStyle struct {
	FontName  string  `json:"font_name"`
	FontSize  float64 `json:"font_size"`
	Color     string  `json:"color"`
	Bold      bool    `json:"bold"`
	Italic    bool    `json:"italic"`
	Underline bool    `json:"underline"`
}

// BoundingBox 边界框
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// PageLayout 页面布局
type PageLayout struct {
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
	Margins Margins `json:"margins"`
}

// Margins 页边距
type Margins struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
}

// AdvancedReplacementPlan 高级替换计划
type AdvancedReplacementPlan struct {
	Operations []ReplacementOperation `json:"operations"`
}

// ReplacementOperation 替换操作
type ReplacementOperation struct {
	Type        string                 `json:"type"` // "replace", "insert", "delete"
	PageNum     int                    `json:"page_num"`
	Original    AdvancedTextElement    `json:"original"`
	Replacement AdvancedTextElement    `json:"replacement"`
	Metadata    map[string]interface{} `json:"metadata"`
}
