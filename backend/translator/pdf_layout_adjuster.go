package translator

import (
	"fmt"
	"log"
	"math"
)

// LayoutAdjuster 布局调整器
// 实现pdf2zh的动态行距调整技术
type LayoutAdjuster struct {
	fontMetrics      *FontMetricsCalculator
	baseLineSpacing  map[string]float64 // 语言 -> 基础行距系数
	minLineSpacing   float64            // 最小行距系数
	maxIterations    int                // 最大调整迭代次数
	overflowStrategy string             // 溢出策略: "shrink", "wrap", "truncate"
}

// NewLayoutAdjuster 创建布局调整器
func NewLayoutAdjuster() *LayoutAdjuster {
	return &LayoutAdjuster{
		fontMetrics: GetGlobalFontMetrics(),
		baseLineSpacing: map[string]float64{
			"zh":      1.4,  // 中文
			"ja":      1.4,  // 日文
			"ko":      1.4,  // 韩文
			"en":      1.2,  // 英文
			"default": 1.3,  // 默认
		},
		minLineSpacing:   1.0,
		maxIterations:    10,
		overflowStrategy: "shrink", // 默认策略：缩小行距
	}
}

// AdjustTextLayout 调整文本布局
func (la *LayoutAdjuster) AdjustTextLayout(
	originalBox BoundingBox,
	originalText string,
	translatedText string,
	font FontFlow,
	targetLang string,
) (*AdjustedLayout, error) {
	
	log.Printf("调整文本布局: 原文=%s, 译文=%s", 
		truncateForLog(originalText, 30), 
		truncateForLog(translatedText, 30))
	
	// 1. 计算原文和译文的尺寸
	originalWidth := la.fontMetrics.CalculateTextWidth(originalText, font.Name, font.Size)
	translatedWidth := la.fontMetrics.CalculateTextWidth(translatedText, font.Name, font.Size)
	
	log.Printf("宽度对比: 原文=%.2f, 译文=%.2f, 容器=%.2f", 
		originalWidth, translatedWidth, originalBox.Width)
	
	// 2. 检查是否需要调整
	if translatedWidth <= originalBox.Width {
		// 单行即可容纳
		return &AdjustedLayout{
			Text:        translatedText,
			Lines:       []string{translatedText},
			BoundingBox: originalBox,
			FontSize:    font.Size,
			LineSpacing: la.getBaseLineSpacing(targetLang),
			Adjusted:    false,
		}, nil
	}
	
	// 3. 需要调整，根据策略处理
	switch la.overflowStrategy {
	case "wrap":
		return la.adjustWithWrapping(originalBox, translatedText, font, targetLang)
	case "shrink":
		return la.adjustWithShrinking(originalBox, translatedText, font, targetLang)
	case "truncate":
		return la.adjustWithTruncation(originalBox, translatedText, font, targetLang)
	default:
		return la.adjustWithWrapping(originalBox, translatedText, font, targetLang)
	}
}

// adjustWithWrapping 使用换行调整
func (la *LayoutAdjuster) adjustWithWrapping(
	originalBox BoundingBox,
	text string,
	font FontFlow,
	targetLang string,
) (*AdjustedLayout, error) {
	
	baseSpacing := la.getBaseLineSpacing(targetLang)
	currentSpacing := baseSpacing
	
	for i := 0; i < la.maxIterations; i++ {
		// 换行
		lines := la.fontMetrics.WrapText(text, font.Name, font.Size, originalBox.Width)
		
		// 计算实际高度
		actualHeight := float64(len(lines)) * font.Size * currentSpacing
		
		log.Printf("换行尝试 %d: 行数=%d, 行距=%.2f, 高度=%.2f/%.2f", 
			i+1, len(lines), currentSpacing, actualHeight, originalBox.Height)
		
		if actualHeight <= originalBox.Height {
			// 成功容纳
			return &AdjustedLayout{
				Text:        text,
				Lines:       lines,
				BoundingBox: originalBox,
				FontSize:    font.Size,
				LineSpacing: currentSpacing,
				Adjusted:    true,
				Method:      "wrap",
			}, nil
		}
		
		// 减小行距
		currentSpacing -= 0.05
		if currentSpacing < la.minLineSpacing {
			break
		}
	}
	
	// 无法完全容纳，返回最佳尝试
	lines := la.fontMetrics.WrapText(text, font.Name, font.Size, originalBox.Width)
	
	return &AdjustedLayout{
		Text:        text,
		Lines:       lines,
		BoundingBox: originalBox,
		FontSize:    font.Size,
		LineSpacing: la.minLineSpacing,
		Adjusted:    true,
		Method:      "wrap",
		Overflow:    true,
	}, nil
}

// adjustWithShrinking 使用缩小字体调整
func (la *LayoutAdjuster) adjustWithShrinking(
	originalBox BoundingBox,
	text string,
	font FontFlow,
	targetLang string,
) (*AdjustedLayout, error) {
	
	baseSpacing := la.getBaseLineSpacing(targetLang)
	currentFontSize := font.Size
	minFontSize := font.Size * 0.7 // 最小缩小到70%
	
	for i := 0; i < la.maxIterations; i++ {
		// 换行
		lines := la.fontMetrics.WrapText(text, font.Name, currentFontSize, originalBox.Width)
		
		// 计算实际高度
		actualHeight := float64(len(lines)) * currentFontSize * baseSpacing
		
		log.Printf("缩小尝试 %d: 字号=%.2f, 行数=%d, 高度=%.2f/%.2f", 
			i+1, currentFontSize, len(lines), actualHeight, originalBox.Height)
		
		if actualHeight <= originalBox.Height {
			// 成功容纳
			return &AdjustedLayout{
				Text:        text,
				Lines:       lines,
				BoundingBox: originalBox,
				FontSize:    currentFontSize,
				LineSpacing: baseSpacing,
				Adjusted:    true,
				Method:      "shrink",
			}, nil
		}
		
		// 减小字号
		currentFontSize -= 0.5
		if currentFontSize < minFontSize {
			break
		}
	}
	
	// 无法完全容纳，返回最小字号
	lines := la.fontMetrics.WrapText(text, font.Name, minFontSize, originalBox.Width)
	
	return &AdjustedLayout{
		Text:        text,
		Lines:       lines,
		BoundingBox: originalBox,
		FontSize:    minFontSize,
		LineSpacing: baseSpacing,
		Adjusted:    true,
		Method:      "shrink",
		Overflow:    true,
	}, nil
}

// adjustWithTruncation 使用截断调整
func (la *LayoutAdjuster) adjustWithTruncation(
	originalBox BoundingBox,
	text string,
	font FontFlow,
	targetLang string,
) (*AdjustedLayout, error) {
	
	baseSpacing := la.getBaseLineSpacing(targetLang)
	
	// 计算可容纳的行数
	maxLines := int(originalBox.Height / (font.Size * baseSpacing))
	if maxLines < 1 {
		maxLines = 1
	}
	
	// 换行
	lines := la.fontMetrics.WrapText(text, font.Name, font.Size, originalBox.Width)
	
	// 截断
	truncated := false
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		// 在最后一行添加省略号
		if len(lines) > 0 {
			lines[len(lines)-1] += "..."
		}
		truncated = true
	}
	
	return &AdjustedLayout{
		Text:        text,
		Lines:       lines,
		BoundingBox: originalBox,
		FontSize:    font.Size,
		LineSpacing: baseSpacing,
		Adjusted:    truncated,
		Method:      "truncate",
		Overflow:    false,
	}, nil
}

// getBaseLineSpacing 获取基础行距
func (la *LayoutAdjuster) getBaseLineSpacing(lang string) float64 {
	if spacing, ok := la.baseLineSpacing[lang]; ok {
		return spacing
	}
	return la.baseLineSpacing["default"]
}

// AdjustedLayout 调整后的布局
type AdjustedLayout struct {
	Text        string      // 文本内容
	Lines       []string    // 分行后的文本
	BoundingBox BoundingBox // 边界框
	FontSize    float64     // 字体大小
	LineSpacing float64     // 行距系数
	Adjusted    bool        // 是否进行了调整
	Method      string      // 调整方法: "wrap", "shrink", "truncate"
	Overflow    bool        // 是否仍然溢出
}

// CalculateActualHeight 计算实际高度
func (al *AdjustedLayout) CalculateActualHeight() float64 {
	return float64(len(al.Lines)) * al.FontSize * al.LineSpacing
}

// GetLinePositions 获取每行的Y坐标
func (al *AdjustedLayout) GetLinePositions() []float64 {
	positions := make([]float64, len(al.Lines))
	currentY := al.BoundingBox.Y
	
	for i := range al.Lines {
		positions[i] = currentY
		currentY += al.FontSize * al.LineSpacing
	}
	
	return positions
}

// BatchAdjustLayout 批量调整布局
func (la *LayoutAdjuster) BatchAdjustLayout(
	elements []TextElementFlow,
	translations map[string]string,
	targetLang string,
) []AdjustedLayout {
	
	results := make([]AdjustedLayout, 0, len(elements))
	
	for _, elem := range elements {
		// 查找翻译
		translated, ok := translations[elem.Content]
		if !ok {
			translated = elem.Content
		}
		
		// 调整布局
		adjusted, err := la.AdjustTextLayout(
			elem.BoundingBox,
			elem.Content,
			translated,
			elem.Font,
			targetLang,
		)
		
		if err != nil {
			log.Printf("布局调整失败: %v", err)
			// 使用原始布局
			adjusted = &AdjustedLayout{
				Text:        translated,
				Lines:       []string{translated},
				BoundingBox: elem.BoundingBox,
				FontSize:    elem.Font.Size,
				LineSpacing: la.getBaseLineSpacing(targetLang),
				Adjusted:    false,
			}
		}
		
		results = append(results, *adjusted)
	}
	
	return results
}

// OptimizePageLayout 优化整页布局
func (la *LayoutAdjuster) OptimizePageLayout(
	page *PDFPageFlow,
	translations map[string]string,
	targetLang string,
) error {
	
	log.Printf("优化页面布局: 页码=%d, 元素数=%d", page.PageNumber, len(page.TextElements))
	
	adjustedCount := 0
	overflowCount := 0
	
	for i := range page.TextElements {
		elem := &page.TextElements[i]
		
		// 跳过公式
		if elem.IsFormula {
			continue
		}
		
		// 查找翻译
		translated, ok := translations[elem.Content]
		if !ok {
			continue
		}
		
		// 调整布局
		adjusted, err := la.AdjustTextLayout(
			elem.BoundingBox,
			elem.Content,
			translated,
			elem.Font,
			targetLang,
		)
		
		if err != nil {
			log.Printf("元素 %s 布局调整失败: %v", elem.ID, err)
			continue
		}
		
		// 更新元素
		elem.Content = adjusted.Text
		elem.Font.Size = adjusted.FontSize
		elem.BoundingBox = adjusted.BoundingBox
		
		// 存储分行信息（如果需要）
		if len(adjusted.Lines) > 1 {
			// 可以在这里存储多行信息
			// 暂时简化处理，将多行合并
			elem.Content = joinLines(adjusted.Lines)
		}
		
		if adjusted.Adjusted {
			adjustedCount++
		}
		if adjusted.Overflow {
			overflowCount++
			log.Printf("警告: 元素 %s 仍然溢出", elem.ID)
		}
	}
	
	log.Printf("页面布局优化完成: 调整=%d, 溢出=%d", adjustedCount, overflowCount)
	
	return nil
}

// AnalyzeLayoutComplexity 分析布局复杂度
func (la *LayoutAdjuster) AnalyzeLayoutComplexity(page *PDFPageFlow) *LayoutComplexity {
	complexity := &LayoutComplexity{
		PageNumber:     page.PageNumber,
		TotalElements:  len(page.TextElements),
		FontSizes:      make(map[float64]int),
		FontFamilies:   make(map[string]int),
		TextDensity:    0,
	}
	
	// 统计字体
	totalChars := 0
	for _, elem := range page.TextElements {
		complexity.FontSizes[elem.Font.Size]++
		complexity.FontFamilies[elem.Font.Name]++
		totalChars += len(elem.Content)
	}
	
	// 计算文本密度
	pageArea := page.MediaBox.Width * page.MediaBox.Height
	if pageArea > 0 {
		complexity.TextDensity = float64(totalChars) / pageArea
	}
	
	// 判断是否为多栏
	complexity.IsMultiColumn = la.detectMultiColumn(page)
	
	// 判断复杂度等级
	if len(complexity.FontSizes) > 5 || complexity.IsMultiColumn {
		complexity.Level = "complex"
	} else if len(complexity.FontSizes) > 3 {
		complexity.Level = "medium"
	} else {
		complexity.Level = "simple"
	}
	
	return complexity
}

// detectMultiColumn 检测是否为多栏布局
func (la *LayoutAdjuster) detectMultiColumn(page *PDFPageFlow) bool {
	if len(page.TextElements) < 10 {
		return false
	}
	
	// 统计X坐标分布
	xPositions := make(map[int]int)
	for _, elem := range page.TextElements {
		// 10pt精度
		x := int(elem.Position.X / 10) * 10
		xPositions[x]++
	}
	
	// 查找明显的聚类
	clusters := 0
	inCluster := false
	
	for x := 0; x < int(page.MediaBox.Width); x += 10 {
		count := xPositions[x]
		
		if count > 3 && !inCluster {
			clusters++
			inCluster = true
		} else if count < 2 && inCluster {
			inCluster = false
		}
	}
	
	return clusters >= 2
}

// LayoutComplexity 布局复杂度
type LayoutComplexity struct {
	PageNumber     int
	TotalElements  int
	FontSizes      map[float64]int
	FontFamilies   map[string]int
	TextDensity    float64
	IsMultiColumn  bool
	Level          string // "simple", "medium", "complex"
}

// GetStatistics 获取统计信息
func (lc *LayoutComplexity) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"page_number":     lc.PageNumber,
		"total_elements":  lc.TotalElements,
		"font_sizes":      len(lc.FontSizes),
		"font_families":   len(lc.FontFamilies),
		"text_density":    fmt.Sprintf("%.4f", lc.TextDensity),
		"is_multi_column": lc.IsMultiColumn,
		"level":           lc.Level,
	}
}

// 辅助函数
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// SetOverflowStrategy 设置溢出策略
func (la *LayoutAdjuster) SetOverflowStrategy(strategy string) {
	la.overflowStrategy = strategy
}

// SetMinLineSpacing 设置最小行距
func (la *LayoutAdjuster) SetMinLineSpacing(spacing float64) {
	la.minLineSpacing = spacing
}

// SetBaseLineSpacing 设置基础行距
func (la *LayoutAdjuster) SetBaseLineSpacing(lang string, spacing float64) {
	la.baseLineSpacing[lang] = spacing
}

// CalculateOptimalFontSize 计算最优字体大小
func (la *LayoutAdjuster) CalculateOptimalFontSize(
	text string,
	fontName string,
	originalSize float64,
	maxWidth float64,
	maxHeight float64,
) float64 {
	
	// 二分查找最优字体大小
	minSize := originalSize * 0.5
	maxSize := originalSize
	tolerance := 0.1
	
	for maxSize-minSize > tolerance {
		midSize := (minSize + maxSize) / 2
		
		// 计算该字号下的尺寸
		width := la.fontMetrics.CalculateTextWidth(text, fontName, midSize)
		
		if width <= maxWidth {
			minSize = midSize
		} else {
			maxSize = midSize
		}
	}
	
	return math.Floor(minSize*10) / 10 // 保留一位小数
}
