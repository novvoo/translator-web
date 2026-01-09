package pdf

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/signintech/gopdf"
)

// PDFBuilder PDF构建器，提供便捷的PDF创建功能
type PDFBuilder struct {
	pdf        *gopdf.GoPdf
	fontHelper *UniFontHelper
	config     PDFConfig
}

// PDFConfig PDF配置
type PDFConfig struct {
	PageSize    *gopdf.Rect
	Unit        string
	DefaultFont string
	DefaultSize float64
	Margins     Margins
	AutoUni     bool // 是否自动启用Unicode支持（多语言）
}

// Margins 页边距
type Margins struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// DefaultPDFConfig 默认PDF配置
func DefaultPDFConfig() PDFConfig {
	return PDFConfig{
		PageSize:    gopdf.PageSizeA4,
		Unit:        "mm",
		DefaultFont: "Arial",
		DefaultSize: 12,
		Margins: Margins{
			Top:    20,
			Right:  20,
			Bottom: 20,
			Left:   20,
		},
		AutoUni: true,
	}
}

// NewPDFBuilder 创建新的PDF构建器
func NewPDFBuilder(config ...PDFConfig) *PDFBuilder {
	var cfg PDFConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultPDFConfig()
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{
		PageSize: *cfg.PageSize,
	})

	builder := &PDFBuilder{
		pdf:    pdf,
		config: cfg,
	}

	// 如果启用自动Unicode支持，创建字体助手
	if cfg.AutoUni {
		builder.fontHelper = NewUniFontHelper(pdf)
	}

	return builder
}

// AddPage 添加新页面
func (pb *PDFBuilder) AddPage() *PDFBuilder {
	pb.pdf.AddPage()
	return pb
}

// SetFont 设置字体
func (pb *PDFBuilder) SetFont(family string, size float64) *PDFBuilder {
	if err := pb.pdf.SetFont(family, "", size); err != nil {
		log.Printf("设置字体失败: %v", err)
	}
	return pb
}

// SetUniFont 设置通用字体
func (pb *PDFBuilder) SetUniFont(size float64) *PDFBuilder {
	if pb.fontHelper == nil {
		pb.fontHelper = NewUniFontHelper(pb.pdf)
	}

	if err := pb.fontHelper.SetUniFont(size); err != nil {
		log.Printf("设置通用字体失败: %v", err)
		// 回退到默认字体
		pb.SetFont(pb.config.DefaultFont, size)
	}
	return pb
}

// WriteText 写入文本
func (pb *PDFBuilder) WriteText(x, y float64, text string) *PDFBuilder {
	pb.pdf.SetXY(x, y)
	rect := &gopdf.Rect{W: 200, H: 10}
	if err := pb.pdf.Cell(rect, text); err != nil {
		log.Printf("写入文本失败: %v", err)
	}
	return pb
}

// WriteUniText 写入通用文本（保持向后兼容）
func (pb *PDFBuilder) WriteUniText(x, y float64, text string) *PDFBuilder {
	return pb.WriteUnicodeText(x, y, text)
}

// WriteUnicodeText 写入Unicode文本（支持多语言）
func (pb *PDFBuilder) WriteUnicodeText(x, y float64, text string) *PDFBuilder {
	if pb.fontHelper == nil || !pb.fontHelper.IsUniFontLoaded() {
		// 如果通用字体未加载，尝试加载
		if pb.fontHelper == nil {
			pb.fontHelper = NewUniFontHelper(pb.pdf)
		}
		if err := pb.fontHelper.EnsureUniFontLoaded(); err != nil {
			log.Printf("加载通用字体失败，使用默认字体: %v", err)
			return pb.WriteText(x, y, text)
		}
	}

	if err := pb.fontHelper.WriteUnicodeText(x, y, text); err != nil {
		log.Printf("写入Unicode文本失败: %v", err)
	}
	return pb
}

// WriteTitle 写入标题
func (pb *PDFBuilder) WriteTitle(x, y float64, title string, size float64) *PDFBuilder {
	if pb.config.AutoUni {
		pb.SetUniFont(size)
		return pb.WriteUnicodeText(x, y, title)
	} else {
		pb.SetFont(pb.config.DefaultFont, size)
		return pb.WriteText(x, y, title)
	}
}

// WriteContent 写入正文内容
func (pb *PDFBuilder) WriteContent(x, y float64, content string) *PDFBuilder {
	if pb.config.AutoUni {
		pb.SetUniFont(pb.config.DefaultSize)
		return pb.WriteUnicodeText(x, y, content)
	} else {
		pb.SetFont(pb.config.DefaultFont, pb.config.DefaultSize)
		return pb.WriteText(x, y, content)
	}
}

// WriteMultilineContent 写入多行内容
func (pb *PDFBuilder) WriteMultilineContent(x, y float64, content string, lineHeight float64) *PDFBuilder {
	lines := splitLines(content)
	currentY := y

	for _, line := range lines {
		if line == "" {
			currentY += lineHeight / 2 // 空行间距小一些
		} else {
			pb.WriteContent(x, currentY, line)
			currentY += lineHeight
		}
	}

	return pb
}

// AddLine 添加分割线
func (pb *PDFBuilder) AddLine(x1, y1, x2, y2 float64) *PDFBuilder {
	pb.pdf.Line(x1, y1, x2, y2)
	return pb
}

// AddRectangle 添加矩形
func (pb *PDFBuilder) AddRectangle(x, y, w, h float64) *PDFBuilder {
	pb.pdf.Rectangle(x, y, w, h, "D", 0, 0)
	return pb
}

// SaveToFile 保存到文件
func (pb *PDFBuilder) SaveToFile(filename string) error {
	// 确保目录存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	return pb.pdf.WritePdf(filename)
}

// GetPDF 获取底层的GoPdf实例
func (pb *PDFBuilder) GetPDF() *gopdf.GoPdf {
	return pb.pdf
}

// GetFontHelper 获取通用字体助手
func (pb *PDFBuilder) GetFontHelper() *UniFontHelper {
	return pb.fontHelper
}

// splitLines 分割文本为行
func splitLines(text string) []string {
	return strings.Split(text, "\n")
}

// CreateSimpleUniPDF 创建简单的多语言PDF文档
func CreateSimpleUniPDF(title, content, filename string) error {
	builder := NewPDFBuilder()

	builder.AddPage().
		WriteTitle(20, 30, title, 18).
		WriteMultilineContent(20, 60, content, 15)

	return builder.SaveToFile(filename)
}

// CreateBilingualPDF 创建多语言PDF文档
func CreateBilingualPDF(UniTitle, englishTitle, UniContent, englishContent, filename string) error {
	builder := NewPDFBuilder()

	builder.AddPage()

	// 通用标题
	builder.SetUniFont(18).
		WriteUnicodeText(20, 30, UniTitle)

	// 英文标题
	builder.SetFont("Arial", 16).
		WriteText(20, 50, englishTitle)

	// 通用内容
	builder.SetUniFont(12).
		WriteMultilineContent(20, 80, UniContent, 15)

	// 计算英文内容的起始位置
	UniLines := len(splitLines(UniContent))
	englishStartY := 80 + float64(UniLines)*15 + 20

	// 英文内容
	builder.SetFont("Arial", 12).
		WriteMultilineContent(20, englishStartY, englishContent, 15)

	return builder.SaveToFile(filename)
}
