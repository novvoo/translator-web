package handlers

import (
	"encoding/json"
	"epub-translator-web/models"
	"epub-translator-web/translator"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	tasks     = make(map[string]*models.TranslateTask)
	taskMutex sync.RWMutex
)

// TranslateHandler 处理翻译请求
func TranslateHandler(c *gin.Context) {
	// 解析表单
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到上传文件"})
		return
	}

	// 检查文件类型
	if filepath.Ext(file.Filename) != ".epub" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .epub 文件"})
		return
	}

	// 解析配置
	var req models.TranslateRequest
	req.TargetLanguage = c.PostForm("targetLanguage")
	req.UserPrompt = c.PostForm("userPrompt")
	req.ForceRetranslate = c.PostForm("forceRetranslate") == "true"

	// 解析 LLM 配置
	llmConfigStr := c.PostForm("llmConfig")
	if llmConfigStr != "" {
		if err := json.Unmarshal([]byte(llmConfigStr), &req.LLMConfig); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "LLM 配置格式错误: " + err.Error()})
			return
		}
	}

	// 验证必填字段
	if req.TargetLanguage == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目标语言不能为空"})
		return
	}
	if req.LLMConfig.Provider == "" {
		req.LLMConfig.Provider = "openai" // 默认使用 OpenAI
	}
	if req.LLMConfig.APIURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API URL 不能为空"})
		return
	}
	// 如果 Model 为空，尝试从 URL 中提取或使用默认值
	if req.LLMConfig.Model == "" {
		// 为不同提供商设置默认模型
		switch req.LLMConfig.Provider {
		case "openai":
			req.LLMConfig.Model = "gpt-3.5-turbo"
		case "claude":
			req.LLMConfig.Model = "claude-3-5-sonnet-20241022"
		case "gemini":
			req.LLMConfig.Model = "gemini-pro"
		case "deepseek":
			req.LLMConfig.Model = "deepseek-chat"
		case "ollama":
			req.LLMConfig.Model = "llama2"
		case "custom":
			// 自定义提供商允许空模型（某些 API 可能不需要）
			req.LLMConfig.Model = "default"
		default:
			req.LLMConfig.Model = "gpt-3.5-turbo"
		}
	}
	// Ollama 本地模型可能不需要 API Key
	if req.LLMConfig.Provider != "ollama" && req.LLMConfig.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API Key 不能为空"})
		return
	}

	// 创建任务
	taskID := uuid.New().String()
	task := &models.TranslateTask{
		ID:             taskID,
		SourceFile:     file.Filename,
		TargetLanguage: req.TargetLanguage,
		Status:         "pending",
		Progress:       0,
		CreatedAt:      time.Now(),
	}

	taskMutex.Lock()
	tasks[taskID] = task
	taskMutex.Unlock()

	// 保存上传文件
	uploadDir := "data/uploads"
	os.MkdirAll(uploadDir, 0755)
	sourcePath := filepath.Join(uploadDir, taskID+".epub")
	if err := c.SaveUploadedFile(file, sourcePath); err != nil {
		task.Status = "failed"
		task.Error = "保存文件失败: " + err.Error()
		c.JSON(http.StatusInternalServerError, gin.H{"error": task.Error})
		return
	}

	// 启动后台翻译任务
	go processTranslation(taskID, sourcePath, req)

	c.JSON(http.StatusOK, gin.H{
		"taskId":  taskID,
		"message": "翻译任务已创建",
	})
}

// processTranslation 处理翻译任务
func processTranslation(taskID, sourcePath string, req models.TranslateRequest) {
	taskMutex.Lock()
	task := tasks[taskID]
	task.Status = "processing"
	taskMutex.Unlock()

	log.Printf("[任务 %s] 开始处理翻译", taskID)

	defer func() {
		if r := recover(); r != nil {
			taskMutex.Lock()
			task.Status = "failed"
			task.Error = fmt.Sprintf("翻译过程出错: %v", r)
			taskMutex.Unlock()
			log.Printf("[任务 %s] 翻译失败（panic）: %v", taskID, r)
		}
	}()

	// 打开 EPUB 文件
	log.Printf("[任务 %s] 打开 EPUB 文件: %s", taskID, sourcePath)
	epub, err := translator.OpenEPUB(sourcePath)
	if err != nil {
		taskMutex.Lock()
		task.Status = "failed"
		task.Error = "打开 EPUB 文件失败: " + err.Error()
		taskMutex.Unlock()
		log.Printf("[任务 %s] 打开文件失败: %v", taskID, err)
		return
	}

	// 创建翻译客户端
	log.Printf("[任务 %s] 创建翻译客户端，提供商: %s, 模型: %s", taskID, req.LLMConfig.Provider, req.LLMConfig.Model)
	cache, _ := translator.NewCache("cache")

	// 如果强制重新翻译，禁用缓存读取（但仍然写入缓存）
	if req.ForceRetranslate {
		log.Printf("[任务 %s] 强制重新翻译模式：将忽略现有缓存", taskID)
		cache.DisableCache()
	}

	providerConfig := translator.ProviderConfig{
		Type:        translator.ProviderType(req.LLMConfig.Provider),
		APIKey:      req.LLMConfig.APIKey,
		APIURL:      req.LLMConfig.APIURL,
		Model:       req.LLMConfig.Model,
		Temperature: req.LLMConfig.Temperature,
		MaxTokens:   req.LLMConfig.MaxTokens,
		Extra:       req.LLMConfig.Extra,
	}

	llm, err := translator.NewTranslatorClient(providerConfig, cache)
	if err != nil {
		taskMutex.Lock()
		task.Status = "failed"
		task.Error = "创建翻译客户端失败: " + err.Error()
		taskMutex.Unlock()
		log.Printf("[任务 %s] 创建客户端失败: %v", taskID, err)
		return
	}
	llm.WithRetry(5, 2*time.Second)

	// 获取所有 HTML 文件
	htmlFiles := epub.GetHTMLFiles()
	log.Printf("[任务 %s] 找到 %d 个 HTML 文件", taskID, len(htmlFiles))
	if len(htmlFiles) == 0 {
		taskMutex.Lock()
		task.Status = "failed"
		task.Error = "EPUB 文件中没有找到内容"
		taskMutex.Unlock()
		log.Printf("[任务 %s] 没有找到 HTML 文件", taskID)
		return
	}

	// 翻译目录（TOC）
	log.Printf("[任务 %s] 翻译目录", taskID)
	toc, _ := translator.ParseTOC(epub)
	if toc != nil {
		translator.TranslateTOC(toc, llm, req.TargetLanguage, req.UserPrompt, cache)
		translator.WriteTOC(epub, toc)
	}

	// 翻译元数据
	log.Printf("[任务 %s] 翻译元数据", taskID)
	translator.TranslateMetadata(epub, llm, req.TargetLanguage, req.UserPrompt, cache)

	// 翻译每个 HTML 文件
	totalFiles := len(htmlFiles)
	log.Printf("[任务 %s] 开始翻译 %d 个 HTML 文件", taskID, totalFiles)

	for i, filename := range htmlFiles {
		log.Printf("[任务 %s] 翻译文件 %d/%d: %s", taskID, i+1, totalFiles, filename)

		// 解析 HTML
		content := epub.Files[filename]
		htmlContent, err := translator.ParseHTML(content)
		if err != nil {
			log.Printf("[任务 %s] 跳过文件 %s: 解析失败 - %v", taskID, filename, err)
			// 更新进度（跳过的文件也算完成）
			progress := float64(i+1) / float64(totalFiles)
			taskMutex.Lock()
			task.Progress = progress
			taskMutex.Unlock()
			continue
		}

		// 提取文本块
		textBlocks := translator.ExtractTextBlocks(htmlContent.Body)
		if len(textBlocks) == 0 {
			log.Printf("[任务 %s] 跳过文件 %s: 没有文本内容", taskID, filename)
			// 更新进度（跳过的文件也算完成）
			progress := float64(i+1) / float64(totalFiles)
			taskMutex.Lock()
			task.Progress = progress
			taskMutex.Unlock()
			continue
		}

		log.Printf("[任务 %s] 文件 %s 包含 %d 个文本块", taskID, filename, len(textBlocks))

		// 翻译文本块（带进度更新）
		translations := make([]string, len(textBlocks))
		for j, text := range textBlocks {
			if text == "" {
				translations[j] = ""
				continue
			}

			translated, err := llm.Translate(text, req.TargetLanguage, req.UserPrompt)
			if err != nil {
				taskMutex.Lock()
				task.Status = "failed"
				task.Error = fmt.Sprintf("翻译 %s 第 %d 段失败: %s", filename, j+1, err.Error())
				taskMutex.Unlock()
				log.Printf("[任务 %s] 翻译失败: %v", taskID, err)
				return
			}
			translations[j] = translated

			// 更新细粒度进度：当前文件内的进度 + 已完成文件的进度
			fileProgress := float64(j+1) / float64(len(textBlocks))
			totalProgress := (float64(i) + fileProgress) / float64(totalFiles)
			taskMutex.Lock()
			task.Progress = totalProgress
			taskMutex.Unlock()

			log.Printf("[任务 %s] 文件 %d/%d, 文本块 %d/%d (总进度: %.1f%%)",
				taskID, i+1, totalFiles, j+1, len(textBlocks), totalProgress*100)

			// 避免请求过快
			time.Sleep(100 * time.Millisecond)
		}

		// 创建翻译映射
		translationMap := make(map[string]string)
		for j, block := range textBlocks {
			translationMap[block] = translations[j]
		}

		// 插入翻译
		translatedHTML := translator.InsertTranslation(htmlContent.Body, translationMap)

		// 更新 EPUB 文件内容
		originalContent := string(content)
		bodyStart := 0
		bodyEnd := len(originalContent)

		if idx := []byte(originalContent); len(idx) > 0 {
			if start := findBodyStart(originalContent); start != -1 {
				bodyStart = start
			}
			if end := findBodyEnd(originalContent); end != -1 {
				bodyEnd = end
			}
		}

		newContent := originalContent[:bodyStart] + translatedHTML + originalContent[bodyEnd:]
		epub.Files[filename] = []byte(newContent)
	}

	// 保存翻译后的 EPUB
	log.Printf("[任务 %s] 保存翻译后的 EPUB", taskID)
	outputDir := "data/outputs"
	os.MkdirAll(outputDir, 0755)
	outputPath := filepath.Join(outputDir, taskID+".epub")

	if err := epub.SaveEPUB(outputPath); err != nil {
		taskMutex.Lock()
		task.Status = "failed"
		task.Error = "保存翻译文件失败: " + err.Error()
		taskMutex.Unlock()
		log.Printf("[任务 %s] 保存失败: %v", taskID, err)
		return
	}

	// 完成任务
	taskMutex.Lock()
	task.Status = "completed"
	task.Progress = 1.0
	task.CompletedAt = time.Now()
	task.OutputPath = outputPath
	taskMutex.Unlock()

	log.Printf("[任务 %s] 翻译完成！输出文件: %s", taskID, outputPath)
}

// GetStatusHandler 获取任务状态
func GetStatusHandler(c *gin.Context) {
	taskID := c.Param("taskId")

	taskMutex.RLock()
	task, exists := tasks[taskID]
	taskMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// DownloadHandler 下载翻译后的文件
func DownloadHandler(c *gin.Context) {
	taskID := c.Param("taskId")

	taskMutex.RLock()
	task, exists := tasks[taskID]
	taskMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务未完成"})
		return
	}

	// 设置下载文件名（使用 FileAttachment 方法，它会正确设置 Content-Disposition）
	filename := "translated_" + task.SourceFile
	c.FileAttachment(task.OutputPath, filename)
}

// GetTasksHandler 获取所有任务
func GetTasksHandler(c *gin.Context) {
	taskMutex.RLock()
	defer taskMutex.RUnlock()

	taskList := make([]*models.TranslateTask, 0, len(tasks))
	for _, task := range tasks {
		taskList = append(taskList, task)
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": taskList,
		"total": len(taskList),
	})
}

// 辅助函数：查找 body 开始位置
func findBodyStart(html string) int {
	start := 0
	for i := 0; i < len(html); i++ {
		if i+5 < len(html) && html[i:i+5] == "<body" {
			for j := i; j < len(html); j++ {
				if html[j] == '>' {
					return j + 1
				}
			}
		}
	}
	return start
}

// 辅助函数：查找 body 结束位置
func findBodyEnd(html string) int {
	for i := len(html) - 1; i >= 6; i-- {
		if html[i-6:i+1] == "</body>" {
			return i - 6
		}
	}
	return len(html)
}
