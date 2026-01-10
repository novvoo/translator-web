package translator

import (
	"fmt"
	"log"
	"math"
	"sort"
)

// ColumnDetector 多栏布局检测器
type ColumnDetector struct {
	minColumnWidth   float64 // 最小列宽
	minGapWidth      float64 // 最小间隙宽度
	densityThreshold int     // 密度阈值
	precision        float64 // 精度（用于直方图）
}

// NewColumnDetector 创建多栏布局检测器
func NewColumnDetector() *ColumnDetector {
	return &ColumnDetector{
		minColumnWidth:   100.0, // 100pt
		minGapWidth:      20.0,  // 20pt
		densityThreshold: 2,
		precision:        10.0, // 10pt精度
	}
}

// ColumnLayout 列布局
type ColumnLayout struct {
	PageNumber   int
	Columns      []ColumnInfo
	ColumnCount  int
	IsMultiColumn bool
	GutterWidth  float64 // 列间距
	PageWidth    float64
	PageHeight   float64
}

// ColumnInfo 列信息
type ColumnInfo struct {
	Index       int
	StartX      float64
	EndX        float64
	Width       float64
	Blocks      []ClusteredTextBlock
	BlockCount  int
	TextDensity float64
}

// DetectColumns 检测页面的列布局
func (cd *ColumnDetector) DetectColumns(page *PDFPageFlow) *ColumnLayout {
	log.Printf("检测页面 %d 的列布局", page.PageNumber)
	
	layout := &ColumnLayout{
		PageNumber: page.PageNumber,
		PageWidth:  page.MediaBox.Width,
		PageHeight: page.MediaBox.Height,
	}
	
	if len(page.TextElements) == 0 {
		layout.Columns = []ColumnInfo{{
			Index:  0,
			StartX: 0,
			EndX:   page.MediaBox.Width,
			Width:  page.MediaBox.Width,
		}}
		layout.ColumnCount = 1
		layout.IsMultiColumn = false
		return layout
	}
	
	// 1. 构建X坐标直方图
	histogram := cd.buildXHistogram(page.TextElements, page.MediaBox.Width)
	
	// 2. 平滑直方图
	smoothed := cd.smoothHistogram(histogram)
	
	// 3. 查找列边界
	boundaries := cd.findColumnBoundaries(smoothed, page.MediaBox.Width)
	
	// 4. 创建列信息
	columns := cd.createColumns(boundaries, page.MediaBox.Width)
	
	// 5. 分配文本块到列
	cd.assignBlocksToColumns(columns, page.TextElements)
	
	// 6. 计算列间距
	gutterWidth := cd.calculateGutterWidth(columns)
	
	layout.Columns = columns
	layout.ColumnCount = len(columns)
	layout.IsMultiColumn = len(columns) > 1
	layout.GutterWidth = gutterWidth
	
	log.Printf("检测到 %d 列布局，列间距=%.1fpt", layout.ColumnCount, layout.GutterWidth)
	
	return layout
}

// buildXHistogram 构建X坐标直方图
func (cd *ColumnDetector) buildXHistogram(elements []TextElementFlow, pageWidth float64) map[int]int {
	histogram := make(map[int]int)
	
	for _, elem := range elements {
		// 将元素的X坐标范围映射到直方图
		startBin := int(elem.Position.X / cd.precision)
		endBin := int((elem.Position.X + elem.BoundingBox.Width) / cd.precision)
		
		for bin := startBin; bin <= endBin; bin++ {
			histogram[bin]++
		}
	}
	
	return histogram
}

// smoothHistogram 平滑直方图（移动平均）
func (cd *ColumnDetector) smoothHistogram(histogram map[int]int) map[int]float64 {
	smoothed := make(map[int]float64)
	windowSize := 3 // 窗口大小
	
	// 找到最大bin
	maxBin := 0
	for bin := range histogram {
		if bin > maxBin {
			maxBin = bin
		}
	}
	
	// 应用移动平均
	for bin := 0; bin <= maxBin; bin++ {
		sum := 0
		count := 0
		
		for offset := -windowSize; offset <= windowSize; offset++ {
			if val, ok := histogram[bin+offset]; ok {
				sum += val
				count++
			}
		}
		
		if count > 0 {
			smoothed[bin] = float64(sum) / float64(count)
		}
	}
	
	return smoothed
}

// findColumnBoundaries 查找列边界
func (cd *ColumnDetector) findColumnBoundaries(histogram map[int]float64, pageWidth float64) []float64 {
	// 1. 找到所有低密度区域（潜在的列间隙）
	gaps := cd.findLowDensityRegions(histogram, pageWidth)
	
	if len(gaps) == 0 {
		// 没有找到间隙，单栏布局
		return []float64{0, pageWidth}
	}
	
	// 2. 过滤和合并间隙
	filteredGaps := cd.filterGaps(gaps)
	
	// 3. 转换为列边界
	boundaries := []float64{0}
	for _, gap := range filteredGaps {
		boundaries = append(boundaries, gap.Center)
	}
	boundaries = append(boundaries, pageWidth)
	
	return boundaries
}

// findLowDensityRegions 查找低密度区域
func (cd *ColumnDetector) findLowDensityRegions(histogram map[int]float64, pageWidth float64) []Gap {
	gaps := []Gap{}
	
	maxBin := int(pageWidth / cd.precision)
	inGap := false
	gapStart := 0
	
	for bin := 0; bin <= maxBin; bin++ {
		density := histogram[bin]
		
		if density < float64(cd.densityThreshold) && !inGap {
			// 进入低密度区域
			inGap = true
			gapStart = bin
		} else if density >= float64(cd.densityThreshold) && inGap {
			// 离开低密度区域
			inGap = false
			gapEnd := bin
			
			gap := Gap{
				Start:  float64(gapStart) * cd.precision,
				End:    float64(gapEnd) * cd.precision,
				Width:  float64(gapEnd-gapStart) * cd.precision,
				Center: float64(gapStart+gapEnd) / 2 * cd.precision,
			}
			
			// 只保留足够宽的间隙
			if gap.Width >= cd.minGapWidth {
				gaps = append(gaps, gap)
			}
		}
	}
	
	return gaps
}

// Gap 间隙
type Gap struct {
	Start  float64
	End    float64
	Width  float64
	Center float64
}

// filterGaps 过滤和合并间隙
func (cd *ColumnDetector) filterGaps(gaps []Gap) []Gap {
	if len(gaps) == 0 {
		return gaps
	}
	
	// 按中心位置排序
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].Center < gaps[j].Center
	})
	
	// 合并相近的间隙
	merged := []Gap{gaps[0]}
	
	for i := 1; i < len(gaps); i++ {
		lastGap := &merged[len(merged)-1]
		currentGap := gaps[i]
		
		// 如果两个间隙很近，合并它们
		if currentGap.Start-lastGap.End < cd.minColumnWidth/2 {
			lastGap.End = currentGap.End
			lastGap.Width = lastGap.End - lastGap.Start
			lastGap.Center = (lastGap.Start + lastGap.End) / 2
		} else {
			merged = append(merged, currentGap)
		}
	}
	
	return merged
}

// createColumns 创建列信息
func (cd *ColumnDetector) createColumns(boundaries []float64, pageWidth float64) []ColumnInfo {
	columns := []ColumnInfo{}
	
	for i := 0; i < len(boundaries)-1; i++ {
		col := ColumnInfo{
			Index:  i,
			StartX: boundaries[i],
			EndX:   boundaries[i+1],
			Width:  boundaries[i+1] - boundaries[i],
			Blocks: []ClusteredTextBlock{},
		}
		
		// 验证列宽
		if col.Width >= cd.minColumnWidth {
			columns = append(columns, col)
		}
	}
	
	// 如果没有有效的列，创建单列
	if len(columns) == 0 {
		columns = []ColumnInfo{{
			Index:  0,
			StartX: 0,
			EndX:   pageWidth,
			Width:  pageWidth,
			Blocks: []ClusteredTextBlock{},
		}}
	}
	
	return columns
}

// assignBlocksToColumns 分配文本块到列
func (cd *ColumnDetector) assignBlocksToColumns(columns []ColumnInfo, elements []TextElementFlow) {
	for _, elem := range elements {
		// 找到元素所属的列
		colIndex := cd.findColumnForElement(columns, elem)
		
		if colIndex >= 0 && colIndex < len(columns) {
			columns[colIndex].BlockCount++
		}
	}
	
	// 计算文本密度
	for i := range columns {
		col := &columns[i]
		if col.Width > 0 {
			col.TextDensity = float64(col.BlockCount) / col.Width
		}
	}
}

// findColumnForElement 查找元素所属的列
func (cd *ColumnDetector) findColumnForElement(columns []ColumnInfo, elem TextElementFlow) int {
	elemCenter := elem.Position.X + elem.BoundingBox.Width/2
	
	for i, col := range columns {
		if elemCenter >= col.StartX && elemCenter < col.EndX {
			return i
		}
	}
	
	// 如果没有找到，返回最近的列
	minDist := math.MaxFloat64
	nearestCol := 0
	
	for i, col := range columns {
		colCenter := (col.StartX + col.EndX) / 2
		dist := math.Abs(elemCenter - colCenter)
		
		if dist < minDist {
			minDist = dist
			nearestCol = i
		}
	}
	
	return nearestCol
}

// calculateGutterWidth 计算列间距
func (cd *ColumnDetector) calculateGutterWidth(columns []ColumnInfo) float64 {
	if len(columns) < 2 {
		return 0
	}
	
	totalGutter := 0.0
	gutterCount := 0
	
	for i := 0; i < len(columns)-1; i++ {
		gutter := columns[i+1].StartX - columns[i].EndX
		if gutter > 0 {
			totalGutter += gutter
			gutterCount++
		}
	}
	
	if gutterCount > 0 {
		return totalGutter / float64(gutterCount)
	}
	
	return 0
}

// ReorderBlocksInColumns 在列内重新排序文本块
func (cd *ColumnDetector) ReorderBlocksInColumns(layout *ColumnLayout, blocks []ClusteredTextBlock) []ClusteredTextBlock {
	if !layout.IsMultiColumn {
		// 单栏，按Y坐标排序
		return sortBlocksByY(blocks)
	}
	
	// 多栏，先按列排序，再按Y坐标排序
	reordered := []ClusteredTextBlock{}
	
	for _, col := range layout.Columns {
		// 找到属于该列的块
		colBlocks := []ClusteredTextBlock{}
		for _, block := range blocks {
			blockCenter := block.BoundingBox.X + block.BoundingBox.Width/2
			if blockCenter >= col.StartX && blockCenter < col.EndX {
				colBlocks = append(colBlocks, block)
			}
		}
		
		// 在列内按Y坐标排序
		colBlocks = sortBlocksByY(colBlocks)
		
		// 添加到结果
		reordered = append(reordered, colBlocks...)
	}
	
	// 重新分配阅读顺序
	for i := range reordered {
		reordered[i].ReadingOrder = i
	}
	
	return reordered
}

// sortBlocksByY 按Y坐标排序文本块
func sortBlocksByY(blocks []ClusteredTextBlock) []ClusteredTextBlock {
	sorted := make([]ClusteredTextBlock, len(blocks))
	copy(sorted, blocks)
	
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].BoundingBox.Y > sorted[j].BoundingBox.Y
	})
	
	return sorted
}

// GetColumnStatistics 获取列统计信息
func (layout *ColumnLayout) GetColumnStatistics() map[string]interface{} {
	stats := map[string]interface{}{
		"page_number":     layout.PageNumber,
		"column_count":    layout.ColumnCount,
		"is_multi_column": layout.IsMultiColumn,
		"gutter_width":    fmt.Sprintf("%.1f", layout.GutterWidth),
		"page_width":      fmt.Sprintf("%.1f", layout.PageWidth),
		"page_height":     fmt.Sprintf("%.1f", layout.PageHeight),
	}
	
	columns := []map[string]interface{}{}
	for _, col := range layout.Columns {
		columns = append(columns, map[string]interface{}{
			"index":        col.Index,
			"start_x":      fmt.Sprintf("%.1f", col.StartX),
			"end_x":        fmt.Sprintf("%.1f", col.EndX),
			"width":        fmt.Sprintf("%.1f", col.Width),
			"block_count":  col.BlockCount,
			"text_density": fmt.Sprintf("%.4f", col.TextDensity),
		})
	}
	stats["columns"] = columns
	
	return stats
}

// DetectColumnType 检测列类型
func (cd *ColumnDetector) DetectColumnType(layout *ColumnLayout) string {
	if layout.ColumnCount == 1 {
		return "single"
	} else if layout.ColumnCount == 2 {
		// 检查是否为对称双栏
		if len(layout.Columns) == 2 {
			col1 := layout.Columns[0]
			col2 := layout.Columns[1]
			
			widthDiff := math.Abs(col1.Width - col2.Width)
			if widthDiff < 20 {
				return "symmetric_double"
			} else {
				return "asymmetric_double"
			}
		}
		return "double"
	} else if layout.ColumnCount == 3 {
		return "triple"
	} else {
		return "multi"
	}
}

// AdjustColumnsForTranslation 为翻译调整列布局
func (cd *ColumnDetector) AdjustColumnsForTranslation(
	layout *ColumnLayout,
	targetLang string,
) *ColumnLayout {
	
	// 根据目标语言调整列宽
	// 中文通常需要更宽的列
	if targetLang == "zh" || targetLang == "ja" || targetLang == "ko" {
		// 如果是多栏，可能需要减少列数或增加列宽
		if layout.ColumnCount > 2 {
			log.Printf("警告：目标语言为 %s，多栏布局可能导致文本拥挤", targetLang)
		}
	}
	
	return layout
}

// VisualizeColumns 可视化列布局（用于调试）
func (cd *ColumnDetector) VisualizeColumns(layout *ColumnLayout) string {
	visual := fmt.Sprintf("页面 %d 列布局:\n", layout.PageNumber)
	visual += fmt.Sprintf("页面宽度: %.1fpt\n", layout.PageWidth)
	visual += fmt.Sprintf("列数: %d\n", layout.ColumnCount)
	visual += fmt.Sprintf("列间距: %.1fpt\n\n", layout.GutterWidth)
	
	for _, col := range layout.Columns {
		visual += fmt.Sprintf("列 %d:\n", col.Index)
		visual += fmt.Sprintf("  范围: %.1f - %.1fpt\n", col.StartX, col.EndX)
		visual += fmt.Sprintf("  宽度: %.1fpt\n", col.Width)
		visual += fmt.Sprintf("  文本块: %d\n", col.BlockCount)
		visual += fmt.Sprintf("  密度: %.4f\n\n", col.TextDensity)
	}
	
	return visual
}

// SetMinColumnWidth 设置最小列宽
func (cd *ColumnDetector) SetMinColumnWidth(width float64) {
	cd.minColumnWidth = width
}

// SetMinGapWidth 设置最小间隙宽度
func (cd *ColumnDetector) SetMinGapWidth(width float64) {
	cd.minGapWidth = width
}

// SetDensityThreshold 设置密度阈值
func (cd *ColumnDetector) SetDensityThreshold(threshold int) {
	cd.densityThreshold = threshold
}
