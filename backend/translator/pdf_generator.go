package translator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// PDFGenerator PDF生成器
type PDFGenerator struct {
	FontPath string
}

// BilingualPDFConfig 双语PDF配置
type BilingualPDFConfig struct {
	Title        string
	Author       string
	Subject      string
	Creator      string
	SourceLang   string
	TargetLang   string
	ShowOriginal bool
	FontSize     float64
	LineSpacing  float64
	Margin       float64
}

// NewPDFGenerator 创建PDF生成器
func NewPDFGenerator(fontPath string) *PDFGenerator {
	return &PDFGenerator{
		FontPath: fontPath,
	}
}

// GenerateBilingualPDF 生成双语PDF
func (g *PDFGenerator) GenerateBilingualPDF(originalContent, translatedContent *PDFContent, outputPath string, config BilingualPDFConfig) error {
	log.Printf("开始生成双语PDF: %s", outputPath)

	// 创建PDF文档
	pdf := gofpdf.New("P", "mm", "A4", "")

	// 设置文档属性
	pdf.SetTitle(config.Title, true)
	pdf.SetAuthor(config.Author, true)
	pdf.SetSubject(config.Subject, true)
	pdf.SetCreator(config.Creator, true)

	// 添加字体支持
	if err := g.addFonts(pdf); err != nil {
		log.Printf("警告：添加字体失败: %v", err)
	}

	// 设置默认字体
	pdf.SetFont("Arial", "", config.FontSize)

	// 生成页面
	if err := g.generatePages(pdf, originalContent, translatedContent, config); err != nil {
		return fmt.Errorf("生成页面失败: %w", err)
	}

	// 保存文件
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return fmt.Errorf("保存PDF文件失败: %w", err)
	}

	log.Printf("双语PDF生成完成: %s", outputPath)
	return nil
}

// GenerateMonolingualPDF 生成单语PDF
func (g *PDFGenerator) GenerateMonolingualPDF(content *PDFContent, outputPath string, config BilingualPDFConfig) error {
	log.Printf("开始生成单语PDF: %s", outputPath)

	// 创建PDF文档
	pdf := gofpdf.New("P", "mm", "A4", "")

	// 设置文档属性
	pdf.SetTitle(config.Title, true)
	pdf.SetAuthor(config.Author, true)
	pdf.SetSubject(config.Subject, true)
	pdf.SetCreator(config.Creator, true)

	// 添加字体支持
	if err := g.addFonts(pdf); err != nil {
		log.Printf("警告：添加字体失败: %v", err)
	}

	// 设置默认字体
	pdf.SetFont("Arial", "", config.FontSize)

	// 生成页面
	if err := g.generateMonoPages(pdf, content, config); err != nil {
		return fmt.Errorf("生成页面失败: %w", err)
	}

	// 保存文件
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return fmt.Errorf("保存PDF文件失败: %w", err)
	}

	log.Printf("单语PDF生成完成: %s", outputPath)
	return nil
}

// addFonts 添加字体支持
func (g *PDFGenerator) addFonts(pdf *gofpdf.Fpdf) error {
	// 添加中文字体支持
	if g.FontPath != "" && fileExists(g.FontPath) {
		fontName := strings.TrimSuffix(filepath.Base(g.FontPath), filepath.Ext(g.FontPath))
		pdf.AddUTF8Font(fontName, "", g.FontPath)
		log.Printf("添加字体: %s", fontName)
	}

	return nil
}

// generatePages 生成双语页面
func (g *PDFGenerator) generatePages(pdf *gofpdf.Fpdf, originalContent, translatedContent *PDFContent, config BilingualPDFConfig) error {
	// 按页面组织文本块
	originalPages := g.groupBlocksByPage(originalContent.TextBlocks)
	translatedPages := g.groupBlocksByPage(translatedContent.TextBlocks)

	maxPages := len(originalPages)
	if len(translatedPages) > maxPages {
		maxPages = len(translatedPages)
	}

	for pageNum := 1; pageNum <= maxPages; pageNum++ {
		pdf.AddPage()

		// 添加页面标题
		pdf.SetFont("Arial", "B", config.FontSize+2)
		pdf.Cell(0, 10, fmt.Sprintf("Page %d", pageNum))
		pdf.Ln(15)

		// 原文部分
		if originalBlocks, exists := originalPages[pageNum]; exists && config.ShowOriginal {
			pdf.SetFont("Arial", "B", config.FontSize)
			pdf.Cell(0, 8, fmt.Sprintf("Original (%s):", config.SourceLang))
			pdf.Ln(10)

			pdf.SetFont("Arial", "", config.FontSize)
			g.renderTextBlocks(pdf, originalBlocks, config)
			pdf.Ln(10)
		}

		// 译文部分
		if translatedBlocks, exists := translatedPages[pageNum]; exists {
			pdf.SetFont("Arial", "B", config.FontSize)
			pdf.Cell(0, 8, fmt.Sprintf("Translation (%s):", config.TargetLang))
			pdf.Ln(10)

			// 使用中文字体（如果可用）
			if g.FontPath != "" {
				fontName := strings.TrimSuffix(filepath.Base(g.FontPath), filepath.Ext(g.FontPath))
				pdf.SetFont(fontName, "", config.FontSize)
			} else {
				pdf.SetFont("Arial", "", config.FontSize)
			}

			g.renderTextBlocks(pdf, translatedBlocks, config)
		}

		// 添加分页符（除了最后一页）
		if pageNum < maxPages {
			pdf.AddPage()
		}
	}

	return nil
}

// generateMonoPages 生成单语页面
func (g *PDFGenerator) generateMonoPages(pdf *gofpdf.Fpdf, content *PDFContent, config BilingualPDFConfig) error {
	// 按页面组织文本块
	pages := g.groupBlocksByPage(content.TextBlocks)

	for pageNum := 1; pageNum <= len(pages); pageNum++ {
		pdf.AddPage()

		if blocks, exists := pages[pageNum]; exists {
			// 使用适当的字体
			if g.FontPath != "" {
				fontName := strings.TrimSuffix(filepath.Base(g.FontPath), filepath.Ext(g.FontPath))
				pdf.SetFont(fontName, "", config.FontSize)
			} else {
				pdf.SetFont("Arial", "", config.FontSize)
			}

			g.renderTextBlocks(pdf, blocks, config)
		}
	}

	return nil
}

// groupBlocksByPage 按页面分组文本块
func (g *PDFGenerator) groupBlocksByPage(blocks []TextBlock) map[int][]TextBlock {
	pages := make(map[int][]TextBlock)

	for _, block := range blocks {
		if _, exists := pages[block.PageNum]; !exists {
			pages[block.PageNum] = make([]TextBlock, 0)
		}
		pages[block.PageNum] = append(pages[block.PageNum], block)
	}

	return pages
}

// renderTextBlocks 渲染文本块
func (g *PDFGenerator) renderTextBlocks(pdf *gofpdf.Fpdf, blocks []TextBlock, config BilingualPDFConfig) {
	for _, block := range blocks {
		// 处理长文本换行
		lines := g.wrapText(pdf, block.Text, 180) // 180mm 宽度

		for _, line := range lines {
			// 检查是否需要换页
			if pdf.GetY() > 250 { // 接近页面底部
				pdf.AddPage()
			}

			// 渲染文本行
			if block.IsFormula {
				pdf.SetFont("Arial", "I", config.FontSize-1) // 公式用斜体
				pdf.SetTextColor(0, 0, 128)                  // 蓝色
			} else {
				if g.FontPath != "" {
					fontName := strings.TrimSuffix(filepath.Base(g.FontPath), filepath.Ext(g.FontPath))
					pdf.SetFont(fontName, "", config.FontSize)
				} else {
					pdf.SetFont("Arial", "", config.FontSize)
				}
				pdf.SetTextColor(0, 0, 0) // 黑色
			}

			pdf.Cell(0, config.LineSpacing, line)
			pdf.Ln(config.LineSpacing)
		}

		// 段落间距
		pdf.Ln(2)
	}
}

// wrapText 文本换行
func (g *PDFGenerator) wrapText(pdf *gofpdf.Fpdf, text string, maxWidth float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	var lines []string
	var currentLine string

	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		// 检查行宽度
		width := pdf.GetStringWidth(testLine)
		if width <= maxWidth {
			currentLine = testLine
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
