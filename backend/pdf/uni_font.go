package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/signintech/gopdf"
)

// UniFontManager 通用字体管理器（支持多语言）
type UniFontManager struct {
	pdf         *gopdf.GoPdf
	loadedFonts map[string]FontInfo
	currentFont string
}

// FontInfo 字体信息
type FontInfo struct {
	Family      string  // 字体族名
	Name        string  // 显示名称
	Filename    string  // 文件名
	Path        string  // 完整路径
	Size        float64 // 当前大小
	IsLoaded    bool    // 是否已加载
	Description string  // 描述
}

// NewUniFontManager 创建新的通用字体管理器
func NewUniFontManager(pdf *gopdf.GoPdf) *UniFontManager {
	return &UniFontManager{
		pdf:         pdf,
		loadedFonts: make(map[string]FontInfo),
	}
}

// GetSystemUniFonts 获取系统通用字体列表
func (cfm *UniFontManager) GetSystemUniFonts() []FontInfo {
	fonts := []FontInfo{
		// 通用字体
		{"SimHei", "黑体", "simhei.ttf", "", 12, false, "简体通用黑体，适合标题和正文"},
		{"MicrosoftYaHei", "微软雅黑", "msyh.ttc", "", 12, false, "现代化通用字体，清晰易读"},
		{"SimSun", "宋体", "simsun.ttc", "", 12, false, "传统通用宋体，适合正文阅读"},
		{"KaiTi", "楷体", "simkai.ttf", "", 12, false, "通用楷体，具有书法风格"},
		{"FangSong", "仿宋", "simfang.ttf", "", 12, false, "仿宋体，介于宋体和楷体之间"},
		{"NSimSun", "新宋体", "nsimsun.ttf", "", 12, false, "新宋体，宋体的改进版本"},

		// 英文字体
		{"Arial", "Arial", "arial.ttf", "", 12, false, "标准英文字体，清晰易读"},
		{"TimesNewRoman", "Times New Roman", "times.ttf", "", 12, false, "经典英文衬线字体"},
		{"Calibri", "Calibri", "calibri.ttf", "", 12, false, "现代英文字体，Office默认字体"},
		{"Verdana", "Verdana", "verdana.ttf", "", 12, false, "网页友好的英文字体"},

		// 日文字体
		{"MSGothic", "MS Gothic", "msgothic.ttc", "", 12, false, "日文哥特体"},
		{"MSMincho", "MS Mincho", "msmincho.ttc", "", 12, false, "日文明朝体"},

		// 韩文字体
		{"Gulim", "굴림", "gulim.ttc", "", 12, false, "韩文굴림字体"},
		{"Batang", "바탕", "batang.ttc", "", 12, false, "韩文바탕字体"},
	}

	fontsDir := getSystemFontsDir()
	if fontsDir == "" {
		return []FontInfo{} // 如果无法获取字体目录，返回空列表
	}

	availableFonts := make([]FontInfo, 0)
	for _, font := range fonts {
		fontPath := filepath.Join(fontsDir, font.Filename)
		if _, err := os.Stat(fontPath); err == nil {
			font.Path = fontPath
			availableFonts = append(availableFonts, font)
		}
	}

	return availableFonts
}

// getSystemFontsDir 获取系统字体目录
func getSystemFontsDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("WINDIR"), "Fonts")
	case "darwin": // macOS
		return "/System/Library/Fonts"
	case "linux":
		// Linux 可能有多个字体目录，优先检查常见的
		dirs := []string{
			"/usr/share/fonts",
			"/usr/local/share/fonts",
			"/System/Library/Fonts", // 某些Linux发行版
		}
		for _, dir := range dirs {
			if _, err := os.Stat(dir); err == nil {
				return dir
			}
		}
		return "/usr/share/fonts" // 默认返回
	default:
		return ""
	}
}

// LoadFont 加载字体
func (cfm *UniFontManager) LoadFont(family string) error {
	// 检查是否已经加载
	if fontInfo, exists := cfm.loadedFonts[family]; exists && fontInfo.IsLoaded {
		return nil
	}

	// 获取系统字体列表
	systemFonts := cfm.GetSystemUniFonts()

	var targetFont *FontInfo
	for _, font := range systemFonts {
		if font.Family == family {
			targetFont = &font
			break
		}
	}

	if targetFont == nil {
		return fmt.Errorf("未找到字体: %s", family)
	}

	// 加载字体到PDF
	if err := cfm.pdf.AddTTFFont(targetFont.Family, targetFont.Path); err != nil {
		return fmt.Errorf("加载字体失败: %w", err)
	}

	// 标记为已加载
	targetFont.IsLoaded = true
	cfm.loadedFonts[family] = *targetFont

	return nil
}

// SetFont 设置当前字体
func (cfm *UniFontManager) SetFont(family string, size float64) error {
	// 确保字体已加载
	if err := cfm.LoadFont(family); err != nil {
		return err
	}

	// 设置字体
	if err := cfm.pdf.SetFont(family, "", size); err != nil {
		return fmt.Errorf("设置字体失败: %w", err)
	}

	// 更新当前字体信息
	cfm.currentFont = family
	if fontInfo, exists := cfm.loadedFonts[family]; exists {
		fontInfo.Size = size
		cfm.loadedFonts[family] = fontInfo
	}

	return nil
}

// WriteText 写入文本
func (cfm *UniFontManager) WriteText(x, y float64, text string) error {
	cfm.pdf.SetXY(x, y)
	rect := &gopdf.Rect{W: 200, H: 10}
	return cfm.pdf.Cell(rect, text)
}

// WriteMultilineText 写入多行文本
func (cfm *UniFontManager) WriteMultilineText(x, y float64, text string, lineHeight float64) error {
	lines := strings.Split(text, "\n")
	currentY := y

	for _, line := range lines {
		if err := cfm.WriteText(x, currentY, line); err != nil {
			return err
		}
		currentY += lineHeight
	}

	return nil
}

// AutoLoadBestFont 自动加载最佳可用字体
func (cfm *UniFontManager) AutoLoadBestFont() (string, error) {
	// 按优先级尝试加载字体（支持多语言）
	priorities := []string{
		// 通用字体优先
		"MicrosoftYaHei", "SimHei", "SimSun",
		// 英文字体
		"Arial", "Calibri", "TimesNewRoman",
		// 日文字体
		"MSGothic", "MSMincho",
		// 韩文字体
		"Gulim", "Batang",
		// 其他通用字体
		"KaiTi", "FangSong",
	}

	for _, family := range priorities {
		if err := cfm.LoadFont(family); err == nil {
			return family, nil
		}
	}

	return "", fmt.Errorf("没有找到可用的通用字体")
}

// GetCurrentFont 获取当前字体信息
func (cfm *UniFontManager) GetCurrentFont() (FontInfo, bool) {
	if cfm.currentFont == "" {
		return FontInfo{}, false
	}

	fontInfo, exists := cfm.loadedFonts[cfm.currentFont]
	return fontInfo, exists
}

// GetLoadedFonts 获取已加载的字体列表
func (cfm *UniFontManager) GetLoadedFonts() []FontInfo {
	fonts := make([]FontInfo, 0, len(cfm.loadedFonts))
	for _, font := range cfm.loadedFonts {
		if font.IsLoaded {
			fonts = append(fonts, font)
		}
	}
	return fonts
}

// UniFontHelper 简化版通用字体助手，便于快速集成（支持多语言）
type UniFontHelper struct {
	pdf        *gopdf.GoPdf
	fontLoaded bool
	fontFamily string
}

// NewUniFontHelper 创建通用字体助手
func NewUniFontHelper(pdf *gopdf.GoPdf) *UniFontHelper {
	return &UniFontHelper{
		pdf: pdf,
	}
}

// EnsureUniFontLoaded 确保通用字体已加载
func (cfh *UniFontHelper) EnsureUniFontLoaded() error {
	if cfh.fontLoaded {
		return nil
	}

	// 尝试加载系统通用字体（支持多语言）
	fonts := []struct {
		family   string
		filename string
	}{
		// 通用字体优先
		{"MicrosoftYaHei", "msyh.ttc"},
		{"SimHei", "simhei.ttf"},
		{"SimSun", "simsun.ttc"},
		// 英文字体
		{"Arial", "arial.ttf"},
		{"Calibri", "calibri.ttf"},
		{"TimesNewRoman", "times.ttf"},
		// 日文字体
		{"MSGothic", "msgothic.ttc"},
		{"MSMincho", "msmincho.ttc"},
		// 韩文字体
		{"Gulim", "gulim.ttc"},
		{"Batang", "batang.ttc"},
	}

	fontsDir := getSystemFontsDir()
	if fontsDir == "" {
		return fmt.Errorf("无法获取系统字体目录")
	}

	for _, font := range fonts {
		fontPath := filepath.Join(fontsDir, font.filename)
		if _, err := os.Stat(fontPath); err == nil {
			if err := cfh.pdf.AddTTFFont(font.family, fontPath); err == nil {
				cfh.fontFamily = font.family
				cfh.fontLoaded = true
				return nil
			}
		}
	}

	return fmt.Errorf("没有找到可用的通用字体")
}

// SetUniFont 设置通用字体
func (cfh *UniFontHelper) SetUniFont(size float64) error {
	if err := cfh.EnsureUniFontLoaded(); err != nil {
		return err
	}
	return cfh.pdf.SetFont(cfh.fontFamily, "", size)
}

// WriteUnicodeText 写入Unicode文本（支持多语言）
func (cfh *UniFontHelper) WriteUnicodeText(x, y float64, text string) error {
	cfh.pdf.SetXY(x, y)
	rect := &gopdf.Rect{W: 200, H: 10}
	return cfh.pdf.Cell(rect, text)
}

// WriteUniText 写入通用文本（保持向后兼容）
func (cfh *UniFontHelper) WriteUniText(x, y float64, text string) error {
	return cfh.WriteUnicodeText(x, y, text)
}

// GetFontFamily 获取当前使用的字体族
func (cfh *UniFontHelper) GetFontFamily() string {
	return cfh.fontFamily
}

// IsUniFontLoaded 检查通用字体是否已加载
func (cfh *UniFontHelper) IsUniFontLoaded() bool {
	return cfh.fontLoaded
}
