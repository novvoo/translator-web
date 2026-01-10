package translator

import (
	"fmt"
	"log"
	"math"
	"sort"
)

// TextClusterer 文本聚类器
// 实现文本块聚类、段落识别、阅读顺序推断
type TextClusterer struct {
	minDistance      float64 // 最小聚类距离
	maxDistance      float64 // 最大聚类距离
	lineHeightFactor float64 // 行高因子
}

// NewTextClusterer 创建文本聚类器
func NewTextClusterer() *TextClusterer {
	return &TextClusterer{
		minDistance:      5.0,  // 5pt
		maxDistance:      50.0, // 50pt
		lineHeightFactor: 1.5,  // 1.5倍行高
	}
}

// ClusteredTextBlock 聚类后的文本块
type ClusteredTextBlock struct {
	ID           string
	Elements     []TextElementFlow
	Type         string      // "paragraph", "title", "list", "formula", "caption"
	BoundingBox  BoundingBox
	ReadingOrder int
	FontSize     float64 // 平均字体大小
	FontName     string  // 主要字体
	Alignment    string  // "left", "center", "right", "justify"
	Indentation  float64 // 缩进
	LineSpacing  float64 // 行间距
}

// ClusterTextElements 聚类文本元素
func (tc *TextClusterer) ClusterTextElements(elements []TextElementFlow) []ClusteredTextBlock {
	if len(elements) == 0 {
		return []ClusteredTextBlock{}
	}
	
	log.Printf("开始聚类 %d 个文本元素", len(elements))
	
	// 1. 按Y坐标排序（从上到下）
	sortedElements := make([]TextElementFlow, len(elements))
	copy(sortedElements, elements)
	sort.Slice(sortedElements, func(i, j int) bool {
		// Y坐标相近时，按X坐标排序
		if math.Abs(sortedElements[i].Position.Y-sortedElements[j].Position.Y) < 5 {
			return sortedElements[i].Position.X < sortedElements[j].Position.X
		}
		return sortedElements[i].Position.Y > sortedElements[j].Position.Y
	})
	
	// 2. 使用DBSCAN聚类
	clusters := tc.dbscanClustering(sortedElements)
	
	log.Printf("聚类完成，生成 %d 个文本块", len(clusters))
	
	// 3. 分析每个聚类
	blocks := make([]ClusteredTextBlock, 0, len(clusters))
	for i, cluster := range clusters {
		block := tc.analyzeCluster(cluster, i)
		blocks = append(blocks, block)
	}
	
	// 4. 推断阅读顺序
	tc.inferReadingOrder(blocks)
	
	// 5. 分类文本块类型
	tc.classifyBlocks(blocks)
	
	return blocks
}

// dbscanClustering DBSCAN聚类算法
func (tc *TextClusterer) dbscanClustering(elements []TextElementFlow) [][]TextElementFlow {
	visited := make(map[int]bool)
	clusters := [][]TextElementFlow{}
	
	for i := range elements {
		if visited[i] {
			continue
		}
		
		// 查找邻居
		neighbors := tc.findNeighbors(elements, i)
		
		if len(neighbors) == 0 {
			// 噪声点，单独成簇
			clusters = append(clusters, []TextElementFlow{elements[i]})
			visited[i] = true
			continue
		}
		
		// 创建新簇
		cluster := []TextElementFlow{}
		queue := []int{i}
		visited[i] = true
		
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			
			cluster = append(cluster, elements[current])
			
			// 查找当前点的邻居
			currentNeighbors := tc.findNeighbors(elements, current)
			
			for _, neighbor := range currentNeighbors {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
		
		clusters = append(clusters, cluster)
	}
	
	return clusters
}

// findNeighbors 查找邻居
func (tc *TextClusterer) findNeighbors(elements []TextElementFlow, index int) []int {
	neighbors := []int{}
	elem := elements[index]
	
	for i, other := range elements {
		if i == index {
			continue
		}
		
		distance := tc.calculateDistance(elem, other)
		
		// 判断是否为邻居
		if distance < tc.maxDistance {
			// 额外检查：是否在合理的行间距内
			yDiff := math.Abs(elem.Position.Y - other.Position.Y)
			maxLineSpacing := math.Max(elem.Font.Size, other.Font.Size) * tc.lineHeightFactor
			
			if yDiff < maxLineSpacing || distance < tc.minDistance {
				neighbors = append(neighbors, i)
			}
		}
	}
	
	return neighbors
}

// calculateDistance 计算两个元素之间的距离
func (tc *TextClusterer) calculateDistance(a, b TextElementFlow) float64 {
	// 使用曼哈顿距离（更适合文本布局）
	dx := math.Abs(a.Position.X - b.Position.X)
	dy := math.Abs(a.Position.Y - b.Position.Y)
	
	// Y方向的距离权重更大（同一行的元素应该聚在一起）
	return dx + dy*2
}

// analyzeCluster 分析聚类
func (tc *TextClusterer) analyzeCluster(cluster []TextElementFlow, id int) ClusteredTextBlock {
	if len(cluster) == 0 {
		return ClusteredTextBlock{}
	}
	
	// 计算边界框
	minX := cluster[0].Position.X
	minY := cluster[0].Position.Y
	maxX := cluster[0].Position.X + cluster[0].BoundingBox.Width
	maxY := cluster[0].Position.Y + cluster[0].BoundingBox.Height
	
	totalFontSize := 0.0
	fontCounts := make(map[string]int)
	
	for _, elem := range cluster {
		// 更新边界
		if elem.Position.X < minX {
			minX = elem.Position.X
		}
		if elem.Position.Y < minY {
			minY = elem.Position.Y
		}
		if elem.Position.X+elem.BoundingBox.Width > maxX {
			maxX = elem.Position.X + elem.BoundingBox.Width
		}
		if elem.Position.Y+elem.BoundingBox.Height > maxY {
			maxY = elem.Position.Y + elem.BoundingBox.Height
		}
		
		// 统计字体
		totalFontSize += elem.Font.Size
		fontCounts[elem.Font.Name]++
	}
	
	// 找出主要字体
	mainFont := ""
	maxCount := 0
	for font, count := range fontCounts {
		if count > maxCount {
			maxCount = count
			mainFont = font
		}
	}
	
	// 计算平均字体大小
	avgFontSize := totalFontSize / float64(len(cluster))
	
	// 计算对齐方式
	alignment := tc.detectAlignment(cluster, minX, maxX)
	
	// 计算缩进
	indentation := minX
	
	// 计算行间距
	lineSpacing := tc.calculateLineSpacing(cluster)
	
	return ClusteredTextBlock{
		ID:           fmt.Sprintf("block_%d", id),
		Elements:     cluster,
		Type:         "unknown", // 稍后分类
		BoundingBox:  BoundingBox{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY},
		ReadingOrder: 0, // 稍后推断
		FontSize:     avgFontSize,
		FontName:     mainFont,
		Alignment:    alignment,
		Indentation:  indentation,
		LineSpacing:  lineSpacing,
	}
}

// detectAlignment 检测对齐方式
func (tc *TextClusterer) detectAlignment(cluster []TextElementFlow, minX, maxX float64) string {
	if len(cluster) == 0 {
		return "left"
	}
	
	// 统计左对齐、右对齐、居中的元素数量
	leftCount := 0
	rightCount := 0
	centerCount := 0
	tolerance := 5.0 // 5pt容差
	
	for _, elem := range cluster {
		leftDist := math.Abs(elem.Position.X - minX)
		rightDist := math.Abs(elem.Position.X + elem.BoundingBox.Width - maxX)
		centerX := (minX + maxX) / 2
		centerDist := math.Abs(elem.Position.X + elem.BoundingBox.Width/2 - centerX)
		
		if leftDist < tolerance {
			leftCount++
		}
		if rightDist < tolerance {
			rightCount++
		}
		if centerDist < tolerance {
			centerCount++
		}
	}
	
	// 判断主要对齐方式
	if centerCount > len(cluster)/2 {
		return "center"
	} else if rightCount > len(cluster)/2 {
		return "right"
	} else if leftCount > len(cluster)/2 {
		return "left"
	} else {
		return "justify"
	}
}

// calculateLineSpacing 计算行间距
func (tc *TextClusterer) calculateLineSpacing(cluster []TextElementFlow) float64 {
	if len(cluster) < 2 {
		return 1.2
	}
	
	// 按Y坐标排序
	sorted := make([]TextElementFlow, len(cluster))
	copy(sorted, cluster)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Position.Y > sorted[j].Position.Y
	})
	
	// 计算相邻行的间距
	spacings := []float64{}
	for i := 0; i < len(sorted)-1; i++ {
		yDiff := sorted[i].Position.Y - sorted[i+1].Position.Y
		fontSize := sorted[i].Font.Size
		
		if yDiff > 0 && fontSize > 0 {
			spacing := yDiff / fontSize
			spacings = append(spacings, spacing)
		}
	}
	
	if len(spacings) == 0 {
		return 1.2
	}
	
	// 计算平均行间距
	total := 0.0
	for _, s := range spacings {
		total += s
	}
	
	return total / float64(len(spacings))
}

// inferReadingOrder 推断阅读顺序
func (tc *TextClusterer) inferReadingOrder(blocks []ClusteredTextBlock) {
	// Z字形扫描：从上到下，从左到右
	sort.Slice(blocks, func(i, j int) bool {
		// 如果Y坐标相近（在同一行），按X坐标排序
		yDiff := math.Abs(blocks[i].BoundingBox.Y - blocks[j].BoundingBox.Y)
		if yDiff < 20 { // 20pt容差
			return blocks[i].BoundingBox.X < blocks[j].BoundingBox.X
		}
		// 否则按Y坐标排序（从上到下）
		return blocks[i].BoundingBox.Y > blocks[j].BoundingBox.Y
	})
	
	// 分配阅读顺序
	for i := range blocks {
		blocks[i].ReadingOrder = i
	}
}

// classifyBlocks 分类文本块
func (tc *TextClusterer) classifyBlocks(blocks []ClusteredTextBlock) {
	if len(blocks) == 0 {
		return
	}
	
	// 计算平均字体大小
	totalFontSize := 0.0
	for _, block := range blocks {
		totalFontSize += block.FontSize
	}
	avgFontSize := totalFontSize / float64(len(blocks))
	
	for i := range blocks {
		block := &blocks[i]
		
		// 1. 检查是否为标题（字体大于平均值）
		if block.FontSize > avgFontSize*1.2 {
			block.Type = "title"
			continue
		}
		
		// 2. 检查是否为列表（有缩进或特殊符号）
		if tc.isListBlock(block) {
			block.Type = "list"
			continue
		}
		
		// 3. 检查是否为公式
		if tc.isFormulaBlock(block) {
			block.Type = "formula"
			continue
		}
		
		// 4. 检查是否为图注
		if tc.isCaptionBlock(block) {
			block.Type = "caption"
			continue
		}
		
		// 5. 默认为段落
		block.Type = "paragraph"
	}
}

// isListBlock 检查是否为列表块
func (tc *TextClusterer) isListBlock(block *ClusteredTextBlock) bool {
	if len(block.Elements) == 0 {
		return false
	}
	
	// 检查第一个元素是否以列表符号开头
	firstText := block.Elements[0].Content
	listMarkers := []string{"•", "◦", "▪", "▫", "–", "—", "1.", "2.", "3.", "a.", "b.", "c.", "(1)", "(2)", "(3)"}
	
	for _, marker := range listMarkers {
		if startsWithIgnoreSpace(firstText, marker) {
			return true
		}
	}
	
	// 检查是否有明显的缩进
	if block.Indentation > 20 {
		return true
	}
	
	return false
}

// isFormulaBlock 检查是否为公式块
func (tc *TextClusterer) isFormulaBlock(block *ClusteredTextBlock) bool {
	// 检查是否所有元素都标记为公式
	formulaCount := 0
	for _, elem := range block.Elements {
		if elem.IsFormula {
			formulaCount++
		}
	}
	
	return formulaCount > len(block.Elements)/2
}

// isCaptionBlock 检查是否为图注
func (tc *TextClusterer) isCaptionBlock(block *ClusteredTextBlock) bool {
	if len(block.Elements) == 0 {
		return false
	}
	
	// 检查是否以"图"、"表"、"Fig"、"Table"等开头
	firstText := block.Elements[0].Content
	captionPrefixes := []string{"图", "表", "Fig", "Figure", "Table", "Equation", "公式"}
	
	for _, prefix := range captionPrefixes {
		if startsWithIgnoreSpace(firstText, prefix) {
			return true
		}
	}
	
	// 检查字体是否较小
	if block.FontSize < 10 {
		return true
	}
	
	return false
}

// GetBlockText 获取文本块的完整文本
func (block *ClusteredTextBlock) GetBlockText() string {
	text := ""
	for i, elem := range block.Elements {
		if i > 0 {
			text += " "
		}
		text += elem.Content
	}
	return text
}

// GetStatistics 获取统计信息
func (block *ClusteredTextBlock) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"id":            block.ID,
		"type":          block.Type,
		"reading_order": block.ReadingOrder,
		"element_count": len(block.Elements),
		"font_size":     fmt.Sprintf("%.1f", block.FontSize),
		"font_name":     block.FontName,
		"alignment":     block.Alignment,
		"indentation":   fmt.Sprintf("%.1f", block.Indentation),
		"line_spacing":  fmt.Sprintf("%.2f", block.LineSpacing),
		"text_length":   len(block.GetBlockText()),
	}
}

// DetectColumns 检测多栏布局
func (tc *TextClusterer) DetectColumns(blocks []ClusteredTextBlock, pageWidth float64) []Column {
	if len(blocks) == 0 {
		return []Column{}
	}
	
	// 统计X坐标分布
	xHistogram := make(map[int]int)
	for _, block := range blocks {
		x := int(block.BoundingBox.X / 10) * 10 // 10pt精度
		xHistogram[x]++
	}
	
	// 查找低密度区域（列分隔）
	gaps := tc.findGaps(xHistogram, int(pageWidth))
	
	if len(gaps) == 0 {
		// 单栏
		return []Column{{
			Index:      0,
			StartX:     0,
			EndX:       pageWidth,
			Width:      pageWidth,
			BlockCount: len(blocks),
		}}
	}
	
	// 创建列
	columns := []Column{}
	prevX := 0.0
	
	for i, gap := range gaps {
		col := Column{
			Index:  i,
			StartX: prevX,
			EndX:   float64(gap),
			Width:  float64(gap) - prevX,
		}
		
		// 统计该列的块数
		for _, block := range blocks {
			if block.BoundingBox.X >= col.StartX && block.BoundingBox.X < col.EndX {
				col.BlockCount++
			}
		}
		
		columns = append(columns, col)
		prevX = float64(gap)
	}
	
	// 添加最后一列
	lastCol := Column{
		Index:  len(columns),
		StartX: prevX,
		EndX:   pageWidth,
		Width:  pageWidth - prevX,
	}
	for _, block := range blocks {
		if block.BoundingBox.X >= lastCol.StartX {
			lastCol.BlockCount++
		}
	}
	columns = append(columns, lastCol)
	
	return columns
}

// findGaps 查找间隙
func (tc *TextClusterer) findGaps(histogram map[int]int, maxX int) []int {
	gaps := []int{}
	threshold := 2 // 密度阈值
	
	inGap := false
	gapStart := 0
	
	for x := 0; x < maxX; x += 10 {
		count := histogram[x]
		
		if count < threshold && !inGap {
			// 进入间隙
			inGap = true
			gapStart = x
		} else if count >= threshold && inGap {
			// 离开间隙
			inGap = false
			gapWidth := x - gapStart
			
			// 只记录足够宽的间隙（至少30pt）
			if gapWidth >= 30 {
				gaps = append(gaps, gapStart+gapWidth/2)
			}
		}
	}
	
	return gaps
}

// Column 列
type Column struct {
	Index      int
	StartX     float64
	EndX       float64
	Width      float64
	BlockCount int
}

// 辅助函数
func startsWithIgnoreSpace(s, prefix string) bool {
	s = trimLeadingSpace(s)
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func trimLeadingSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}

// ClusterPageBlocks 聚类整页的文本块
func (tc *TextClusterer) ClusterPageBlocks(page *PDFPageFlow) []ClusteredTextBlock {
	return tc.ClusterTextElements(page.TextElements)
}

// GetBlocksByType 按类型获取文本块
func GetBlocksByType(blocks []ClusteredTextBlock, blockType string) []ClusteredTextBlock {
	result := []ClusteredTextBlock{}
	for _, block := range blocks {
		if block.Type == blockType {
			result = append(result, block)
		}
	}
	return result
}

// GetBlocksByReadingOrder 按阅读顺序获取文本块
func GetBlocksByReadingOrder(blocks []ClusteredTextBlock) []ClusteredTextBlock {
	sorted := make([]ClusteredTextBlock, len(blocks))
	copy(sorted, blocks)
	
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ReadingOrder < sorted[j].ReadingOrder
	})
	
	return sorted
}
