package translator

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// PDFRegenerator PDF重新生成器 - 基于PDF流处理器的动态重建
type PDFRegenerator struct {
	processor *PDFFlowProcessor // PDF流处理器
}

// NewPDFRegenerator 创建PDF重新生成器
func NewPDFRegenerator() *PDFRegenerator {
	return &PDFRegenerator{}
}

// RegeneratePDF 重新生成PDF - 使用PDF流处理器进行动态重建
func (r *PDFRegenerator) RegeneratePDF(inputPath, outputPath string, translations map[string]string) error {
	log.Printf("开始重新生成PDF: %s -> %s", inputPath, outputPath)
	log.Printf("需要替换的文本数量: %d", len(translations))

	// 1. 创建PDF流处理器
	processor, err := NewPDFFlowProcessor(inputPath, outputPath)
	if err != nil {
		return fmt.Errorf("创建PDF流处理器失败: %w", err)
	}
	r.processor = processor
	defer processor.Cleanup() // 确保清理临时文件

	// 2. 解析PDF结构并保存到临时目录
	log.Printf("解析PDF结构...")
	if err := processor.ProcessPDF(); err != nil {
		return fmt.Errorf("PDF结构解析失败: %w", err)
	}

	// 3. 应用翻译到PDF流数据
	log.Printf("应用翻译...")
	if err := processor.ApplyTranslations(translations); err != nil {
		return fmt.Errorf("应用翻译失败: %w", err)
	}

	// 4. 基于更新后的流数据生成新PDF
	log.Printf("生成新PDF...")
	if err := processor.GeneratePDF(); err != nil {
		return fmt.Errorf("生成PDF失败: %w", err)
	}

	// 5. 导出处理报告
	if err := r.exportProcessingReport(processor, translations); err != nil {
		log.Printf("警告：导出处理报告失败: %v", err)
	}

	log.Printf("PDF重新生成完成: %s", outputPath)
	return nil
}

// GetWorkDir 获取工作目录（用于调试）
func (r *PDFRegenerator) GetWorkDir() string {
	if r.processor != nil {
		return r.processor.workDir
	}
	return ""
}

// GetFlowData 获取流数据（用于调试）
func (r *PDFRegenerator) GetFlowData() *PDFFlowData {
	if r.processor != nil {
		return r.processor.flowData
	}
	return nil
}

// exportProcessingReport 导出处理报告
func (r *PDFRegenerator) exportProcessingReport(processor *PDFFlowProcessor, translations map[string]string) error {
	reportPath := strings.TrimSuffix(processor.outputPath, ".pdf") + "_processing_report.txt"

	file, err := os.Create(reportPath)
	if err != nil {
		return fmt.Errorf("创建处理报告失败: %w", err)
	}
	defer file.Close()

	fmt.Fprintf(file, "=== PDF流处理器处理报告 ===\n")
	fmt.Fprintf(file, "生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "输入文件: %s\n", processor.inputPath)
	fmt.Fprintf(file, "输出文件: %s\n", processor.outputPath)
	fmt.Fprintf(file, "工作目录: %s\n", processor.workDir)
	fmt.Fprintf(file, "翻译映射数量: %d\n", len(translations))
	fmt.Fprintf(file, "\n")

	// 输出流数据统计
	if processor.flowData != nil {
		fmt.Fprintf(file, "=== PDF流数据统计 ===\n")
		fmt.Fprintf(file, "文档标题: %s\n", processor.flowData.Metadata.Title)
		fmt.Fprintf(file, "文档作者: %s\n", processor.flowData.Metadata.Author)
		fmt.Fprintf(file, "总页数: %d\n", len(processor.flowData.Pages))
		fmt.Fprintf(file, "原始文件大小: %d 字节\n", processor.flowData.OriginalSize)
		fmt.Fprintf(file, "处理时间: %s\n", processor.flowData.ProcessTime.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(file, "\n")

		// 统计各类元素
		totalTextElements := 0
		totalImageElements := 0
		totalGraphicsElements := 0
		translatedElements := 0

		for _, page := range processor.flowData.Pages {
			totalTextElements += len(page.TextElements)
			totalImageElements += len(page.ImageElements)
			totalGraphicsElements += len(page.GraphicsElements)

			// 统计已翻译的元素
			for _, element := range page.TextElements {
				if element.Language == "zh" {
					translatedElements++
				}
			}
		}

		fmt.Fprintf(file, "文本元素总数: %d\n", totalTextElements)
		fmt.Fprintf(file, "图像元素总数: %d\n", totalImageElements)
		fmt.Fprintf(file, "图形元素总数: %d\n", totalGraphicsElements)
		fmt.Fprintf(file, "已翻译元素: %d\n", translatedElements)
		if totalTextElements > 0 {
			fmt.Fprintf(file, "翻译覆盖率: %.2f%%\n", float64(translatedElements)/float64(totalTextElements)*100)
		}
		fmt.Fprintf(file, "\n")

		// 输出每页详情
		fmt.Fprintf(file, "=== 页面详情 ===\n")
		for _, page := range processor.flowData.Pages {
			fmt.Fprintf(file, "页面 %d:\n", page.PageNumber)
			fmt.Fprintf(file, "  页面尺寸: %.2f x %.2f\n", page.MediaBox.Width, page.MediaBox.Height)
			fmt.Fprintf(file, "  文本元素: %d 个\n", len(page.TextElements))
			fmt.Fprintf(file, "  图像元素: %d 个\n", len(page.ImageElements))
			fmt.Fprintf(file, "  图形元素: %d 个\n", len(page.GraphicsElements))
			fmt.Fprintf(file, "  内容流: %d 个\n", len(page.ContentStreams))

			// 输出前几个文本元素作为示例
			for i, element := range page.TextElements {
				if i >= 3 { // 只显示前3个
					break
				}
				fmt.Fprintf(file, "    文本%d: \"%s\" (%.2f, %.2f) [%s]\n",
					i+1, truncateString(element.Content, 50),
					element.Position.X, element.Position.Y, element.Language)
			}
			fmt.Fprintf(file, "\n")
		}
	}

	// 输出翻译映射表
	fmt.Fprintf(file, "=== 翻译映射表 ===\n")
	i := 1
	for original, translated := range translations {
		fmt.Fprintf(file, "[翻译%d]\n", i)
		fmt.Fprintf(file, "原文: %s\n", original)
		fmt.Fprintf(file, "译文: %s\n", translated)
		fmt.Fprintf(file, "原文长度: %d 字符\n", len(original))
		fmt.Fprintf(file, "译文长度: %d 字符\n", len(translated))
		fmt.Fprintf(file, "\n")
		i++
	}

	fmt.Fprintf(file, "=== 处理方式 ===\n")
	fmt.Fprintf(file, "1. 使用PDF流处理器解析PDF结构\n")
	fmt.Fprintf(file, "2. 将PDF流数据保存到临时目录: %s\n", processor.workDir)
	fmt.Fprintf(file, "3. 在流数据中应用翻译并重新计算布局\n")
	fmt.Fprintf(file, "4. 基于更新后的流数据重新生成PDF\n")
	fmt.Fprintf(file, "5. 保留所有图片、图形、样式等非文本元素\n")
	fmt.Fprintf(file, "6. 动态调整文本位置以适应翻译后的内容\n")
	fmt.Fprintf(file, "7. 支持通用字体和复杂排版\n")

	log.Printf("处理报告已导出: %s", reportPath)
	return nil
}
