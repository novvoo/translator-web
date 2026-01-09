package translator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"github.com/ledongthuc/pdf"
)

// PDFStylePreservingReplacer 保留样式的PDF替换器
type PDFStylePreservingReplacer struct {
	fontDetector *SystemFontDetector
}

// StylePreservingConfig 样式保留配置
type StylePreservingConfig struct {
	Mode               string  // "monolingual" 或 "bilingual"
	BilingualLayout    string  // "side-by-side", "top-bottom", "interleaved"
	PreserveFormatting bool    // 是否保留原始格式
	FontScale          float64 // 字体缩放比例
	LineSpacing        float64 // 行间距
	MarginAdjustment   float64 // 页边距调整
	ColorPreservation  bool    // 是否保留颜色
}

// PageElement 页面元素
type PageElement struct {
	Text     string  `json:"text"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	FontSize float64 `json:"font_size"`
	FontName string  `json:"font_name"`
	Color    string  `json:"color"`
	PageNum  int     `json:"page_num"`
}

// ReconstructedPage 重构的页面
type ReconstructedPage struct {
	PageNum    int           `json:"page_num"`
	Elements   []PageElement `json:"elements"`
	PageWidth  float64       `json:"page_width"`
	PageHeight float64       `json:"page_height"`
}

// NewPDFStylePreservingReplacer 创建样式保留替换器
func NewPDFStylePreservingReplacer() *PDFStylePreservingReplacer {
	return &PDFStylePreservingReplacer{
		fontDetector: NewSystemFontDetector(),
	}
}

// ReplaceWithStylePreservation 保留样式的替换
func (r *PDFStylePreservingReplacer) ReplaceWithStylePreservation(inputPath, outputPath string, translations map[string]string, config StylePreservingConfig) error {
	log.Printf("开始保留样式的PDF替换: %s -> %s", inputPath, outputPath)

	// 1. 深度解析原始PDF，提取所有样式信息
	pages, err := r.extractPagesWithStyles(inputPath)
	if err != nil {
		return fmt.Errorf("提取页面样式失败: %w", err)
	}

	// 2. 应用翻译，保留样式
	translatedPages := r.applyTranslationsWithStyles(pages, translations, config)

	// 3. 重新构建PDF，保留原始样式
	return r.reconstructPDFWithStyles(translatedPages, outputPath, config)
}

// extractPagesWithStyles 提取页面及其样式信息
func (r *PDFStylePreservingReplacer) extractPagesWithStyles(inputPath string) ([]ReconstructedPage, error) {
	log.Printf("提取PDF页面样式信息: %s", inputPath)

	file, reader, err := pdf.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("打开PDF失败: %w", err)
	}
	defer file.Close()

	var pages []ReconstructedPage
	pageCount := reader.NumPage()

	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		reconstructedPage, err := r.extractPageElements(page, pageNum)
		if err != nil {
			log.Printf("警告：提取第%d页元素失败: %v", pageNum, err)
			continue
		}

		pages = append(pages, reconstructedPage)
	}

	log.Printf("成功提取 %d 页的样式信息", len(pages))
	return pages, nil
}

// extractPageElements 提取页面元素
func (r *PDFStylePreservingReplacer) extractPageElements(page pdf.Page, pageNum int) (ReconstructedPage, error) {
	reconstructedPage := ReconstructedPage{
		PageNum:    pageNum,
		Elements:   make([]PageElement, 0),
		PageWidth:  595.28, // A4 默认宽度
		PageHeight: 841.89, // A4 默认高度
	}

	// 获取页面尺寸
	if mediaBox := page.V.Key("MediaBox"); !mediaBox.IsNull() {
		// 尝试解析页面尺寸
		// 这里简化处理，实际应该解析MediaBox数组
	}

	// 提取文本内容和位置信息
	content := page.Content()
	if content.Text != nil {
		for _, text := range content.Text {
			if strings.TrimSpace(text.S) == "" {
				continue
			}

			element := PageElement{
				Text:     text.S,
				X:        text.X,
				Y:        text.Y,
				FontSize: text.FontSize,
				FontName: text.Font,
				Color:    "#000000", // 默认黑色，实际应该从PDF中提取
				PageNum:  pageNum,
				Width:    r.estimateTextWidth(text.S, text.FontSize),
				Height:   text.FontSize,
			}

			reconstructedPage.Elements = append(reconstructedPage.Elements, element)
		}
	}

	return reconstructedPage, nil
}

// estimateTextWidth 估算文本宽度
func (r *PDFStylePreservingReplacer) estimateTextWidth(text string, fontSize float64) float64 {
	// 简单的文本宽度估算，实际应该根据字体进行精确计算
	return float64(len(text)) * fontSize * 0.6
}

// applyTranslationsWithStyles 应用翻译并保留样式
func (r *PDFStylePreservingReplacer) applyTranslationsWithStyles(pages []ReconstructedPage, translations map[string]string, config StylePreservingConfig) []ReconstructedPage {
	log.Printf("应用翻译，模式: %s", config.Mode)

	var result []ReconstructedPage

	for _, page := range pages {
		switch config.Mode {
		case "monolingual":
			result = append(result, r.applyMonolingualTranslation(page, translations, config))
		case "bilingual":
			result = append(result, r.applyBilingualTranslation(page, translations, config)...)
		default:
			result = append(result, r.applyMonolingualTranslation(page, translations, config))
		}
	}

	return result
}

// applyMonolingualTranslation 应用单语翻译
func (r *PDFStylePreservingReplacer) applyMonolingualTranslation(page ReconstructedPage, translations map[string]string, config StylePreservingConfig) ReconstructedPage {
	translatedPage := ReconstructedPage{
		PageNum:    page.PageNum,
		Elements:   make([]PageElement, 0, len(page.Elements)),
		PageWidth:  page.PageWidth,
		PageHeight: page.PageHeight,
	}

	for _, element := range page.Elements {
		translatedElement := element

		// 查找翻译
		if translation, exists := translations[element.Text]; exists {
			translatedElement.Text = translation

			// 调整字体大小以适应翻译文本
			if config.FontScale != 0 {
				translatedElement.FontSize *= config.FontScale
			}

			// 重新计算宽度
			translatedElement.Width = r.estimateTextWidth(translation, translatedElement.FontSize)
		}

		translatedPage.Elements = append(translatedPage.Elements, translatedElement)
	}

	return translatedPage
}

// applyBilingualTranslation 应用双语翻译
func (r *PDFStylePreservingReplacer) applyBilingualTranslation(page ReconstructedPage, translations map[string]string, config StylePreservingConfig) []ReconstructedPage {
	switch config.BilingualLayout {
	case "side-by-side":
		return r.createSideBySideLayout(page, translations, config)
	case "top-bottom":
		return r.createTopBottomLayout(page, translations, config)
	case "interleaved":
		return r.createInterleavedLayout(page, translations, config)
	default:
		return r.createTopBottomLayout(page, translations, config)
	}
}

// createSideBySideLayout 创建左右对照布局
func (r *PDFStylePreservingReplacer) createSideBySideLayout(page ReconstructedPage, translations map[string]string, config StylePreservingConfig) []ReconstructedPage {
	bilingualPage := ReconstructedPage{
		PageNum:    page.PageNum,
		Elements:   make([]PageElement, 0, len(page.Elements)*2),
		PageWidth:  page.PageWidth,
		PageHeight: page.PageHeight,
	}

	halfWidth := page.PageWidth / 2

	for _, element := range page.Elements {
		// 原文放在左侧
		originalElement := element
		originalElement.X = element.X * 0.5 // 缩放到左半页
		bilingualPage.Elements = append(bilingualPage.Elements, originalElement)

		// 译文放在右侧
		if translation, exists := translations[element.Text]; exists {
			translatedElement := element
			translatedElement.Text = translation
			translatedElement.X = halfWidth + element.X*0.5 // 右半页
			if config.FontScale != 0 {
				translatedElement.FontSize *= config.FontScale
			}
			translatedElement.Width = r.estimateTextWidth(translation, translatedElement.FontSize)
			bilingualPage.Elements = append(bilingualPage.Elements, translatedElement)
		}
	}

	return []ReconstructedPage{bilingualPage}
}

// createTopBottomLayout 创建上下对照布局
func (r *PDFStylePreservingReplacer) createTopBottomLayout(page ReconstructedPage, translations map[string]string, config StylePreservingConfig) []ReconstructedPage {
	bilingualPage := ReconstructedPage{
		PageNum:    page.PageNum,
		Elements:   make([]PageElement, 0, len(page.Elements)*2),
		PageWidth:  page.PageWidth,
		PageHeight: page.PageHeight * 2, // 增加页面高度
	}

	for _, element := range page.Elements {
		// 原文保持原位置
		bilingualPage.Elements = append(bilingualPage.Elements, element)

		// 译文放在下方
		if translation, exists := translations[element.Text]; exists {
			translatedElement := element
			translatedElement.Text = translation
			translatedElement.Y = element.Y - element.FontSize*config.LineSpacing // 下移
			if config.FontScale != 0 {
				translatedElement.FontSize *= config.FontScale
			}
			translatedElement.Width = r.estimateTextWidth(translation, translatedElement.FontSize)
			bilingualPage.Elements = append(bilingualPage.Elements, translatedElement)
		}
	}

	return []ReconstructedPage{bilingualPage}
}

// createInterleavedLayout 创建交错布局
func (r *PDFStylePreservingReplacer) createInterleavedLayout(page ReconstructedPage, translations map[string]string, config StylePreservingConfig) []ReconstructedPage {
	interleavedPage := ReconstructedPage{
		PageNum:    page.PageNum,
		Elements:   make([]PageElement, 0, len(page.Elements)),
		PageWidth:  page.PageWidth,
		PageHeight: page.PageHeight,
	}

	for _, element := range page.Elements {
		// 创建双语文本
		bilingualText := element.Text
		if translation, exists := translations[element.Text]; exists {
			bilingualText = element.Text + "\n" + translation
		}

		bilingualElement := element
		bilingualElement.Text = bilingualText
		if config.FontScale != 0 {
			bilingualElement.FontSize *= config.FontScale
		}
		bilingualElement.Height *= 2 // 增加高度以容纳两行文本
		bilingualElement.Width = r.estimateTextWidth(bilingualText, bilingualElement.FontSize)

		interleavedPage.Elements = append(interleavedPage.Elements, bilingualElement)
	}

	return []ReconstructedPage{interleavedPage}
}

// reconstructPDFWithStyles 重构PDF并保留样式
func (r *PDFStylePreservingReplacer) reconstructPDFWithStyles(pages []ReconstructedPage, outputPath string, config StylePreservingConfig) error {
	log.Printf("重构PDF，保留样式: %s", outputPath)

	// 创建新的PDF文档
	pdf := gofpdf.New("P", "pt", "A4", "")

	// 添加字体支持
	if err := r.addFontSupport(pdf); err != nil {
		log.Printf("警告：添加字体支持失败: %v", err)
	}

	// 重构每一页
	for _, page := range pages {
		pdf.AddPage()

		// 设置页面尺寸
		if page.PageWidth > 0 && page.PageHeight > 0 {
			pdf.SetPageBox("MediaBox", 0, 0, page.PageWidth, page.PageHeight)
		}

		// 渲染页面元素
		for _, element := range page.Elements {
			r.renderElement(pdf, element, config)
		}
	}

	// 保存PDF
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return fmt.Errorf("保存PDF失败: %w", err)
	}

	log.Printf("PDF重构完成: %s", outputPath)
	return nil
}

// addFontSupport 添加字体支持
func (r *PDFStylePreservingReplacer) addFontSupport(pdf *gofpdf.Fpdf) error {
	// 尝试添加通用字体支持
	fontPath := r.fontDetector.GetSystemFontPath("zh")
	if fontPath != "" && r.fileExists(fontPath) {
		fontName := strings.TrimSuffix(filepath.Base(fontPath), filepath.Ext(fontPath))

		// 使用AddUTF8Font方法，这个方法可以直接处理TTF文件
		pdf.AddUTF8Font(fontName, "", fontPath)

		// 检查是否成功
		if err := pdf.Error(); err != nil {
			log.Printf("添加字体失败: %v", err)
			return err
		}

		log.Printf("添加字体支持: %s", fontPath)
	}
	return nil
}

// renderElement 渲染页面元素
func (r *PDFStylePreservingReplacer) renderElement(pdf *gofpdf.Fpdf, element PageElement, config StylePreservingConfig) {
	// 设置字体
	fontName := "Arial" // 默认字体
	if element.FontName != "" {
		// 尝试映射PDF字体名到gofpdf字体名
		fontName = r.mapFontName(element.FontName)
	}

	pdf.SetFont(fontName, "", element.FontSize)

	// 设置颜色（如果配置要求保留颜色）
	if config.ColorPreservation && element.Color != "" {
		r.setTextColor(pdf, element.Color)
	}

	// 设置位置并渲染文本
	pdf.SetXY(element.X, element.Y)

	// 处理多行文本
	lines := strings.Split(element.Text, "\n")
	for i, line := range lines {
		if i > 0 {
			pdf.SetXY(element.X, element.Y+float64(i)*element.FontSize*config.LineSpacing)
		}
		pdf.Cell(element.Width, element.Height, line)
	}
}

// mapFontName 映射字体名称
func (r *PDFStylePreservingReplacer) mapFontName(pdfFontName string) string {
	// 简单的字体名称映射
	fontMap := map[string]string{
		"Times-Roman":    "Times",
		"Helvetica":      "Arial",
		"Courier":        "Courier",
		"Times-Bold":     "Times",
		"Helvetica-Bold": "Arial",
		"Courier-Bold":   "Courier",
	}

	if mappedName, exists := fontMap[pdfFontName]; exists {
		return mappedName
	}

	return "Arial" // 默认字体
}

// setTextColor 设置文本颜色
func (r *PDFStylePreservingReplacer) setTextColor(pdf *gofpdf.Fpdf, colorStr string) {
	// 简单的颜色解析，实际应该支持更多颜色格式
	if colorStr == "#000000" || colorStr == "black" {
		pdf.SetTextColor(0, 0, 0)
	} else if colorStr == "#FF0000" || colorStr == "red" {
		pdf.SetTextColor(255, 0, 0)
	} else if colorStr == "#0000FF" || colorStr == "blue" {
		pdf.SetTextColor(0, 0, 255)
	} else {
		pdf.SetTextColor(0, 0, 0) // 默认黑色
	}
}

// fileExists 检查文件是否存在
func (r *PDFStylePreservingReplacer) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetDefaultStylePreservingConfig 获取默认样式保留配置
func GetDefaultStylePreservingConfig() StylePreservingConfig {
	return StylePreservingConfig{
		Mode:               "monolingual",
		BilingualLayout:    "top-bottom",
		PreserveFormatting: true,
		FontScale:          1.0,
		LineSpacing:        1.2,
		MarginAdjustment:   0,
		ColorPreservation:  true,
	}
}

// GetBilingualStylePreservingConfig 获取双语样式保留配置
func GetBilingualStylePreservingConfig(layout string) StylePreservingConfig {
	config := GetDefaultStylePreservingConfig()
	config.Mode = "bilingual"
	config.BilingualLayout = layout
	config.FontScale = 0.9 // 双语模式下稍微缩小字体
	return config
}
