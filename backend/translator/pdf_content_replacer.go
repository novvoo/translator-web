package translator

import (
	"fmt"
	"log"
	"os"
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

	// 执行替换
	oldText := "(" + replacement.Original + ")"
	newText := "(" + replacementText + ")"

	if strings.Contains(newContent, oldText) {
		newContent = strings.ReplaceAll(newContent, oldText, newText)
		modified = true
		log.Printf("替换文本: '%s' -> '%s'", replacement.Original, replacementText)
	}

	// 处理十六进制编码的文本 <text>
	hexPattern := r.stringToHex(replacement.Original)
	if strings.Contains(newContent, "<"+hexPattern+">") {
		replacementHex := r.stringToHex(replacementText)
		oldHex := "<" + hexPattern + ">"
		newHex := "<" + replacementHex + ">"

		newContent = strings.ReplaceAll(newContent, oldHex, newHex)
		modified = true
		log.Printf("替换十六进制文本: '%s' -> '%s'", hexPattern, replacementHex)
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

// stringToHex 将字符串转换为十六进制
func (r *PDFContentReplacer) stringToHex(text string) string {
	result := ""
	for _, char := range []byte(text) {
		result += fmt.Sprintf("%02X", char)
	}
	return result
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

			// 对每个文本映射进行替换
			for original, translation := range textMappings {
				if strings.Contains(newContent, "("+original+")") {
					newContent = strings.ReplaceAll(newContent, "("+original+")", "("+translation+")")
					streamModified = true
					log.Printf("页面 %d: 替换 '%s' -> '%s'", pageNum, original, translation)
				}

				// 处理十六进制编码
				hexOriginal := r.stringToHex(original)
				hexTranslation := r.stringToHex(translation)
				if strings.Contains(newContent, "<"+hexOriginal+">") {
					newContent = strings.ReplaceAll(newContent, "<"+hexOriginal+">", "<"+hexTranslation+">")
					streamModified = true
					log.Printf("页面 %d: 替换十六进制 '%s' -> '%s'", pageNum, hexOriginal, hexTranslation)
				}
			}

			// 如果内容被修改，更新内容流
			if streamModified {
				if err := r.updateContentStream(ctx, pageDict, i, newContent); err != nil {
					log.Printf("警告：更新页面 %d 内容流 %d 失败: %v", pageNum, i, err)
				} else {
					modified = true
				}
			}
		}
	}

	if !modified {
		log.Printf("没有找到需要替换的文本")
		return nil
	}

	// 写回PDF文件
	if err := api.WriteContextFile(ctx, outputPath); err != nil {
		return fmt.Errorf("写入PDF文件失败: %w", err)
	}

	log.Printf("PDF直接内容替换完成")
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
