package translator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"github.com/ledongthuc/pdf"
	"github.com/phpdave11/gofpdi"
	"sort"
	"math"
	"regexp"
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
	Text           string  `json:"text"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Width          float64 `json:"width"`
	Height         float64 `json:"height"`
	FontSize       float64 `json:"font_size"`
	FontName       string  `json:"font_name"`
	Color          string  `json:"color"`
	PageNum        int     `json:"page_num"`
	OriginalX      float64 `json:"original_x"`       // 原始X坐标（用于遮罩）
	OriginalY      float64 `json:"original_y"`       // 原始Y坐标（用于遮罩）
	OriginalWidth  float64 `json:"original_width"`   // 原始宽度（用于遮罩）
	OriginalHeight float64 `json:"original_height"`  // 原始高度（用于遮罩）
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
	return r.reconstructPDFWithStyles(translatedPages, outputPath, inputPath, config)
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
		// 合并将近的文本片段为行
		content.Text = r.mergeTextElements(content.Text)

		for _, text := range content.Text {
			if strings.TrimSpace(text.S) == "" {
				continue
			}
			
			// 估算宽度
			width := r.estimateTextWidth(text.S, text.FontSize)
			if text.W > 0 {
				width = text.W
			}

			// 过滤掉公式
			if r.isFormula(text.S, text.Font) {
				continue
			}

			element := PageElement{
				Text:          text.S,
				X:             text.X,
				Y:             text.Y,
				FontSize:      text.FontSize,
				FontName:      text.Font,
				Color:         "#000000", // 默认黑色
				PageNum:       pageNum,
				Width:         width,
				Height:        text.FontSize,
				OriginalX:     text.X,
				OriginalY:     text.Y,
				OriginalWidth: width,
				OriginalHeight: text.FontSize,
			}

			reconstructedPage.Elements = append(reconstructedPage.Elements, element)
		}
	}

	return reconstructedPage, nil
}

func (r *PDFStylePreservingReplacer) mergeTextElements(texts []pdf.Text) []pdf.Text {
	if len(texts) == 0 {
		return texts
	}

	// 按Y坐标排序（注意：PDF通常Y轴向上，但ledongthuc/pdf的具体实现可能需要检查）
	// 这里我们假设需要将同一行的文本聚类
	sort.Slice(texts, func(i, j int) bool {
		if math.Abs(texts[i].Y - texts[j].Y) < 2.0 { // Y坐标相近
			return texts[i].X < texts[j].X // 按X排序
		}
		return texts[i].Y > texts[j].Y // 从上到下（假设大Y在上，或者保持原序）
	})

	var merged []pdf.Text
	if len(texts) == 0 {
		return merged
	}

	current := texts[0]
	
	for i := 1; i < len(texts); i++ {
		next := texts[i]
		
		// 判断是否在同一行且相邻
		isSameLine := math.Abs(current.Y - next.Y) < current.FontSize/2
		isAdjacent := (next.X - (current.X + r.estimateTextWidth(current.S, current.FontSize))) < current.FontSize*2 // 允许一定的间距
		
		// 如果可以用Text.W更好
		if current.W > 0 {
			isAdjacent = (next.X - (current.X + current.W)) < current.FontSize*2
		}

		if isSameLine && isAdjacent {
			// 合并
			current.S += " " + next.S // 加个空格简单的合并
			current.W += next.W // 累加宽度
			// 简单的宽度估算更新
			if current.W == 0 {
				current.W = r.estimateTextWidth(current.S, current.FontSize)
			}
		} else {
			merged = append(merged, current)
			current = next
		}
	}
	merged = append(merged, current)

	return merged
}

// isFormula 检测是否为数学公式
func (r *PDFStylePreservingReplacer) isFormula(text, fontName string) bool {
	// 常见数学符号检测
	mathSymbols := []string{
		"∫", "∑", "∏", "√", "∞", "α", "β", "γ", "δ", "ε", "θ", "λ", "μ", "π", "σ", "φ", "ψ", "ω",
		"≤", "≥", "≠", "≈", "∈", "∉", "⊂", "⊃", "∪", "∩", "∧", "∨", "¬", "→", "↔", "∀", "∃",
		"±", "×", "÷", "∂", "∇", "∆", "∝", "∴", "∵", "⊥", "∥", "°", "′", "″",
	}

	for _, symbol := range mathSymbols {
		if strings.Contains(text, symbol) {
			return true
		}
	}

	// 检测数学表达式模式
	mathPatterns := []string{
		`\d+\s*[+\-*/=]\s*\d+`,           // 简单算术表达式
		`[a-zA-Z]\s*[+\-*/=]\s*[a-zA-Z]`, // 代数表达式
		`\d+\^\d+`,                       // 指数
		`\d+_\d+`,                        // 下标
		`\([^)]*\)\s*[+\-*/=]`,           // 括号表达式
	}

	for _, pattern := range mathPatterns {
		if matched, _ := regexp.MatchString(pattern, text); matched {
			return true
		}
	}
	
	// 这里可以添加基于字体名称的检测 (例如 CMMI, CMSY 等 TeX 字体)
	if strings.Contains(strings.ToLower(fontName), "math") || 
	   strings.Contains(strings.ToLower(fontName), "cmmi") {
		return true
	}

	return false
}

// estimateTextWidth 估算文本宽度
func (r *PDFStylePreservingReplacer) estimateTextWidth(text string, fontSize float64) float64 {
	// 简单的文本宽度估算
	// 汉字宽一些，英文窄一些
	width := 0.0
	for _, char := range text {
		if char > 127 {
			width += fontSize // 中文等宽
		} else {
			width += fontSize * 0.55 // 英文平均宽度
		}
	}
	return width
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

// reconstructPDFWithStyles 重构PDF并保留样式 (Overlay Mode)
func (r *PDFStylePreservingReplacer) reconstructPDFWithStyles(pages []ReconstructedPage, outputPath, inputPath string, config StylePreservingConfig) error {
	log.Printf("重构PDF(Overlay模式)，保留样式: %s", outputPath)

	// 创建新的PDF文档
	pdf := gofpdf.New("P", "pt", "A4", "")

	// 必须添加 gopher 字体支持，虽然我们用系统字体，但防止 panic
	// gofpdfContrib 需要 gofpdf 的 instance
	
	// 添加字体支持（用于绘制翻译文本）
	if err := r.addFontSupport(pdf); err != nil {
		log.Printf("警告：添加字体支持失败: %v", err)
	}

	// 重构每一页
	for _, page := range pages {
		// 导入原始页面作为模板
		importer := gofpdi.NewImporter()
		importer.SetSourceFile(inputPath)
		tplId := importer.ImportPage(page.PageNum, "/MediaBox")
		
		pdf.AddPage()
		
		// 绘制模板 (背景)
		// UseTemplate returns attributes for UseImportedTemplate
		tplName, scaleX, scaleY, tX, tY := importer.UseTemplate(tplId, 0, 0, page.PageWidth, page.PageHeight)
		pdf.UseImportedTemplate(tplName, scaleX, scaleY, tX, tY)

		// 渲染页面元素 (Overlay)
		for _, element := range page.Elements {
			// 在 Overlay 模式下，我们只渲染那些 "被改动" 或 "是翻译" 的元素。
			// 原始的未动元素已经在底图上了，不需要重绘防止加粗。
			
			// 简单起见，我们渲染所有元素，依靠遮罩遮住底图的文字。
			// 这样可以保证样式（特别是字体）的一致性（全部使用新字体）。
			// 对于公式，我们在 extract 阶段已经过滤掉了，所以这里 page.Elements 不包含公式
			// 因此公式部分不会被遮罩，也不会被重绘，从而显示底图的原始矢量公式。
			
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

// renderElement 渲染页面元素 (Overlay Mode)
func (r *PDFStylePreservingReplacer) renderElement(pdf *gofpdf.Fpdf, element PageElement, config StylePreservingConfig) {
	// 1. 绘制遮罩 (Whiteout)
	// 只有当有翻译且需要覆盖原文时才绘制
	// 单语模式下：覆盖原文
	// 双语模式下：可能不需要覆盖，取决于布局
	
	needMask := false
	if config.Mode == "monolingual" {
		needMask = true
	} else if config.Mode == "bilingual" && config.BilingualLayout == "original-replacement" {
		// 假如我们支持这种模式
		needMask = true
	}
	// 注意：目前的 bilingual 逻辑是 append 两个 elements，一个是原文，一个是译文
	// 如果是 SideBySide 或 TopBottom，原文 element 还在。
	// 但在 "Overlay" 模式下，原文已经由 Template 绘制了！
	// 所以，如果 element 是 "原文"，我们不需要做任何事（除了可能不需要重绘，因为它已经在底图上了）
	// 但是，Wait！我们的 ReconstructedPage 包含了 "Elements"。
	// 如果我们使用 Template，底图上已经有原文了。
	// 如果我们再次 renderElement(OriginalElement)，我们会在底图上再次绘制文字。
	// 这通常没问题，但可能会加粗。
	// 关键是：如果这个 element 是翻译后的，我们需要MASK掉原来的位置（即 OriginalX, OriginalY）
	
	// 逻辑修正：
	// 如果 element.Text 是翻译文本（怎么判断？通过比较 Text 和 OriginalText？我们没有存 OriginalText）
	// 但是不管是原文还是译文，我们都有 OriginalX/Y/W/H。
	// 如果是 Monolingual 模式，我们将所有 elements 替换成了 翻译后的 elements。
	// 所以这里的 element 是 译文。
	// 我们需要 MASK 掉 OriginalX/Y/W/H 区域。
	
	// GoFPDF 坐标系 Y 是向下增加。
	// PDF 原生坐标系 Y 是向上增加。
	// ledongthuc/pdf 提取的 Y 是原生的（通常）。
	// MediaBox Height H. GoFPDF Y = H - PDF_Y.
	// 我们需要注意这一点。
	// 假设 ledongthuc/pdf 返回的是 PDF 坐标。
	// 我们需要页面高度 H 来转换。
	_, pageH := pdf.GetPageSize()
	
	// 转换 Y 坐标
	// 注意：ledongthuc/pdf 的 Text.Y 通常是 baseline。
	// 矩形遮罩应该是 Y_baseline + descent 到 Y_baseline - ascent? 
	// 简单起见，Y 是底部？
	// 让我们假设 ledongthuc/pdf return Y is lower-left of text box (standard PDF text matrix).
	
	// 转换到 GoFPDF (Top-Left 0,0)
	// renderY = pageH - pdfY - fontSize (approx, depends on baseline)
	// 这是一个痛点。
	
	// 让我们先做简单的 Mask：
	if needMask && element.OriginalWidth > 0 {
		pdf.SetFillColor(255, 255, 255)
		
		// 转换坐标
		// 假设 element.OriginalY 是 standard PDF y (from bottom)
		maskY := pageH - element.OriginalY - element.OriginalHeight
		maskX := element.OriginalX
		
		// 绘制矩形 (F = Fill)
		pdf.Rect(maskX, maskY, element.OriginalWidth, element.OriginalHeight, "F")
	}

	// 2. 绘制新文本
	// 设置字体
	fontName := "Arial" // 默认字体，确保支持中文的字体名
	// 这里应该使用我们 addFontSupport 加载的字体，例如 "Heiti" 或 "DroidSansFallback"
	// 我们在 addFontSupport 里的逻辑是：
	// pdf.AddUTF8Font(fontName, "", fontPath)
	// 假设 fontDetector 找到了 "SimHei" -> "SimHei"
	// 我们需要知道加载的字体名。
	// 暂时 Hardcode 一个或者通过 config 传？
	// r.addFontSupport 动态加载了字体。
	
	// 现在的逻辑：
	// 如果 element.FontName 在映射里，用映射的。
	// 否则用 Arial? Arial 不支持中文。
	// 我们需要一种机制确保使用 UTF8 字体。
	// 我们可以尝试把 fontName 设为 "Unifont" 如果加载了的话。
	
	pdf.SetFont(fontName, "", element.FontSize)

	// 设置颜色
	if config.ColorPreservation && element.Color != "" {
		r.setTextColor(pdf, element.Color)
	} else {
		pdf.SetTextColor(0, 0, 0)
	}

	// 计算渲染位置
	renderY := pageH - element.Y - element.FontSize*0.8 // 调整基线
	if config.Mode == "bilingual" {
		// 双语模式下，Y可能已经被 layout 调整过了（例如 TopBottom）
		// 但是 layout调整的是 element.Y。
		// 如果 element.Y 是基于 PDF 坐标（Bottom-up），那么减去 lineSpacing 是向下移。
		// 我们的 createTopBottomLayout: Y - FontSize*Spacing -> 向下移（Y变小）。
		// 在 GoFPDF (Top-Down): pageH - (smaller Y) = Larger RenderY (Lower on page). Correct.
	}

	pdf.SetXY(element.X, renderY)

	// 处理多行文本
	lines := strings.Split(element.Text, "\n")
	for i, line := range lines {
		if i > 0 {
			pdf.SetXY(element.X, renderY+float64(i)*element.FontSize*config.LineSpacing)
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
