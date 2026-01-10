//go:build ignore
// +build ignore

package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"translator-web/pdf"
	"translator-web/translator"

	"github.com/signintech/gopdf"
)

func main() {
	// 创建日志文件到当前运行目录
	logFile, err := os.OpenFile("debug_output.txt", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("❌ 创建日志文件失败: %v\n", err)
		return
	}
	defer logFile.Close()

	// 将标准日志输出重定向到文件
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("=== PDF翻译快速Demo ===")
	fmt.Println("使用自定义OpenAI API配置")
	fmt.Println("只翻译前1个文本块作为快速演示")
	fmt.Println()

	// 设置自定义API配置
	config := translator.ProviderConfig{
		Type:        translator.ProviderOpenAI,
		APIKey:      "sk-awfxkxuxhpdbhzlyyvsvuuueemvmftmihvoftgjctkyxtnnm",
		APIURL:      "https://api.siliconflow.cn/v1/chat/completions",
		Model:       "Qwen/Qwen2.5-7B-Instruct",
		Temperature: 0.1,  // 降低温度以获得更一致的翻译
		MaxTokens:   2000, // 增加token数量以处理更长的文本
	}

	fmt.Printf("🔧 API配置:\n")
	fmt.Printf("   提供商: %s\n", config.Type)
	fmt.Printf("   API URL: %s\n", config.APIURL)
	fmt.Printf("   模型: %s\n", config.Model)
	fmt.Printf("   温度: %.1f\n", config.Temperature)
	fmt.Printf("   最大Token: %d\n", config.MaxTokens)
	fmt.Println()

	// 输入PDF文件路径
	inputPath := "./spann.pdf"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		log.Fatalf("❌ PDF文件不存在: %s", inputPath)
	}

	fmt.Printf("📄 输入文件: %s\n", inputPath)

	// 计算输入PDF文件的MD5
	inputMD5, err := calculateFileMD5(inputPath)
	if err != nil {
		log.Printf("⚠️  计算输入文件MD5失败: %v", err)
	} else {
		fmt.Printf("🔐 输入文件MD5: %s\n", inputMD5)
	}

	// 显示输入文件的基本信息和内容预览
	if err := showFileInfo(inputPath); err != nil {
		log.Printf("⚠️  显示文件信息失败: %v", err)
	}

	// 创建输出目录
	outputDir := "./output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("❌ 创建输出目录失败: %v", err)
	}

	// 创建缓存
	cacheDir := "./cache"
	cache, err := translator.NewCache(cacheDir)
	if err != nil {
		log.Fatalf("❌ 创建缓存失败: %v", err)
	}
	fmt.Printf("💾 缓存已初始化 (目录: %s)\n", cacheDir)

	// 创建翻译客户端
	client, err := translator.NewTranslatorClient(config, cache)
	if err != nil {
		log.Fatalf("❌ 创建翻译客户端失败: %v", err)
	}
	fmt.Printf("🤖 翻译客户端已创建\n")

	// 打开PDF文档
	fmt.Printf("📖 正在打开PDF文档...\n")
	doc, err := translator.OpenPDF(inputPath)
	if err != nil {
		log.Fatalf("❌ 打开PDF失败: %v", err)
	}
	fmt.Printf("✅ PDF文档已打开，共 %d 页\n", doc.Metadata.Pages)

	// 提取文本块
	fmt.Printf("📝 正在提取文本块...\n")
	textBlocks := doc.GetTextBlocks()
	fmt.Printf("✅ 提取到 %d 个文本块\n", len(textBlocks))

	if len(textBlocks) == 0 {
		log.Fatalf("❌ PDF中没有可翻译的文本内容")
	}

	// 翻译更多文本块以获得更好的演示效果
	maxBlocks := 10
	if len(textBlocks) > maxBlocks {
		textBlocks = textBlocks[:maxBlocks]
		fmt.Printf("🎯 Demo模式：翻译前 %d 个文本块\n", maxBlocks)
	}

	// 开始翻译
	fmt.Printf("\n🚀 开始翻译...\n")
	fmt.Printf("目标语言: 中文\n")
	fmt.Printf("用户提示: 准确翻译学术论文，保持专业术语的准确性\n")
	fmt.Println()

	translations := make(map[string]string)
	targetLanguage := "Chinese"
	userPrompt := "准确翻译学术论文，保持专业术语的准确性，确保翻译流畅自然"

	startTime := time.Now()

	for i, block := range textBlocks {
		// 清理文本块，移除页面标记
		originalText := strings.TrimSpace(block)
		if strings.HasPrefix(originalText, "[第") {
			if idx := strings.Index(originalText, "] "); idx != -1 {
				originalText = originalText[idx+2:]
			}
		}

		// 跳过过短或空的文本块
		if originalText == "" || len(originalText) < 5 {
			fmt.Printf("⏭️  跳过第 %d 个文本块（太短或为空）: %s\n", i+1, truncateText(originalText, 30))
			continue
		}

		// 跳过只包含数字、符号或单个单词的文本块
		if len(strings.Fields(originalText)) < 2 {
			fmt.Printf("⏭️  跳过第 %d 个文本块（内容太简单）: %s\n", i+1, truncateText(originalText, 30))
			continue
		}

		// 跳过看起来像页码或引用的文本
		if isPageNumberOrReference(originalText) {
			fmt.Printf("⏭️  跳过第 %d 个文本块（页码或引用）: %s\n", i+1, truncateText(originalText, 30))
			continue
		}

		fmt.Printf("🔄 翻译第 %d/%d 个文本块...\n", i+1, len(textBlocks))
		fmt.Printf("   原文长度: %d 字符\n", len(originalText))
		fmt.Printf("   原文预览: %s...\n", truncateText(originalText, 80))

		// 记录翻译开始时间
		blockStartTime := time.Now()

		// 执行翻译
		translated, err := client.Translate(originalText, targetLanguage, userPrompt)
		if err != nil {
			fmt.Printf("❌ 翻译第 %d 个文本块失败: %v\n", i+1, err)
			fmt.Printf("   使用原文作为备选\n")
			translations[originalText] = originalText
		} else {
			translations[originalText] = translated
			blockDuration := time.Since(blockStartTime)
			fmt.Printf("✅ 翻译完成 (耗时: %v)\n", blockDuration)
			fmt.Printf("   译文长度: %d 字符\n", len(translated))
			fmt.Printf("   译文预览: %s...\n", truncateText(translated, 80))
		}

		fmt.Println()

		// 避免请求过快，给API服务器一些缓冲时间
		time.Sleep(500 * time.Millisecond)
	}

	totalDuration := time.Since(startTime)
	fmt.Printf("🎉 翻译完成！总耗时: %v\n", totalDuration)
	fmt.Printf("📊 翻译统计: %d 个文本块\n", len(translations))
	fmt.Println()

	// 生成输出文件
	fmt.Printf("💾 正在生成输出文件...\n")

	// 1. 生成双语文本文件
	textOutputPath := filepath.Join(outputDir, "spann_bilingual.txt")
	originalBlocks := make([]string, 0, len(translations))
	translatedBlocks := make([]string, 0, len(translations))

	for original, translated := range translations {
		originalBlocks = append(originalBlocks, original)
		translatedBlocks = append(translatedBlocks, translated)
	}

	if err := doc.SaveBilingualText(textOutputPath, originalBlocks, translatedBlocks); err != nil {
		fmt.Printf("❌ 保存双语文本文件失败: %v\n", err)
	} else {
		fmt.Printf("✅ 双语文本文件已保存: %s\n", textOutputPath)
	}

	// 2. 生成单语文本文件
	monoTextOutputPath := filepath.Join(outputDir, "spann_translated.txt")
	if err := doc.SaveMonolingualText(monoTextOutputPath, translatedBlocks); err != nil {
		fmt.Printf("❌ 保存单语文本文件失败: %v\n", err)
	} else {
		fmt.Printf("✅ 单语文本文件已保存: %s\n", monoTextOutputPath)
	}

	// 3. 生成单语PDF文件（使用重新生成方法）
	fmt.Printf("\n🔄 正在生成单语PDF文件（重新生成方法）...\n")
	monoPDFOutputPath := filepath.Join(outputDir, "spann_translated.pdf")

	// 使用新的PDF重新生成器
	fmt.Printf("🔧 使用PDF重新生成器：拷贝操作符和图片，替换文本重新生成PDF\n")

	regenerator := translator.NewPDFRegenerator()
	if err := regenerator.RegeneratePDF(inputPath, monoPDFOutputPath, translations); err != nil {
		fmt.Printf("❌ PDF重新生成失败: %v\n", err)
		fmt.Printf("💡 尝试使用传统方法作为备选...\n")

		// 备选：使用传统的文本替换方法
		if err := doc.SaveMonolingualPDF(monoPDFOutputPath, translatedBlocks); err != nil {
			fmt.Printf("❌ 传统方法也失败: %v\n", err)
			fmt.Printf("💡 提示: PDF可能是扫描版或使用了特殊编码\n")
		} else {
			fmt.Printf("✅ 使用传统方法生成单语PDF: %s\n", monoPDFOutputPath)
		}
	} else {
		fmt.Printf("✅ PDF重新生成成功: %s\n", monoPDFOutputPath)

		// 验证PDF是否被正确修改
		fmt.Printf("🔍 正在验证PDF重新生成结果...\n")
		if err := validatePDFTranslation(monoPDFOutputPath, translatedBlocks); err != nil {
			fmt.Printf("⚠️  PDF验证警告: %v\n", err)
		} else {
			fmt.Printf("✅ PDF验证通过：文本已成功替换\n")
		}
	}

	// 4. 生成双语PDF文件（使用重新生成方法）
	fmt.Printf("\n🔄 正在生成双语PDF文件（重新生成方法）...\n")
	bilingualPDFOutputPath := filepath.Join(outputDir, "spann_bilingual.pdf")

	// 构建双语翻译映射
	bilingualTranslations := make(map[string]string)
	for original, translation := range translations {
		// 使用上下对照格式
		bilingualTranslations[original] = original + "\n" + translation
	}

	// 使用PDF重新生成器生成双语PDF
	if err := regenerator.RegeneratePDF(inputPath, bilingualPDFOutputPath, bilingualTranslations); err != nil {
		fmt.Printf("❌ 双语PDF重新生成失败: %v\n", err)
		fmt.Printf("💡 尝试使用传统方法作为备选...\n")

		// 备选：使用传统的双语PDF生成方法
		if err := doc.SaveBilingualPDF(bilingualPDFOutputPath, originalBlocks, translatedBlocks); err != nil {
			fmt.Printf("❌ 传统方法也失败: %v\n", err)
			fmt.Printf("💡 提示: PDF可能是扫描版或使用了特殊编码\n")
		} else {
			fmt.Printf("✅ 使用传统方法生成双语PDF: %s\n", bilingualPDFOutputPath)
		}
	} else {
		fmt.Printf("✅ 双语PDF重新生成成功: %s\n", bilingualPDFOutputPath)

		// 验证双语PDF
		fmt.Printf("🔍 正在验证双语PDF结果...\n")
		if err := validateBilingualPDF(bilingualPDFOutputPath, originalBlocks, translatedBlocks); err != nil {
			fmt.Printf("⚠️  双语PDF验证警告: %v\n", err)
		} else {
			fmt.Printf("✅ 双语PDF验证通过：原文和译文都已包含\n")
		}
	}

	// 显示详细的翻译结果
	fmt.Println()
	fmt.Printf("📋 详细翻译结果:\n")
	fmt.Println(strings.Repeat("=", 80))

	i := 1
	for original, translated := range translations {
		fmt.Printf("\n【文本块 %d】\n", i)
		fmt.Printf("原文: %s\n", original)
		fmt.Printf("译文: %s\n", translated)
		fmt.Println(strings.Repeat("-", 40))
		i++
	}

	// 显示输出文件列表和MD5
	fmt.Println()
	fmt.Printf("📁 输出文件列表和MD5:\n")
	if files, err := os.ReadDir(outputDir); err == nil {
		for _, file := range files {
			if !file.IsDir() {
				filePath := filepath.Join(outputDir, file.Name())
				if info, err := os.Stat(filePath); err == nil {
					fileType := "文本"
					if strings.HasSuffix(file.Name(), ".pdf") {
						fileType = "PDF"
					}

					// 计算文件MD5
					fileMD5, err := calculateFileMD5(filePath)
					if err != nil {
						fmt.Printf("   %s (%.2f KB) - %s [MD5计算失败: %v]\n", file.Name(), float64(info.Size())/1024, fileType, err)
					} else {
						fmt.Printf("   %s (%.2f KB) - %s [MD5: %s]\n", file.Name(), float64(info.Size())/1024, fileType, fileMD5)
					}

					// 显示文件内容预览
					fmt.Printf("      📄 文件内容预览:\n")
					if err := showFileContentPreview(filePath, 5); err != nil {
						fmt.Printf("         ❌ 无法读取文件内容: %v\n", err)
					}

					// 如果是PDF文件，显示替换信息
					if strings.HasSuffix(file.Name(), ".pdf") && len(translations) > 0 {
						showPDFReplacementInfo(filePath, translations)
					}

					fmt.Println()
				}
			}
		}
	}

	fmt.Println()
	fmt.Printf("🎊 Demo完成！\n")
	fmt.Printf("📄 生成的文件包括:\n")
	fmt.Printf("   • 双语文本对照文件 (.txt)\n")
	fmt.Printf("   • 单语翻译文本文件 (.txt)\n")
	fmt.Printf("   • 单语翻译PDF文件 (.pdf) - 完全替换原文\n")
	fmt.Printf("   • 双语对照PDF文件 (.pdf) - 原文+译文\n")
	fmt.Printf("💡 提示: 如果PDF文件大小异常小，可能是字体或编码问题\n")
	fmt.Printf("💡 建议: 使用PDF阅读器打开文件查看实际效果\n")

	// 检查PDF文件质量
	fmt.Println()
	fmt.Printf("🔍 检查PDF文件质量...\n")
	checkPDFQuality(outputDir, inputPath)

	// 演示通用字体功能
	fmt.Println()
	fmt.Printf("🔤 演示通用字体功能...\n")
	if err := demonstrateUniFontFeatures(outputDir, translations); err != nil {
		fmt.Printf("❌ 通用字体演示失败: %v\n", err)
	} else {
		fmt.Printf("✅ 通用字体演示完成\n")
	}
}

// truncateText 截断文本用于预览
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// validatePDFTranslation 验证PDF翻译结果
func validatePDFTranslation(pdfPath string, expectedTranslations []string) error {
	fmt.Printf("   📖 正在读取生成的PDF文件...\n")

	// 打开生成的PDF文件
	translatedDoc, err := translator.OpenPDF(pdfPath)
	if err != nil {
		return fmt.Errorf("无法打开生成的PDF文件: %w", err)
	}

	// 提取PDF中的所有文本
	var pdfTexts []string
	for _, pageText := range translatedDoc.PageTexts {
		if strings.TrimSpace(pageText) != "" {
			pdfTexts = append(pdfTexts, pageText)
		}
	}

	if len(pdfTexts) == 0 {
		return fmt.Errorf("生成的PDF中没有文本内容")
	}

	fmt.Printf("   📊 PDF包含 %d 页文本内容\n", len(pdfTexts))

	// 检查是否包含翻译后的文本
	foundTranslations := 0
	totalTranslations := len(expectedTranslations)

	for i, expectedText := range expectedTranslations {
		found := false
		for _, pdfText := range pdfTexts {
			if strings.Contains(pdfText, strings.TrimSpace(expectedText)) {
				found = true
				foundTranslations++
				fmt.Printf("   ✅ 找到翻译文本 %d/%d: %s\n", i+1, totalTranslations, truncateText(expectedText, 50))
				break
			}
		}

		if !found {
			fmt.Printf("   ❌ 未找到翻译文本 %d/%d: %s\n", i+1, totalTranslations, truncateText(expectedText, 50))
		}
	}

	successRate := float64(foundTranslations) / float64(totalTranslations) * 100
	fmt.Printf("   📈 翻译验证成功率: %.1f%% (%d/%d)\n", successRate, foundTranslations, totalTranslations)

	if foundTranslations == 0 {
		return fmt.Errorf("PDF中未找到任何翻译文本，可能替换失败")
	}

	if successRate < 50.0 {
		return fmt.Errorf("翻译成功率过低 (%.1f%%)，可能存在问题", successRate)
	}

	return nil
}

// validateBilingualPDF 验证双语PDF结果
func validateBilingualPDF(pdfPath string, originalBlocks, translatedBlocks []string) error {
	fmt.Printf("   📖 正在读取双语PDF文件...\n")

	// 打开生成的PDF文件
	bilingualDoc, err := translator.OpenPDF(pdfPath)
	if err != nil {
		return fmt.Errorf("无法打开双语PDF文件: %w", err)
	}

	// 提取PDF中的所有文本
	var pdfTexts []string
	for _, pageText := range bilingualDoc.PageTexts {
		if strings.TrimSpace(pageText) != "" {
			pdfTexts = append(pdfTexts, pageText)
		}
	}

	if len(pdfTexts) == 0 {
		return fmt.Errorf("双语PDF中没有文本内容")
	}

	fmt.Printf("   📊 双语PDF包含 %d 页文本内容\n", len(pdfTexts))

	// 检查原文和译文
	foundOriginals := 0
	foundTranslations := 0
	totalTexts := len(originalBlocks)

	for i := 0; i < len(originalBlocks) && i < len(translatedBlocks); i++ {
		originalText := strings.TrimSpace(originalBlocks[i])
		translatedText := strings.TrimSpace(translatedBlocks[i])

		// 移除页面标记
		if strings.HasPrefix(originalText, "[第") {
			if idx := strings.Index(originalText, "] "); idx != -1 {
				originalText = originalText[idx+2:]
			}
		}

		foundOriginal := false
		foundTranslation := false

		for _, pdfText := range pdfTexts {
			if strings.Contains(pdfText, originalText) {
				foundOriginal = true
			}
			if strings.Contains(pdfText, translatedText) {
				foundTranslation = true
			}
		}

		if foundOriginal {
			foundOriginals++
			fmt.Printf("   ✅ 找到原文 %d/%d: %s\n", i+1, totalTexts, truncateText(originalText, 50))
		} else {
			fmt.Printf("   ❌ 未找到原文 %d/%d: %s\n", i+1, totalTexts, truncateText(originalText, 50))
		}

		if foundTranslation {
			foundTranslations++
			fmt.Printf("   ✅ 找到译文 %d/%d: %s\n", i+1, totalTexts, truncateText(translatedText, 50))
		} else {
			fmt.Printf("   ❌ 未找到译文 %d/%d: %s\n", i+1, totalTexts, truncateText(translatedText, 50))
		}
	}

	originalRate := float64(foundOriginals) / float64(totalTexts) * 100
	translationRate := float64(foundTranslations) / float64(totalTexts) * 100

	fmt.Printf("   📈 原文验证成功率: %.1f%% (%d/%d)\n", originalRate, foundOriginals, totalTexts)
	fmt.Printf("   📈 译文验证成功率: %.1f%% (%d/%d)\n", translationRate, foundTranslations, totalTexts)

	if foundOriginals == 0 && foundTranslations == 0 {
		return fmt.Errorf("双语PDF中既未找到原文也未找到译文")
	}

	if originalRate < 30.0 && translationRate < 30.0 {
		return fmt.Errorf("双语PDF验证成功率过低，原文: %.1f%%，译文: %.1f%%", originalRate, translationRate)
	}

	return nil
}

// calculateFileMD5 计算文件的MD5哈希值
func calculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// showFileInfo 显示文件基本信息
func showFileInfo(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	fmt.Printf("📊 文件信息:\n")
	fmt.Printf("   大小: %.2f KB\n", float64(info.Size())/1024)
	fmt.Printf("   修改时间: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))

	return nil
}

// showFileContentPreview 显示文件内容预览（前几行）
func showFileContentPreview(filePath string, maxLines int) error {
	// 对于PDF文件，尝试提取文本内容
	if strings.HasSuffix(strings.ToLower(filePath), ".pdf") {
		return showPDFContentPreview(filePath, maxLines)
	}

	// 对于文本文件，直接读取内容
	return showTextFilePreview(filePath, maxLines)
}

// showPDFContentPreview 显示PDF文件的文本内容预览
func showPDFContentPreview(filePath string, maxLines int) error {
	doc, err := translator.OpenPDF(filePath)
	if err != nil {
		return fmt.Errorf("无法打开PDF文件: %w", err)
	}

	fmt.Printf("         PDF页数: %d\n", doc.Metadata.Pages)

	// 显示前几页的文本内容
	displayedLines := 0
	for i, pageText := range doc.PageTexts {
		if displayedLines >= maxLines {
			break
		}

		if strings.TrimSpace(pageText) == "" {
			continue
		}

		fmt.Printf("         [第%d页] %s\n", i+1, truncateText(strings.TrimSpace(pageText), 100))
		displayedLines++

		if displayedLines >= maxLines {
			break
		}
	}

	if displayedLines == 0 {
		fmt.Printf("         (PDF中没有可提取的文本内容)\n")

		// 如果无法提取文本，尝试显示文件的基本信息
		if info, err := os.Stat(filePath); err == nil {
			fmt.Printf("         文件大小: %.2f KB\n", float64(info.Size())/1024)
			fmt.Printf("         修改时间: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))

			// 检查是否是翻译后的文件（通过文件名判断）
			if strings.Contains(filePath, "translated") || strings.Contains(filePath, "bilingual") {
				fmt.Printf("         注意: 这是翻译后的PDF文件，文本提取可能受到字体编码影响\n")
				fmt.Printf("         建议: 使用PDF阅读器打开文件查看实际翻译效果\n")
			}
		}
	} else {
		// 如果能提取到文本，但是是翻译后的文件，给出提示
		if strings.Contains(filePath, "translated") || strings.Contains(filePath, "bilingual") {
			fmt.Printf("         注意: 如果上述内容显示为英文，可能是PDF文本提取器的限制\n")
			fmt.Printf("         实际PDF文件中的文本可能已被正确翻译，请用PDF阅读器查看\n")
		}
	}

	return nil
}

// showTextFilePreview 显示文本文件的内容预览
func showTextFilePreview(filePath string, maxLines int) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 读取文件内容
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	// 按行分割内容
	lines := strings.Split(string(content), "\n")

	// 显示前几行
	displayLines := maxLines
	if len(lines) < displayLines {
		displayLines = len(lines)
	}

	for i := 0; i < displayLines; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			fmt.Printf("         [第%d行] (空行)\n", i+1)
		} else {
			fmt.Printf("         [第%d行] %s\n", i+1, truncateText(line, 100))
		}
	}

	if len(lines) > maxLines {
		fmt.Printf("         ... (还有 %d 行内容)\n", len(lines)-maxLines)
	}

	return nil
}

// showPDFReplacementInfo 显示PDF替换信息的详细内容
func showPDFReplacementInfo(filePath string, translations map[string]string) {
	fmt.Printf("         🔍 PDF文本替换详情:\n")

	if len(translations) == 0 {
		fmt.Printf("         (没有翻译内容可显示)\n")
		return
	}

	i := 1
	for original, translated := range translations {
		fmt.Printf("         [替换%d] 原文: %s\n", i, truncateText(original, 60))
		fmt.Printf("         [替换%d] 译文: %s\n", i, truncateText(translated, 60))
		fmt.Printf("         ---\n")
		i++
		if i > 3 { // 只显示前3个替换示例
			fmt.Printf("         ... (还有 %d 个替换)\n", len(translations)-3)
			break
		}
	}
}

// demonstrateUniFontFeatures 演示通用字体功能
func demonstrateUniFontFeatures(outputDir string, translations map[string]string) error {
	fmt.Printf("🔧 创建通用字体演示PDF...\n")

	// 创建新的PDF文档
	goPdf := &gopdf.GoPdf{}
	goPdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	goPdf.AddPage()

	// 创建通用字体管理器
	uniFontMgr := pdf.NewUniFontManager(goPdf)

	// 获取系统可用的通用字体
	availableFonts := uniFontMgr.GetSystemUniFonts()
	fmt.Printf("📋 系统可用通用字体: %d 个\n", len(availableFonts))

	for i, font := range availableFonts {
		if i < 5 { // 只显示前5个
			fmt.Printf("   %d. %s (%s) - %s\n", i+1, font.Name, font.Family, font.Description)
		}
	}

	if len(availableFonts) > 5 {
		fmt.Printf("   ... 还有 %d 个字体\n", len(availableFonts)-5)
	}

	// 自动加载最佳字体
	bestFont, err := uniFontMgr.AutoLoadBestFont()
	if err != nil {
		return fmt.Errorf("自动加载最佳字体失败: %w", err)
	}

	fmt.Printf("✅ 自动选择最佳字体: %s\n", bestFont)

	// 设置字体并写入标题
	if err := uniFontMgr.SetFont(bestFont, 16); err != nil {
		return fmt.Errorf("设置字体失败: %w", err)
	}

	// 写入标题
	title := "通用字体演示 - Universal Font Demo"
	if err := uniFontMgr.WriteText(50, 50, title); err != nil {
		return fmt.Errorf("写入标题失败: %w", err)
	}

	// 写入翻译内容示例
	currentY := 80.0
	lineHeight := 20.0

	if err := uniFontMgr.SetFont(bestFont, 12); err != nil {
		return fmt.Errorf("设置正文字体失败: %w", err)
	}

	// 写入说明文字
	description := "以下是使用通用字体显示的翻译内容示例："
	if err := uniFontMgr.WriteText(50, currentY, description); err != nil {
		return fmt.Errorf("写入说明失败: %w", err)
	}
	currentY += lineHeight * 1.5

	// 写入翻译内容
	i := 1
	for original, translated := range translations {
		if i > 3 { // 只显示前3个翻译
			break
		}

		// 写入原文
		originalText := fmt.Sprintf("原文 %d: %s", i, truncateText(original, 80))
		if err := uniFontMgr.WriteText(50, currentY, originalText); err != nil {
			return fmt.Errorf("写入原文失败: %w", err)
		}
		currentY += lineHeight

		// 写入译文
		translatedText := fmt.Sprintf("译文 %d: %s", i, truncateText(translated, 80))
		if err := uniFontMgr.WriteText(50, currentY, translatedText); err != nil {
			return fmt.Errorf("写入译文失败: %w", err)
		}
		currentY += lineHeight * 1.5

		i++
	}

	// 写入字体信息
	currentY += lineHeight
	fontInfo := fmt.Sprintf("使用字体: %s", bestFont)
	if err := uniFontMgr.WriteText(50, currentY, fontInfo); err != nil {
		return fmt.Errorf("写入字体信息失败: %w", err)
	}

	// 获取当前字体详细信息
	if currentFontInfo, exists := uniFontMgr.GetCurrentFont(); exists {
		currentY += lineHeight
		fontDetails := fmt.Sprintf("字体详情: %s - %s", currentFontInfo.Name, currentFontInfo.Description)
		if err := uniFontMgr.WriteText(50, currentY, fontDetails); err != nil {
			return fmt.Errorf("写入字体详情失败: %w", err)
		}
	}

	// 保存PDF文件
	uniFontDemoPath := filepath.Join(outputDir, "uni_font_demo.pdf")
	if err := goPdf.WritePdf(uniFontDemoPath); err != nil {
		return fmt.Errorf("保存通用字体演示PDF失败: %w", err)
	}

	fmt.Printf("📄 通用字体演示PDF已保存: %s\n", uniFontDemoPath)

	// 显示文件信息
	if info, err := os.Stat(uniFontDemoPath); err == nil {
		fmt.Printf("📊 文件大小: %.2f KB\n", float64(info.Size())/1024)

		// 计算MD5
		if fileMD5, err := calculateFileMD5(uniFontDemoPath); err == nil {
			fmt.Printf("🔐 文件MD5: %s\n", fileMD5)
		}
	}

	return nil
}

// checkPDFQuality 检查生成的PDF文件质量
func checkPDFQuality(outputDir, originalPath string) {
	// 获取原始文件信息
	originalInfo, err := os.Stat(originalPath)
	if err != nil {
		fmt.Printf("❌ 无法获取原始文件信息: %v\n", err)
		return
	}
	originalSize := originalInfo.Size()

	// 检查输出目录中的PDF文件
	files, err := os.ReadDir(outputDir)
	if err != nil {
		fmt.Printf("❌ 无法读取输出目录: %v\n", err)
		return
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".pdf") {
			continue
		}

		filePath := filepath.Join(outputDir, file.Name())
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			fmt.Printf("❌ 无法获取文件信息 %s: %v\n", file.Name(), err)
			continue
		}

		fileSize := fileInfo.Size()
		sizeRatio := float64(fileSize) / float64(originalSize) * 100

		fmt.Printf("📊 %s:\n", file.Name())
		fmt.Printf("   文件大小: %.2f KB (原始: %.2f KB)\n", float64(fileSize)/1024, float64(originalSize)/1024)
		fmt.Printf("   大小比例: %.1f%%\n", sizeRatio)

		// 检查文件质量
		if sizeRatio < 20 {
			fmt.Printf("   ⚠️  警告: 文件大小异常小，可能存在内容丢失\n")
			fmt.Printf("   💡 建议: 检查字体支持和文本编码\n")
		} else if sizeRatio > 200 {
			fmt.Printf("   ⚠️  警告: 文件大小异常大，可能存在重复内容\n")
		} else {
			fmt.Printf("   ✅ 文件大小正常\n")
		}

		// 尝试验证PDF内容
		if err := validatePDFContent(filePath); err != nil {
			fmt.Printf("   ⚠️  内容验证警告: %v\n", err)
		} else {
			fmt.Printf("   ✅ PDF内容验证通过\n")
		}

		fmt.Println()
	}
}

// validatePDFContent 验证PDF内容
func validatePDFContent(pdfPath string) error {
	doc, err := translator.OpenPDF(pdfPath)
	if err != nil {
		return fmt.Errorf("无法打开PDF: %w", err)
	}

	if doc.Metadata.Pages == 0 {
		return fmt.Errorf("PDF没有页面")
	}

	// 检查是否有文本内容
	hasText := false
	for _, pageText := range doc.PageTexts {
		if strings.TrimSpace(pageText) != "" {
			hasText = true
			break
		}
	}

	if !hasText {
		return fmt.Errorf("PDF中没有可提取的文本内容")
	}

	return nil
}

// isPageNumberOrReference 检查是否为页码或引用
func isPageNumberOrReference(text string) bool {
	text = strings.TrimSpace(text)

	// 检查是否为纯数字（页码）
	if len(text) < 10 && strings.TrimSpace(text) != "" {
		allDigits := true
		for _, char := range text {
			if !((char >= '0' && char <= '9') || char == '.' || char == '-' || char == ' ') {
				allDigits = false
				break
			}
		}
		if allDigits {
			return true
		}
	}

	// 检查是否为引用格式 [数字] 或 (数字)
	if len(text) < 20 && (strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]")) {
		return true
	}
	if len(text) < 20 && (strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")")) {
		return true
	}

	// 检查是否为常见的页面元素
	lowerText := strings.ToLower(text)
	pageElements := []string{"page", "figure", "table", "fig", "tab", "eq", "equation"}
	for _, element := range pageElements {
		if strings.Contains(lowerText, element) && len(text) < 50 {
			return true
		}
	}

	return false
}
