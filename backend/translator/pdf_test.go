package translator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestOpenPDF 测试打开PDF文件
func TestOpenPDF(t *testing.T) {
	// 测试文件路径
	testPDFPath := "../../../spann.pdf"

	// 检查文件是否存在
	if _, err := os.Stat(testPDFPath); os.IsNotExist(err) {
		t.Fatalf("测试文件不存在: %s", testPDFPath)
	}

	t.Logf("开始测试PDF文件: %s", testPDFPath)

	// 测试打开PDF
	doc, err := OpenPDF(testPDFPath)
	if err != nil {
		t.Fatalf("打开PDF失败: %v", err)
	}

	// 验证基本信息
	if doc == nil {
		t.Fatal("返回的文档对象为空")
	}

	t.Logf("✓ PDF文件成功打开")
	t.Logf("  文件路径: %s", doc.Path)
	t.Logf("  总页数: %d", doc.Metadata.Pages)
	t.Logf("  提取的页面数: %d", len(doc.PageTexts))

	// 验证页数
	if doc.Metadata.Pages <= 0 {
		t.Error("页数应该大于0")
	}

	if len(doc.PageTexts) != doc.Metadata.Pages {
		t.Errorf("提取的页面数(%d)与总页数(%d)不匹配", len(doc.PageTexts), doc.Metadata.Pages)
	}

	// 统计非空页面
	nonEmptyPages := 0
	totalChars := 0

	for i, pageText := range doc.PageTexts {
		charCount := len(pageText)
		totalChars += charCount

		if charCount > 0 {
			nonEmptyPages++
			t.Logf("  第%d页: %d 字符", i+1, charCount)

			// 显示前100个字符作为预览
			preview := pageText
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			t.Logf("    预览: %s", preview)
		} else {
			t.Logf("  第%d页: 空白页", i+1)
		}
	}

	t.Logf("\n统计信息:")
	t.Logf("  非空页面: %d/%d", nonEmptyPages, doc.Metadata.Pages)
	t.Logf("  总字符数: %d", totalChars)
	t.Logf("  平均每页字符数: %.2f", float64(totalChars)/float64(doc.Metadata.Pages))

	// 验证至少有一些内容被提取
	if totalChars == 0 {
		t.Error("警告: 没有提取到任何文本内容")
	}
}

// TestValidatePDF 测试PDF验证功能
func TestValidatePDF(t *testing.T) {
	testPDFPath := "../../../spann.pdf"

	t.Log("测试PDF文件验证...")

	err := ValidatePDF(testPDFPath)
	if err != nil {
		t.Fatalf("PDF验证失败: %v", err)
	}

	t.Log("✓ PDF文件验证通过")
}

// TestGetPDFPageCount 测试获取页数功能
func TestGetPDFPageCount(t *testing.T) {
	testPDFPath := "../../../spann.pdf"

	t.Log("测试获取PDF页数...")

	pageCount, err := GetPDFPageCount(testPDFPath)
	if err != nil {
		t.Fatalf("获取页数失败: %v", err)
	}

	if pageCount <= 0 {
		t.Error("页数应该大于0")
	}

	t.Logf("✓ PDF总页数: %d", pageCount)
}

// TestGetTextBlocks 测试获取文本块功能
func TestGetTextBlocks(t *testing.T) {
	testPDFPath := "../../../spann.pdf"

	t.Log("测试获取文本块...")

	doc, err := OpenPDF(testPDFPath)
	if err != nil {
		t.Fatalf("打开PDF失败: %v", err)
	}

	blocks := doc.GetTextBlocks()

	t.Logf("✓ 提取到 %d 个文本块", len(blocks))

	// 显示前5个文本块
	displayCount := 5
	if len(blocks) < displayCount {
		displayCount = len(blocks)
	}

	t.Log("\n前几个文本块示例:")
	for i := 0; i < displayCount; i++ {
		block := blocks[i]
		preview := block
		if len(preview) > 150 {
			preview = preview[:150] + "..."
		}
		t.Logf("  块 %d: %s", i+1, preview)
	}

	// 验证文本块不为空
	if len(blocks) == 0 {
		t.Error("警告: 没有提取到任何文本块")
	}
}

// TestPDFParser 测试PDF解析器
func TestPDFParser(t *testing.T) {
	testPDFPath := "../../../spann.pdf"

	t.Log("测试PDF解析器...")

	// 创建解析器
	parser := NewPDFParser("", "")

	// 解析PDF
	content, err := parser.ParsePDF(testPDFPath)
	if err != nil {
		t.Fatalf("解析PDF失败: %v", err)
	}

	t.Logf("✓ PDF解析成功")
	t.Logf("  总页数: %d", content.PageCount)
	t.Logf("  文本块数量: %d", len(content.TextBlocks))

	// 显示元数据
	if len(content.Metadata) > 0 {
		t.Log("\n元数据:")
		for key, value := range content.Metadata {
			t.Logf("  %s: %s", key, value)
		}
	}

	// 统计公式块
	formulaCount := 0
	for _, block := range content.TextBlocks {
		if block.IsFormula {
			formulaCount++
		}
	}

	t.Logf("\n文本块统计:")
	t.Logf("  普通文本块: %d", len(content.TextBlocks)-formulaCount)
	t.Logf("  数学公式块: %d", formulaCount)

	// 显示前几个文本块的详细信息
	displayCount := 3
	if len(content.TextBlocks) < displayCount {
		displayCount = len(content.TextBlocks)
	}

	t.Log("\n文本块详细信息示例:")
	for i := 0; i < displayCount; i++ {
		block := content.TextBlocks[i]
		preview := block.Text
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		t.Logf("  块 %d:", i+1)
		t.Logf("    页码: %d", block.PageNum)
		t.Logf("    位置: (%.2f, %.2f)", block.X, block.Y)
		t.Logf("    字体: %s (%.2f)", block.FontName, block.FontSize)
		t.Logf("    是否为公式: %v", block.IsFormula)
		t.Logf("    内容: %s", preview)
	}
}

// TestPDFContentExtraction 综合测试：完整的内容提取流程
func TestPDFContentExtraction(t *testing.T) {
	testPDFPath := "../../../spann.pdf"

	t.Log("=== 综合测试：PDF内容提取完整流程 ===\n")

	// 步骤1: 验证文件
	t.Log("步骤1: 验证PDF文件...")
	if err := ValidatePDF(testPDFPath); err != nil {
		t.Fatalf("  ✗ 验证失败: %v", err)
	}
	t.Log("  ✓ 文件验证通过")

	// 步骤2: 获取页数
	t.Log("\n步骤2: 获取页数...")
	pageCount, err := GetPDFPageCount(testPDFPath)
	if err != nil {
		t.Fatalf("  ✗ 获取页数失败: %v", err)
	}
	t.Logf("  ✓ 总页数: %d", pageCount)

	// 步骤3: 打开并提取内容
	t.Log("\n步骤3: 打开PDF并提取内容...")
	doc, err := OpenPDF(testPDFPath)
	if err != nil {
		t.Fatalf("  ✗ 打开失败: %v", err)
	}
	t.Log("  ✓ PDF打开成功")

	// 步骤4: 获取文本块
	t.Log("\n步骤4: 获取文本块...")
	blocks := doc.GetTextBlocks()
	t.Logf("  ✓ 提取到 %d 个文本块", len(blocks))

	// 步骤5: 使用解析器进行高级解析
	t.Log("\n步骤5: 使用解析器进行高级解析...")
	parser := NewPDFParser("", "")
	content, err := parser.ParsePDF(testPDFPath)
	if err != nil {
		t.Fatalf("  ✗ 解析失败: %v", err)
	}
	t.Logf("  ✓ 解析成功，提取 %d 个文本块", len(content.TextBlocks))

	// 步骤6: 生成测试报告
	t.Log("\n步骤6: 生成测试报告...")
	reportPath := "../../../pdf_test_report.txt"
	if err := generateTestReport(doc, content, reportPath); err != nil {
		t.Errorf("  ✗ 生成报告失败: %v", err)
	} else {
		absPath, _ := filepath.Abs(reportPath)
		t.Logf("  ✓ 测试报告已保存到: %s", absPath)
	}

	t.Log("\n=== 测试完成 ===")
}

// generateTestReport 生成测试报告
func generateTestReport(doc *PDFDocument, content *PDFContent, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "PDF内容提取测试报告\n")
	fmt.Fprintf(file, "==================\n\n")

	fmt.Fprintf(file, "文件信息:\n")
	fmt.Fprintf(file, "  路径: %s\n", doc.Path)
	fmt.Fprintf(file, "  总页数: %d\n\n", doc.Metadata.Pages)

	fmt.Fprintf(file, "基础提取结果:\n")
	fmt.Fprintf(file, "  提取页面数: %d\n", len(doc.PageTexts))

	totalChars := 0
	nonEmptyPages := 0
	for _, text := range doc.PageTexts {
		totalChars += len(text)
		if len(text) > 0 {
			nonEmptyPages++
		}
	}
	fmt.Fprintf(file, "  非空页面: %d\n", nonEmptyPages)
	fmt.Fprintf(file, "  总字符数: %d\n", totalChars)
	fmt.Fprintf(file, "  平均每页字符数: %.2f\n\n", float64(totalChars)/float64(doc.Metadata.Pages))

	fmt.Fprintf(file, "高级解析结果:\n")
	fmt.Fprintf(file, "  文本块数量: %d\n", len(content.TextBlocks))

	formulaCount := 0
	for _, block := range content.TextBlocks {
		if block.IsFormula {
			formulaCount++
		}
	}
	fmt.Fprintf(file, "  普通文本块: %d\n", len(content.TextBlocks)-formulaCount)
	fmt.Fprintf(file, "  数学公式块: %d\n\n", formulaCount)

	if len(content.Metadata) > 0 {
		fmt.Fprintf(file, "PDF元数据:\n")
		for key, value := range content.Metadata {
			fmt.Fprintf(file, "  %s: %s\n", key, value)
		}
		fmt.Fprintf(file, "\n")
	}

	fmt.Fprintf(file, "页面内容预览:\n")
	fmt.Fprintf(file, "==================\n\n")

	for i, pageText := range doc.PageTexts {
		fmt.Fprintf(file, "第 %d 页 (共 %d 字符):\n", i+1, len(pageText))
		fmt.Fprintf(file, "-------------------\n")

		if len(pageText) == 0 {
			fmt.Fprintf(file, "(空白页)\n\n")
		} else {
			preview := pageText
			if len(preview) > 500 {
				preview = preview[:500] + "\n...(内容已截断)..."
			}
			fmt.Fprintf(file, "%s\n\n", preview)
		}
	}

	return nil
}
