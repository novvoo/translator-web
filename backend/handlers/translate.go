package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"translator-web/middleware"
	"translator-web/models"
	"translator-web/translator"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TaskManager 管理所有用户的任务
type TaskManager struct {
	// sessionID -> taskID -> task
	userTasks map[string]map[string]*models.TranslateTask
	mu        sync.RWMutex
}

var taskManager *TaskManager

func init() {
	taskManager = &TaskManager{
		userTasks: make(map[string]map[string]*models.TranslateTask),
	}
}

// AddTask 为用户添加任务
func (tm *TaskManager) AddTask(sessionID string, task *models.TranslateTask) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.userTasks[sessionID] == nil {
		tm.userTasks[sessionID] = make(map[string]*models.TranslateTask)
	}
	tm.userTasks[sessionID][task.ID] = task
}

// GetTask 获取用户的特定任务
func (tm *TaskManager) GetTask(sessionID, taskID string) (*models.TranslateTask, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if userTasks, exists := tm.userTasks[sessionID]; exists {
		task, found := userTasks[taskID]
		return task, found
	}
	return nil, false
}

// GetUserTasks 获取用户的所有任务
func (tm *TaskManager) GetUserTasks(sessionID string) []*models.TranslateTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	userTasks, exists := tm.userTasks[sessionID]
	if !exists {
		return []*models.TranslateTask{}
	}

	tasks := make([]*models.TranslateTask, 0, len(userTasks))
	for _, task := range userTasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// UpdateTask 更新任务（用于更新进度等）
func (tm *TaskManager) UpdateTask(sessionID, taskID string, updateFn func(*models.TranslateTask)) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if userTasks, exists := tm.userTasks[sessionID]; exists {
		if task, found := userTasks[taskID]; found {
			updateFn(task)
		}
	}
}

// TranslateHandler 处理翻译请求
func TranslateHandler(c *gin.Context) {
	// 获取会话 ID
	sessionID := middleware.GetSessionID(c)
	if sessionID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的会话"})
		return
	}

	// 解析表单
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到上传文件"})
		return
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".epub" && ext != ".pdf" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .epub 和 .pdf 文件"})
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
	// 本地模型（Ollama、NLTranslator 等）不需要 API Key
	needsAPIKey := req.LLMConfig.Provider != "ollama" &&
		req.LLMConfig.Provider != "nltranslator"

	if needsAPIKey && req.LLMConfig.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API Key 不能为空"})
		return
	}

	// 创建任务
	taskID := uuid.New().String()
	task := &models.TranslateTask{
		ID:             taskID,
		SessionID:      sessionID,
		SourceFile:     file.Filename,
		TargetLanguage: req.TargetLanguage,
		Status:         "pending",
		Progress:       0,
		CreatedAt:      time.Now(),
	}

	// 添加到任务管理器
	taskManager.AddTask(sessionID, task)

	// 为用户创建独立的目录
	userDir := filepath.Join("data", "users", sessionID)
	uploadDir := filepath.Join(userDir, "uploads")
	os.MkdirAll(uploadDir, 0755)

	// 根据文件类型确定保存路径
	sourcePath := filepath.Join(uploadDir, taskID+ext)
	if err := c.SaveUploadedFile(file, sourcePath); err != nil {
		taskManager.UpdateTask(sessionID, taskID, func(t *models.TranslateTask) {
			t.Status = "failed"
			t.Error = "保存文件失败: " + err.Error()
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败: " + err.Error()})
		return
	}

	// 启动后台翻译任务
	go processTranslation(sessionID, taskID, sourcePath, req)

	c.JSON(http.StatusOK, gin.H{
		"taskId":  taskID,
		"message": "翻译任务已创建",
	})
}

// processTranslation 处理翻译任务
func processTranslation(sessionID, taskID, sourcePath string, req models.TranslateRequest) {
	taskManager.UpdateTask(sessionID, taskID, func(t *models.TranslateTask) {
		t.Status = "processing"
	})

	log.Printf("[会话 %s][任务 %s] 开始处理翻译", sessionID[:8], taskID)

	defer func() {
		if r := recover(); r != nil {
			taskManager.UpdateTask(sessionID, taskID, func(t *models.TranslateTask) {
				t.Status = "failed"
				t.Error = fmt.Sprintf("翻译过程出错: %v", r)
			})
			log.Printf("[会话 %s][任务 %s] 翻译失败（panic）: %v", sessionID[:8], taskID, r)
		}
	}()

	// 为每个用户创建独立的缓存目录
	userCacheDir := filepath.Join("data", "users", sessionID, "cache")
	os.MkdirAll(userCacheDir, 0755)

	log.Printf("[会话 %s][任务 %s] 创建翻译客户端，提供商: %s, 模型: %s", sessionID[:8], taskID, req.LLMConfig.Provider, req.LLMConfig.Model)
	cache, _ := translator.NewCache(userCacheDir)

	// 如果强制重新翻译，禁用缓存读取（但仍然写入缓存）
	if req.ForceRetranslate {
		log.Printf("[会话 %s][任务 %s] 强制重新翻译模式：将忽略现有缓存", sessionID[:8], taskID)
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

	// 创建统一文档翻译器
	docTranslator, err := translator.NewDocumentTranslator(providerConfig, cache)
	if err != nil {
		taskManager.UpdateTask(sessionID, taskID, func(t *models.TranslateTask) {
			t.Status = "failed"
			t.Error = "创建翻译客户端失败: " + err.Error()
		})
		log.Printf("[会话 %s][任务 %s] 创建客户端失败: %v", sessionID[:8], taskID, err)
		return
	}

	// 确定输出路径
	userOutputDir := filepath.Join("data", "users", sessionID, "outputs")
	os.MkdirAll(userOutputDir, 0755)

	ext := strings.ToLower(filepath.Ext(sourcePath))
	var outputPath string
	if ext == ".pdf" {
		// PDF 默认输出为 HTML 文件（更好的格式）
		outputPath = filepath.Join(userOutputDir, taskID+".html")
	} else {
		// EPUB 保持原格式
		outputPath = filepath.Join(userOutputDir, taskID+ext)
	}

	// 进度回调函数
	progressCallback := func(progress float64) {
		taskManager.UpdateTask(sessionID, taskID, func(t *models.TranslateTask) {
			t.Progress = progress
		})
	}

	// 执行翻译
	log.Printf("[会话 %s][任务 %s] 开始翻译文档: %s", sessionID[:8], taskID, sourcePath)
	actualOutputPath, err := docTranslator.TranslateDocument(sourcePath, outputPath, req.TargetLanguage, req.UserPrompt, progressCallback)
	if err != nil {
		taskManager.UpdateTask(sessionID, taskID, func(t *models.TranslateTask) {
			t.Status = "failed"
			t.Error = "翻译失败: " + err.Error()
		})
		log.Printf("[会话 %s][任务 %s] 翻译失败: %v", sessionID[:8], taskID, err)
		return
	}

	// 翻译完成
	taskManager.UpdateTask(sessionID, taskID, func(t *models.TranslateTask) {
		t.Status = "completed"
		t.Progress = 1.0
		t.CompletedAt = time.Now()
		t.OutputPath = actualOutputPath // 使用实际的输出路径
	})

	log.Printf("[会话 %s][任务 %s] 翻译完成: %s", sessionID[:8], taskID, actualOutputPath)
}

// GetStatusHandler 获取任务状态
func GetStatusHandler(c *gin.Context) {
	sessionID := middleware.GetSessionID(c)
	if sessionID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的会话"})
		return
	}

	taskID := c.Param("taskId")

	task, exists := taskManager.GetTask(sessionID, taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在或无权访问"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// DownloadHandler 下载翻译后的文件
func DownloadHandler(c *gin.Context) {
	sessionID := middleware.GetSessionID(c)
	if sessionID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的会话"})
		return
	}

	taskID := c.Param("taskId")

	task, exists := taskManager.GetTask(sessionID, taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在或无权访问"})
		return
	}

	if task.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务未完成"})
		return
	}

	// 设置下载文件名（根据实际输出文件类型）
	outputExt := strings.ToLower(filepath.Ext(task.OutputPath))
	sourceExt := strings.ToLower(filepath.Ext(task.SourceFile))

	var filename string
	if sourceExt == ".pdf" && outputExt == ".html" {
		// PDF 翻译输出为 HTML 格式
		baseName := strings.TrimSuffix(task.SourceFile, filepath.Ext(task.SourceFile))
		filename = "translated_" + baseName + ".html"
	} else {
		// 其他情况保持原扩展名
		filename = "translated_" + task.SourceFile
	}

	c.FileAttachment(task.OutputPath, filename)
}

// GetTasksHandler 获取当前用户的所有任务
func GetTasksHandler(c *gin.Context) {
	sessionID := middleware.GetSessionID(c)
	if sessionID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的会话"})
		return
	}

	taskList := taskManager.GetUserTasks(sessionID)

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
