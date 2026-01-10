package translator

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/math/fixed"
)

// FontMetricsCalculator 字体度量计算器
// 使用真实TTF字体文件精确计算文本宽度
type FontMetricsCalculator struct {
	fontCache  map[string]*truetype.Font
	widthCache map[string]float64
	mutex      sync.RWMutex
	fontPaths  map[string]string // 字体名称 -> 文件路径
}

// NewFontMetricsCalculator 创建字体度量计算器
func NewFontMetricsCalculator() *FontMetricsCalculator {
	fmc := &FontMetricsCalculator{
		fontCache:  make(map[string]*truetype.Font),
		widthCache: make(map[string]float64),
		fontPaths:  make(map[string]string),
	}
	
	// 初始化系统字体路径
	fmc.initSystemFonts()
	
	return fmc
}

// initSystemFonts 初始化系统字体路径
func (fmc *FontMetricsCalculator) initSystemFonts() {
	// 常见字体映射
	commonFonts := map[string][]string{
		"Arial": {
			"/System/Library/Fonts/Supplemental/Arial.ttf",                    // macOS
			"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf", // Linux
			"C:\\Windows\\Fonts\\arial.ttf",                                   // Windows
		},
		"Times": {
			"/System/Library/Fonts/Supplemental/Times New Roman.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationSerif-Regular.ttf",
			"C:\\Windows\\Fonts\\times.ttf",
		},
		"Helvetica": {
			"/System/Library/Fonts/Helvetica.ttc",
			"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
			"C:\\Windows\\Fonts\\arial.ttf",
		},
		"Courier": {
			"/System/Library/Fonts/Courier.dfont",
			"/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf",
			"C:\\Windows\\Fonts\\cour.ttf",
		},
		// 中文字体
		"SimHei": {
			"/System/Library/Fonts/STHeiti Light.ttc",
			"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
			"C:\\Windows\\Fonts\\simhei.ttf",
		},
		"SimSun": {
			"/System/Library/Fonts/Songti.ttc",
			"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
			"C:\\Windows\\Fonts\\simsun.ttc",
		},
	}
	
	// 查找存在的字体文件
	for fontName, paths := range commonFonts {
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				fmc.fontPaths[fontName] = path
				log.Printf("找到字体: %s -> %s", fontName, path)
				break
			}
		}
	}
	
	// 扫描系统字体目录
	fmc.scanFontDirectories()
}

// scanFontDirectories 扫描系统字体目录
func (fmc *FontMetricsCalculator) scanFontDirectories() {
	fontDirs := []string{
		"/System/Library/Fonts",                    // macOS
		"/Library/Fonts",                           // macOS
		"/usr/share/fonts",                         // Linux
		"/usr/local/share/fonts",                   // Linux
		"C:\\Windows\\Fonts",                       // Windows
		os.Getenv("HOME") + "/.fonts",              // User fonts
		os.Getenv("HOME") + "/Library/Fonts",       // macOS user fonts
	}
	
	for _, dir := range fontDirs {
		if _, err := os.Stat(dir); err == nil {
			fmc.scanDirectory(dir)
		}
	}
}

// scanDirectory 扫描目录中的字体文件
func (fmc *FontMetricsCalculator) scanDirectory(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if info.IsDir() {
			return nil
		}
		
		ext := filepath.Ext(path)
		if ext == ".ttf" || ext == ".ttc" || ext == ".otf" {
			// 提取字体名称（简化版）
			fontName := filepath.Base(path)
			fontName = fontName[:len(fontName)-len(ext)]
			
			// 只保存还没有的字体
			if _, exists := fmc.fontPaths[fontName]; !exists {
				fmc.fontPaths[fontName] = path
			}
		}
		
		return nil
	})
}

// CalculateTextWidth 计算文本宽度（精确版本）
func (fmc *FontMetricsCalculator) CalculateTextWidth(text string, fontName string, fontSize float64) float64 {
	if text == "" {
		return 0
	}
	
	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%.1f:%s", fontName, fontSize, text)
	fmc.mutex.RLock()
	if width, ok := fmc.widthCache[cacheKey]; ok {
		fmc.mutex.RUnlock()
		return width
	}
	fmc.mutex.RUnlock()
	
	// 加载字体
	ttfFont, err := fmc.loadFont(fontName)
	if err != nil {
		// 回退到估算
		return fmc.estimateTextWidth(text, fontSize)
	}
	
	// 计算精确宽度
	width := fmc.calculateWithFont(text, ttfFont, fontSize)
	
	// 缓存结果
	fmc.mutex.Lock()
	fmc.widthCache[cacheKey] = width
	fmc.mutex.Unlock()
	
	return width
}

// loadFont 加载字体
func (fmc *FontMetricsCalculator) loadFont(fontName string) (*truetype.Font, error) {
	// 检查缓存
	fmc.mutex.RLock()
	if ttfFont, ok := fmc.fontCache[fontName]; ok {
		fmc.mutex.RUnlock()
		return ttfFont, nil
	}
	fmc.mutex.RUnlock()
	
	// 查找字体路径
	fontPath, ok := fmc.fontPaths[fontName]
	if !ok {
		// 尝试模糊匹配
		fontPath = fmc.fuzzyMatchFont(fontName)
		if fontPath == "" {
			return nil, fmt.Errorf("字体未找到: %s", fontName)
		}
	}
	
	// 读取字体文件
	fontBytes, err := ioutil.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("读取字体文件失败: %w", err)
	}
	
	// 解析字体
	ttfFont, err := truetype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("解析字体失败: %w", err)
	}
	
	// 缓存字体
	fmc.mutex.Lock()
	fmc.fontCache[fontName] = ttfFont
	fmc.mutex.Unlock()
	
	return ttfFont, nil
}

// fuzzyMatchFont 模糊匹配字体
func (fmc *FontMetricsCalculator) fuzzyMatchFont(fontName string) string {
	fontNameLower := toLower(fontName)
	
	// 尝试匹配
	for name, path := range fmc.fontPaths {
		nameLower := toLower(name)
		if contains(nameLower, fontNameLower) || contains(fontNameLower, nameLower) {
			return path
		}
	}
	
	// 默认字体
	if path, ok := fmc.fontPaths["Arial"]; ok {
		return path
	}
	
	return ""
}

// calculateWithFont 使用字体计算宽度
func (fmc *FontMetricsCalculator) calculateWithFont(text string, ttfFont *truetype.Font, fontSize float64) float64 {
	// 创建字体面
	face := truetype.NewFace(ttfFont, &truetype.Options{
		Size: fontSize,
		DPI:  72, // PDF使用72 DPI
	})
	defer face.Close()
	
	// 计算总宽度
	totalWidth := fixed.Int26_6(0)
	prevIndex := truetype.Index(0)
	
	for _, r := range text {
		index := ttfFont.Index(r)
		if index != 0 {
			// 获取字符宽度
			advance, ok := face.GlyphAdvance(r)
			if ok {
				totalWidth += advance
			}
			
			// 添加字距调整（kerning）
			if prevIndex != 0 {
				kern := ttfFont.Kern(fixed.Int26_6(ttfFont.FUnitsPerEm()), prevIndex, index)
				totalWidth += kern
			}
			
			prevIndex = index
		}
	}
	
	// 转换为浮点数（点）
	return float64(totalWidth) / 64.0
}

// estimateTextWidth 估算文本宽度（回退方法）
func (fmc *FontMetricsCalculator) estimateTextWidth(text string, fontSize float64) float64 {
	width := 0.0
	
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			// 中文字符（等宽）
			width += fontSize
		} else if r == ' ' {
			// 空格
			width += fontSize * 0.25
		} else if r >= 'A' && r <= 'Z' {
			// 大写字母
			width += fontSize * 0.65
		} else if r >= 'a' && r <= 'z' {
			// 小写字母
			width += fontSize * 0.55
		} else if r >= '0' && r <= '9' {
			// 数字
			width += fontSize * 0.6
		} else {
			// 其他字符
			width += fontSize * 0.5
		}
	}
	
	return width
}

// CalculateTextHeight 计算文本高度
func (fmc *FontMetricsCalculator) CalculateTextHeight(fontName string, fontSize float64) float64 {
	ttfFont, err := fmc.loadFont(fontName)
	if err != nil {
		return fontSize * 1.2 // 默认行高
	}
	
	// 获取字体度量
	bounds := ttfFont.Bounds(fixed.Int26_6(ttfFont.FUnitsPerEm()))
	scale := fontSize / float64(ttfFont.FUnitsPerEm())
	
	ascent := float64(bounds.Max.Y) * scale
	descent := float64(-bounds.Min.Y) * scale
	
	return ascent + descent
}

// GetFontMetrics 获取字体度量信息
func (fmc *FontMetricsCalculator) GetFontMetrics(fontName string, fontSize float64) FontMetrics {
	ttfFont, err := fmc.loadFont(fontName)
	if err != nil {
		// 返回默认度量
		return FontMetrics{
			Ascent:     fontSize * 0.8,
			Descent:    fontSize * 0.2,
			LineHeight: fontSize * 1.2,
			CapHeight:  fontSize * 0.7,
			XHeight:    fontSize * 0.5,
		}
	}
	
	// 计算真实度量
	bounds := ttfFont.Bounds(fixed.Int26_6(ttfFont.FUnitsPerEm()))
	scale := fontSize / float64(ttfFont.FUnitsPerEm())
	
	return FontMetrics{
		Ascent:     float64(bounds.Max.Y) * scale,
		Descent:    float64(-bounds.Min.Y) * scale,
		LineHeight: fontSize * 1.2,
		CapHeight:  fontSize * 0.7, // 简化
		XHeight:    fontSize * 0.5, // 简化
	}
}

// ClearCache 清空缓存
func (fmc *FontMetricsCalculator) ClearCache() {
	fmc.mutex.Lock()
	defer fmc.mutex.Unlock()
	
	fmc.widthCache = make(map[string]float64)
}

// GetCacheSize 获取缓存大小
func (fmc *FontMetricsCalculator) GetCacheSize() int {
	fmc.mutex.RLock()
	defer fmc.mutex.RUnlock()
	
	return len(fmc.widthCache)
}

// GetLoadedFonts 获取已加载的字体列表
func (fmc *FontMetricsCalculator) GetLoadedFonts() []string {
	fmc.mutex.RLock()
	defer fmc.mutex.RUnlock()
	
	fonts := make([]string, 0, len(fmc.fontCache))
	for name := range fmc.fontCache {
		fonts = append(fonts, name)
	}
	
	return fonts
}

// GetAvailableFonts 获取可用的字体列表
func (fmc *FontMetricsCalculator) GetAvailableFonts() []string {
	fmc.mutex.RLock()
	defer fmc.mutex.RUnlock()
	
	fonts := make([]string, 0, len(fmc.fontPaths))
	for name := range fmc.fontPaths {
		fonts = append(fonts, name)
	}
	
	return fonts
}

// 辅助函数
func toLower(s string) string {
	result := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result += string(r + 32)
		} else {
			result += string(r)
		}
	}
	return result
}

func contains(s, substr string) bool {
	return indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// WrapText 文本换行（使用精确宽度）
func (fmc *FontMetricsCalculator) WrapText(text string, fontName string, fontSize float64, maxWidth float64) []string {
	if text == "" {
		return []string{}
	}
	
	words := splitWords(text)
	lines := []string{}
	currentLine := ""
	
	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word
		
		width := fmc.CalculateTextWidth(testLine, fontName, fontSize)
		
		if width > maxWidth && currentLine != "" {
			// 当前行已满，开始新行
			lines = append(lines, currentLine)
			currentLine = word
		} else {
			currentLine = testLine
		}
	}
	
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	
	return lines
}

// splitWords 分割单词（支持中英文）
func splitWords(text string) []string {
	words := []string{}
	currentWord := ""
	
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' {
			if currentWord != "" {
				words = append(words, currentWord)
				currentWord = ""
			}
		} else if r >= 0x4e00 && r <= 0x9fff {
			// 中文字符，每个字符作为一个单词
			if currentWord != "" {
				words = append(words, currentWord)
				currentWord = ""
			}
			words = append(words, string(r))
		} else {
			currentWord += string(r)
		}
	}
	
	if currentWord != "" {
		words = append(words, currentWord)
	}
	
	return words
}

// 全局字体度量计算器实例
var globalFontMetrics *FontMetricsCalculator
var fontMetricsOnce sync.Once

// GetGlobalFontMetrics 获取全局字体度量计算器
func GetGlobalFontMetrics() *FontMetricsCalculator {
	fontMetricsOnce.Do(func() {
		globalFontMetrics = NewFontMetricsCalculator()
	})
	return globalFontMetrics
}
