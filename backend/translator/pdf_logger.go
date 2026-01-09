package translator

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PDFLogger PDF专用日志记录器
type PDFLogger struct {
	logFile   *os.File
	logger    *log.Logger
	workDir   string
	sessionID string
	mutex     sync.Mutex
	debugMode bool
}

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// NewPDFLogger 创建PDF日志记录器
func NewPDFLogger(workDir, sessionID string) (*PDFLogger, error) {
	return NewPDFLoggerWithConsole(workDir, sessionID, false) // 默认不输出到控制台
}

// NewPDFLoggerWithConsole 创建PDF日志记录器，可选择是否输出到控制台
func NewPDFLoggerWithConsole(workDir, sessionID string, enableConsole bool) (*PDFLogger, error) {
	// 创建日志目录 - 使用当前工作目录下的logs目录
	currentDir, _ := os.Getwd()
	logDir := filepath.Join(currentDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建日志文件
	timestamp := time.Now().Format("20060102_150405")
	logFileName := fmt.Sprintf("pdf_processing_%s_%s.log", sessionID, timestamp)
	logFilePath := filepath.Join(logDir, logFileName)

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}

	// 根据配置决定是否同时输出到控制台
	var writer io.Writer
	if enableConsole {
		// 创建多重写入器，同时写入文件和控制台
		writer = io.MultiWriter(logFile, os.Stdout)
	} else {
		// 只写入文件
		writer = logFile
	}
	logger := log.New(writer, "", log.LstdFlags|log.Lshortfile)

	pdfLogger := &PDFLogger{
		logFile:   logFile,
		logger:    logger,
		workDir:   workDir,
		sessionID: sessionID,
		debugMode: true, // 默认开启调试模式
	}

	pdfLogger.Info("PDF日志记录器已初始化", map[string]interface{}{
		"工作目录": workDir,
		"会话ID": sessionID,
		"日志文件": logFilePath,
	})

	return pdfLogger, nil
}

// SetDebugMode 设置调试模式
func (l *PDFLogger) SetDebugMode(enabled bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.debugMode = enabled
	l.Info("调试模式已更新", map[string]interface{}{
		"启用": enabled,
	})
}

// Debug 记录调试信息
func (l *PDFLogger) Debug(message string, data ...map[string]interface{}) {
	if !l.debugMode {
		return
	}
	l.log(LogLevelDebug, message, data...)
}

// Info 记录信息
func (l *PDFLogger) Info(message string, data ...map[string]interface{}) {
	l.log(LogLevelInfo, message, data...)
}

// Warn 记录警告
func (l *PDFLogger) Warn(message string, data ...map[string]interface{}) {
	l.log(LogLevelWarn, message, data...)
}

// Error 记录错误
func (l *PDFLogger) Error(message string, err error, data ...map[string]interface{}) {
	logData := make(map[string]interface{})
	if len(data) > 0 {
		for k, v := range data[0] {
			logData[k] = v
		}
	}
	if err != nil {
		logData["错误"] = err.Error()
	}
	l.log(LogLevelError, message, logData)
}

// log 内部日志记录方法
func (l *PDFLogger) log(level LogLevel, message string, data ...map[string]interface{}) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	levelStr := l.getLevelString(level)

	// 构建日志消息
	logMessage := fmt.Sprintf("[%s] %s", levelStr, message)

	// 添加数据字段
	if len(data) > 0 && data[0] != nil {
		for key, value := range data[0] {
			logMessage += fmt.Sprintf(" | %s: %v", key, value)
		}
	}

	l.logger.Println(logMessage)
}

// getLevelString 获取日志级别字符串
func (l *PDFLogger) getLevelString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogTextExtraction 记录文本提取过程
func (l *PDFLogger) LogTextExtraction(pageNum int, operator string, operands []string, extractedText string) {
	l.Debug("文本提取", map[string]interface{}{
		"页码":   pageNum,
		"操作符":  operator,
		"操作数":  fmt.Sprintf("%v", operands),
		"提取文本": l.truncateString(extractedText, 100),
		"文本长度": len(extractedText),
	})
}

// LogTranslation 记录翻译过程
func (l *PDFLogger) LogTranslation(pageNum int, elementID string, originalText, translatedText string) {
	l.Info("文本翻译", map[string]interface{}{
		"页码":   pageNum,
		"元素ID": elementID,
		"原文":   l.truncateString(originalText, 50),
		"译文":   l.truncateString(translatedText, 50),
		"原文长度": len(originalText),
		"译文长度": len(translatedText),
	})
}

// LogPageProcessing 记录页面处理进度
func (l *PDFLogger) LogPageProcessing(pageNum, totalPages int, textElements, imageElements, graphicsElements int) {
	l.Info("页面处理完成", map[string]interface{}{
		"页码":   pageNum,
		"总页数":  totalPages,
		"进度":   fmt.Sprintf("%.1f%%", float64(pageNum)/float64(totalPages)*100),
		"文本元素": textElements,
		"图像元素": imageElements,
		"图形元素": graphicsElements,
		"总元素数": textElements + imageElements + graphicsElements,
	})
}

// LogOperationTiming 记录操作耗时
func (l *PDFLogger) LogOperationTiming(operation string, duration time.Duration, data ...map[string]interface{}) {
	logData := map[string]interface{}{
		"操作": operation,
		"耗时": duration.String(),
		"毫秒": duration.Milliseconds(),
	}

	if len(data) > 0 && data[0] != nil {
		for k, v := range data[0] {
			logData[k] = v
		}
	}

	l.Info("操作耗时统计", logData)
}

// LogResourceExtraction 记录资源提取
func (l *PDFLogger) LogResourceExtraction(resourceType string, resourceName string, size int64) {
	l.Debug("资源提取", map[string]interface{}{
		"资源类型": resourceType,
		"资源名称": resourceName,
		"大小":   l.formatBytes(size),
	})
}

// LogError 记录详细错误信息
func (l *PDFLogger) LogError(operation string, err error, context map[string]interface{}) {
	logData := map[string]interface{}{
		"操作": operation,
		"错误": err.Error(),
	}

	if context != nil {
		for k, v := range context {
			logData[k] = v
		}
	}

	l.Error("操作失败", err, logData)
}

// LogMemoryUsage 记录内存使用情况
func (l *PDFLogger) LogMemoryUsage(operation string, beforeMB, afterMB float64) {
	l.Debug("内存使用", map[string]interface{}{
		"操作":  operation,
		"处理前": fmt.Sprintf("%.2f MB", beforeMB),
		"处理后": fmt.Sprintf("%.2f MB", afterMB),
		"变化":  fmt.Sprintf("%+.2f MB", afterMB-beforeMB),
	})
}

// LogFileOperation 记录文件操作
func (l *PDFLogger) LogFileOperation(operation, filePath string, size int64) {
	l.Debug("文件操作", map[string]interface{}{
		"操作":   operation,
		"文件路径": filePath,
		"大小":   l.formatBytes(size),
	})
}

// LogStatistics 记录统计信息
func (l *PDFLogger) LogStatistics(stats map[string]interface{}) {
	l.Info("处理统计", stats)
}

// SaveDebugData 保存调试数据到文件
func (l *PDFLogger) SaveDebugData(filename string, data interface{}) error {
	debugDir := filepath.Join(l.workDir, "debug")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("创建调试目录失败: %w", err)
	}

	debugFile := filepath.Join(debugDir, filename)

	var content []byte
	var err error

	switch v := data.(type) {
	case string:
		content = []byte(v)
	case []byte:
		content = v
	default:
		content = []byte(fmt.Sprintf("%+v", data))
	}

	if err = os.WriteFile(debugFile, content, 0644); err != nil {
		return fmt.Errorf("保存调试数据失败: %w", err)
	}

	l.Debug("调试数据已保存", map[string]interface{}{
		"文件": debugFile,
		"大小": l.formatBytes(int64(len(content))),
	})

	return nil
}

// Close 关闭日志记录器
func (l *PDFLogger) Close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// 直接记录日志，避免调用Info方法导致死锁
	if l.logger != nil {
		l.logger.Printf("[INFO] PDF日志记录器正在关闭 | 会话ID: %s", l.sessionID)
	}

	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// 辅助方法

// truncateString 截断字符串
func (l *PDFLogger) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// formatBytes 格式化字节数
func (l *PDFLogger) formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetLogFilePath 获取日志文件路径
func (l *PDFLogger) GetLogFilePath() string {
	if l.logFile != nil {
		return l.logFile.Name()
	}
	return ""
}

// GetWorkDir 获取工作目录
func (l *PDFLogger) GetWorkDir() string {
	return l.workDir
}
