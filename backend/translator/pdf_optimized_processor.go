package translator

import (
	"fmt"
	"log"
)

// OptimizedPDFProcessor 优化的PDF处理器
// 集成了坐标精确计算、公式保护、布局调整、文本聚类等pdf2zh技术
type OptimizedPDFProcessor struct {
	baseProcessor    *PDFFlowProcessor
	positionCalc     *TextPositionCalculator
	formulaProtector *FormulaProtector
	fontMetrics      *FontMetricsCalculator
	layoutAdjuster   *LayoutAdjuster
	textClusterer    *TextClusterer
	columnDetector   *ColumnDetector
	logger           *PDFLogger
}

// NewOptimizedPDFProcessor 创建优化的PDF处理器
func NewOptimizedPDFProcessor(inputPath, outputPath string) (*OptimizedPDFProcessor, error) {
	// 创建基础处理器
	baseProcessor, err := NewPDFFlowProcessor(inputPath, outputPath)
	if err != nil {
		return nil, fmt.Errorf("创建基础处理器失败: %w", err)
	}
	
	return &OptimizedPDFProcessor{
		baseProcessor:    baseProcessor,
		positionCalc:     NewTextPositionCalculator(),
		formulaProtector: NewFormulaProtector(),
		fontMetrics:      GetGlobalFontMetrics(),
		layoutAdjuster:   NewLayoutAdjuster(),
		textClusterer:    NewTextClusterer(),
		columnDetector:   NewColumnDetector(),
		logger:           baseProcessor.logger,
	}, nil
}

// ProcessPDFWithOptimization 使用优化技术处理PDF
func (opp *OptimizedPDFProcessor) ProcessPDFWithOptimization() error {
	opp.logger.Info("开始优化处理PDF", nil)
	
	// 1. 基础解析
	if err := opp.baseProcessor.ProcessPDF(); err != nil {
		return fmt.Errorf("基础解析失败: %w", err)
	}
	
	// 2. 应用优化
	if err := opp.applyOptimizations(); err != nil {
		return fmt.Errorf("应用优化失败: %w", err)
	}
	
	opp.logger.Info("优化处理完成", nil)
	return nil
}

// applyOptimizations 应用所有优化
func (opp *OptimizedPDFProcessor) applyOptimizations() error {
	flowData := opp.baseProcessor.flowData
	if flowData == nil {
		return fmt.Errorf("流数据为空")
	}
	
	totalProtected := 0
	totalRecalculated := 0
	totalClustered := 0
	totalColumns := 0
	
	for pageIdx := range flowData.Pages {
		page := &flowData.Pages[pageIdx]
		
		opp.logger.Info("优化页面", map[string]interface{}{
			"页码": page.PageNumber,
		})
		
		// 1. 重新计算精确坐标
		recalculated := opp.recalculatePositions(page)
		totalRecalculated += recalculated
		
		// 2. 保护公式
		protected := opp.formulaProtector.ProtectFormulas(page)
		totalProtected += protected
		
		// 3. 文本聚类
		blocks := opp.textClusterer.ClusterPageBlocks(page)
		totalClustered += len(blocks)
		
		opp.logger.Info("文本聚类完成", map[string]interface{}{
			"页码":  page.PageNumber,
			"文本块": len(blocks),
		})
		
		// 4. 检测列布局
		columnLayout := opp.columnDetector.DetectColumns(page)
		totalColumns += columnLayout.ColumnCount
		
		opp.logger.Info("列布局检测完成", map[string]interface{}{
			"页码": page.PageNumber,
			"列数": columnLayout.ColumnCount,
		})
		
		// 5. 根据列布局重新排序文本块
		if columnLayout.IsMultiColumn {
			blocks = opp.columnDetector.ReorderBlocksInColumns(columnLayout, blocks)
			opp.logger.Info("多栏布局重排完成", map[string]interface{}{
				"页码": page.PageNumber,
			})
		}
		
		opp.logger.Info("页面优化完成", map[string]interface{}{
			"页码":    page.PageNumber,
			"重新计算":  recalculated,
			"保护公式数": protected,
			"文本块数":  len(blocks),
			"列数":    columnLayout.ColumnCount,
		})
	}
	
	opp.logger.Info("所有页面优化完成", map[string]interface{}{
		"总重新计算": totalRecalculated,
		"总保护公式": totalProtected,
		"总文本块":  totalClustered,
		"平均列数":  float64(totalColumns) / float64(len(flowData.Pages)),
	})
	
	// 保存优化后的流数据
	return opp.baseProcessor.saveFlowData()
}

// recalculatePositions 重新计算精确位置
func (opp *OptimizedPDFProcessor) recalculatePositions(page *PDFPageFlow) int {
	recalculatedCount := 0
	
	// 重置位置计算器
	opp.positionCalc = NewTextPositionCalculator()
	
	// 状态栈（用于q/Q操作符）
	stateStack := make([]*TextPositionCalculator, 0)
	
	// 遍历所有内容流
	for _, stream := range page.ContentStreams {
		for _, op := range stream.ParsedOps {
			// 处理状态保存/恢复
			if op.Operator == "q" {
				// 保存状态
				stateStack = append(stateStack, opp.positionCalc.Clone())
			} else if op.Operator == "Q" {
				// 恢复状态
				if len(stateStack) > 0 {
					opp.positionCalc.Restore(stateStack[len(stateStack)-1])
					stateStack = stateStack[:len(stateStack)-1]
				}
			}
			
			// 更新位置计算器状态
			opp.positionCalc.ProcessOperator(op)
			
			// 如果是文本显示操作符，重新计算位置
			if op.Operator == "Tj" || op.Operator == "TJ" || op.Operator == "'" || op.Operator == "\"" {
				// 提取文本内容
				text := opp.extractTextFromOp(op)
				if text == "" {
					continue
				}
				
				// 计算精确位置
				x, y, width, height := opp.positionCalc.CalculateTextPosition(text)
				
				// 查找对应的文本元素并更新
				for i := range page.TextElements {
					elem := &page.TextElements[i]
					
					// 简单匹配：内容相同
					if elem.Content == text {
						// 检查位置是否有显著变化
						if !opp.positionsMatch(elem.Position.X, elem.Position.Y, x, y) {
							opp.logger.Debug("更新文本位置", map[string]interface{}{
								"内容":  opp.logger.truncateString(text, 50),
								"旧X":  fmt.Sprintf("%.2f", elem.Position.X),
								"旧Y":  fmt.Sprintf("%.2f", elem.Position.Y),
								"新X":  fmt.Sprintf("%.2f", x),
								"新Y":  fmt.Sprintf("%.2f", y),
								"宽度变化": fmt.Sprintf("%.2f -> %.2f", elem.BoundingBox.Width, width),
							})
							
							// 更新位置
							elem.Position.X = x
							elem.Position.Y = y
							elem.BoundingBox.X = x
							elem.BoundingBox.Y = y
							elem.BoundingBox.Width = width
							elem.BoundingBox.Height = height
							
							recalculatedCount++
						}
						break
					}
				}
				
				// 更新文本位置（为下一个文本做准备）
				opp.positionCalc.UpdateTextPosition(text)
			}
		}
	}
	
	return recalculatedCount
}

// positionsMatch 检查位置是否匹配（允许小误差）
func (opp *OptimizedPDFProcessor) positionsMatch(x1, y1, x2, y2 float64) bool {
	tolerance := 2.0 // 2pt容差
	
	dx := x1 - x2
	if dx < 0 {
		dx = -dx
	}
	
	dy := y1 - y2
	if dy < 0 {
		dy = -dy
	}
	
	return dx < tolerance && dy < tolerance
}

// extractTextFromOp 从操作符提取文本
func (opp *OptimizedPDFProcessor) extractTextFromOp(op PDFOperation) string {
	if len(op.Operands) == 0 {
		return ""
	}
	
	// 简化版本，实际应该使用baseProcessor的方法
	switch op.Operator {
	case "Tj":
		return opp.cleanPDFText(op.Operands[0])
	case "TJ":
		return opp.extractTextFromTJArray(op.Operands[0])
	case "'":
		return opp.cleanPDFText(op.Operands[0])
	case "\"":
		if len(op.Operands) >= 3 {
			return opp.cleanPDFText(op.Operands[2])
		}
	}
	
	return ""
}

// cleanPDFText 清理PDF文本
func (opp *OptimizedPDFProcessor) cleanPDFText(text string) string {
	// 移除括号
	text = trimParentheses(text)
	// 移除转义字符
	text = unescapePDFString(text)
	return text
}

// extractTextFromTJArray 从TJ数组提取文本
func (opp *OptimizedPDFProcessor) extractTextFromTJArray(arrayStr string) string {
	// 简化实现
	return opp.cleanPDFText(arrayStr)
}

// ApplyTranslationsWithProtection 应用翻译（保护公式）
func (opp *OptimizedPDFProcessor) ApplyTranslationsWithProtection(translations map[string]string) error {
	opp.logger.Info("应用翻译（保护公式+布局调整）", map[string]interface{}{
		"翻译项数": len(translations),
	})
	
	// 1. 过滤掉公式占位符的翻译
	filteredTranslations := make(map[string]string)
	for original, translated := range translations {
		// 如果是占位符，不翻译
		if opp.formulaProtector.IsPlaceholder(original) {
			opp.logger.Debug("跳过公式占位符", map[string]interface{}{
				"占位符": original,
			})
			continue
		}
		
		// 恢复译文中的公式占位符
		translated = opp.formulaProtector.RestoreFormulas(translated)
		
		filteredTranslations[original] = translated
	}
	
	opp.logger.Info("过滤后的翻译", map[string]interface{}{
		"原始数量": len(translations),
		"过滤后":  len(filteredTranslations),
	})
	
	// 2. 应用布局调整
	if err := opp.applyLayoutAdjustments(filteredTranslations); err != nil {
		opp.logger.Warn("布局调整失败", map[string]interface{}{
			"错误": err.Error(),
		})
	}
	
	// 3. 应用翻译
	return opp.baseProcessor.ApplyTranslations(filteredTranslations)
}

// applyLayoutAdjustments 应用布局调整
func (opp *OptimizedPDFProcessor) applyLayoutAdjustments(translations map[string]string) error {
	flowData := opp.baseProcessor.flowData
	if flowData == nil {
		return fmt.Errorf("流数据为空")
	}
	
	totalAdjusted := 0
	totalOverflow := 0
	
	for pageIdx := range flowData.Pages {
		page := &flowData.Pages[pageIdx]
		
		// 优化页面布局
		if err := opp.layoutAdjuster.OptimizePageLayout(page, translations, "zh"); err != nil {
			opp.logger.Warn("页面布局优化失败", map[string]interface{}{
				"页码": page.PageNumber,
				"错误": err.Error(),
			})
			continue
		}
		
		// 统计调整情况
		for _, elem := range page.TextElements {
			if translated, ok := translations[elem.Content]; ok {
				// 检查是否需要调整
				originalWidth := opp.fontMetrics.CalculateTextWidth(elem.Content, elem.Font.Name, elem.Font.Size)
				translatedWidth := opp.fontMetrics.CalculateTextWidth(translated, elem.Font.Name, elem.Font.Size)
				
				if translatedWidth > originalWidth*1.2 {
					totalAdjusted++
					
					if translatedWidth > elem.BoundingBox.Width {
						totalOverflow++
					}
				}
			}
		}
	}
	
	opp.logger.Info("布局调整完成", map[string]interface{}{
		"调整元素数": totalAdjusted,
		"溢出元素数": totalOverflow,
	})
	
	return nil
}

// GeneratePDF 生成PDF
func (opp *OptimizedPDFProcessor) GeneratePDF() error {
	return opp.baseProcessor.GeneratePDF()
}

// Cleanup 清理
func (opp *OptimizedPDFProcessor) Cleanup() error {
	opp.formulaProtector.Clear()
	return opp.baseProcessor.Cleanup()
}

// GetStatistics 获取统计信息
func (opp *OptimizedPDFProcessor) GetStatistics() map[string]interface{} {
	formulaStats := opp.formulaProtector.GetStatistics()
	
	stats := map[string]interface{}{
		"formula_protection": formulaStats,
		"position_calculator": opp.positionCalc.GetCurrentState(),
		"font_metrics": map[string]interface{}{
			"cache_size":      opp.fontMetrics.GetCacheSize(),
			"loaded_fonts":    len(opp.fontMetrics.GetLoadedFonts()),
			"available_fonts": len(opp.fontMetrics.GetAvailableFonts()),
		},
	}
	
	// 添加页面级统计
	if opp.baseProcessor.flowData != nil {
		pageStats := []map[string]interface{}{}
		
		for _, page := range opp.baseProcessor.flowData.Pages {
			// 分析布局复杂度
			complexity := opp.layoutAdjuster.AnalyzeLayoutComplexity(&page)
			
			// 检测列布局
			columnLayout := opp.columnDetector.DetectColumns(&page)
			
			pageStats = append(pageStats, map[string]interface{}{
				"page_number":  page.PageNumber,
				"text_elements": len(page.TextElements),
				"complexity":   complexity.GetStatistics(),
				"columns":      columnLayout.GetColumnStatistics(),
			})
		}
		
		stats["pages"] = pageStats
	}
	
	return stats
}

// GetWorkDir 获取工作目录
func (opp *OptimizedPDFProcessor) GetWorkDir() string {
	return opp.baseProcessor.workDir
}

// GetFlowData 获取流数据
func (opp *OptimizedPDFProcessor) GetFlowData() *PDFFlowData {
	return opp.baseProcessor.flowData
}

// trimParentheses 移除括号
func trimParentheses(s string) string {
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		return s[1 : len(s)-1]
	}
	return s
}

// unescapePDFString 反转义PDF字符串
func unescapePDFString(s string) string {
	// 简化实现
	s = replaceAll(s, "\\n", "\n")
	s = replaceAll(s, "\\r", "\r")
	s = replaceAll(s, "\\t", "\t")
	s = replaceAll(s, "\\(", "(")
	s = replaceAll(s, "\\)", ")")
	s = replaceAll(s, "\\\\", "\\")
	return s
}

// replaceAll 替换所有
func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}

// 使用示例函数
func ExampleOptimizedProcessing(inputPath, outputPath string, translations map[string]string) error {
	log.Printf("开始优化处理: %s -> %s", inputPath, outputPath)
	
	// 1. 创建优化处理器
	processor, err := NewOptimizedPDFProcessor(inputPath, outputPath)
	if err != nil {
		return fmt.Errorf("创建处理器失败: %w", err)
	}
	defer processor.Cleanup()
	
	// 2. 处理PDF（包含坐标优化和公式保护）
	if err := processor.ProcessPDFWithOptimization(); err != nil {
		return fmt.Errorf("处理PDF失败: %w", err)
	}
	
	// 3. 应用翻译（自动保护公式）
	if err := processor.ApplyTranslationsWithProtection(translations); err != nil {
		return fmt.Errorf("应用翻译失败: %w", err)
	}
	
	// 4. 生成PDF
	if err := processor.GeneratePDF(); err != nil {
		return fmt.Errorf("生成PDF失败: %w", err)
	}
	
	// 5. 打印统计信息
	stats := processor.GetStatistics()
	log.Printf("处理统计: %+v", stats)
	
	log.Printf("优化处理完成: %s", outputPath)
	return nil
}
