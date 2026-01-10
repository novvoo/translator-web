package translator

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"translator-web/pdf"

	"github.com/jung-kurt/gofpdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PDFFlowProcessor PDF流处理器 - 基于临时目录的动态PDF重建
type PDFFlowProcessor struct {
	workDir     string
	inputPath   string
	outputPath  string
	flowData    *PDFFlowData
	fontManager *FontManager
	uniFontMgr  *pdf.UniFontManager // 添加通用字体管理器
	logger      *PDFLogger
	sessionID   string
	UniFontName string // 添加通用字体名称字段
}

// PDFFlowData PDF流数据结构
type PDFFlowData struct {
	Metadata     PDFDocumentMetadata `json:"metadata"`
	Pages        []PDFPageFlow       `json:"pages"`
	Resources    PDFResourcesFlow    `json:"resources"`
	ProcessTime  time.Time           `json:"process_time"`
	OriginalSize int64               `json:"original_size"`
}

// PDFDocumentMetadata PDF文档元数据
type PDFDocumentMetadata struct {
	Title        string            `json:"title"`
	Author       string            `json:"author"`
	Subject      string            `json:"subject"`
	Creator      string            `json:"creator"`
	Producer     string            `json:"producer"`
	CreationDate time.Time         `json:"creation_date"`
	ModDate      time.Time         `json:"mod_date"`
	PageCount    int               `json:"page_count"`
	CustomProps  map[string]string `json:"custom_props"`
}

// PDFPageFlow 页面流数据
type PDFPageFlow struct {
	PageNumber       int                   `json:"page_number"`
	MediaBox         BoundingBox           `json:"media_box"`
	CropBox          *BoundingBox          `json:"crop_box,omitempty"`
	BleedBox         *BoundingBox          `json:"bleed_box,omitempty"`
	TrimBox          *BoundingBox          `json:"trim_box,omitempty"`
	ArtBox           *BoundingBox          `json:"art_box,omitempty"`
	Rotation         int                   `json:"rotation"`
	TextElements     []TextElementFlow     `json:"text_elements"`
	ImageElements    []ImageElementFlow    `json:"image_elements"`
	GraphicsElements []GraphicsElementFlow `json:"graphics_elements"`
	Annotations      []AnnotationFlow      `json:"annotations"`
	ContentStreams   []ContentStreamFlow   `json:"content_streams"`
}

// BoundingBox 边界框
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// TextElementFlow 文本元素流
type TextElementFlow struct {
	ID           string          `json:"id"`
	Content      string          `json:"content"`
	Position     PositionFlow    `json:"position"`
	Font         FontFlow        `json:"font"`
	Color        ColorFlow       `json:"color"`
	Transform    TransformMatrix `json:"transform"`
	BoundingBox  BoundingBox     `json:"bounding_box"`
	TextState    TextStateFlow   `json:"text_state"`
	IsFormula    bool            `json:"is_formula"`
	Language     string          `json:"language"`
	Confidence   float64         `json:"confidence"`
	OriginalOps  []string        `json:"original_ops"`
	Dependencies []string        `json:"dependencies"`
}

// PositionFlow 位置流信息
type PositionFlow struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Baseline  float64 `json:"baseline"`
	Leading   float64 `json:"leading"`
	WordSpace float64 `json:"word_space"`
	CharSpace float64 `json:"char_space"`
}

// FontFlow 字体流信息
type FontFlow struct {
	Name         string             `json:"name"`
	Size         float64            `json:"size"`
	Weight       string             `json:"weight"`
	Style        string             `json:"style"`
	Encoding     string             `json:"encoding"`
	Embedded     bool               `json:"embedded"`
	Subset       bool               `json:"subset"`
	FontFile     string             `json:"font_file,omitempty"`
	Metrics      FontMetrics        `json:"metrics"`
	CharWidths   map[rune]float64   `json:"char_widths"`
	KerningPairs map[string]float64 `json:"kerning_pairs"`
}

// FontMetrics 字体度量信息
type FontMetrics struct {
	Ascent     float64 `json:"ascent"`
	Descent    float64 `json:"descent"`
	LineHeight float64 `json:"line_height"`
	CapHeight  float64 `json:"cap_height"`
	XHeight    float64 `json:"x_height"`
}

// ColorFlow 颜色流信息
type ColorFlow struct {
	Space      string    `json:"space"`  // RGB, CMYK, Gray, etc.
	Values     []float64 `json:"values"` // Color values
	Alpha      float64   `json:"alpha"`  // Transparency
	Pattern    string    `json:"pattern,omitempty"`
	ColorSpace string    `json:"color_space,omitempty"`
}

// TransformMatrix 变换矩阵
type TransformMatrix struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
	C float64 `json:"c"`
	D float64 `json:"d"`
	E float64 `json:"e"`
	F float64 `json:"f"`
}

// TextStateFlow 文本状态流
type TextStateFlow struct {
	CharSpace  float64 `json:"char_space"`
	WordSpace  float64 `json:"word_space"`
	Scale      float64 `json:"scale"`
	Leading    float64 `json:"leading"`
	RenderMode int     `json:"render_mode"`
	Rise       float64 `json:"rise"`
	Knockout   bool    `json:"knockout"`
}

// ImageElementFlow 图像元素流
type ImageElementFlow struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Position         PositionFlow    `json:"position"`
	Size             SizeFlow        `json:"size"`
	Transform        TransformMatrix `json:"transform"`
	BoundingBox      BoundingBox     `json:"bounding_box"`
	Format           string          `json:"format"`
	ColorSpace       string          `json:"color_space"`
	BitsPerComponent int             `json:"bits_per_component"`
	Width            int             `json:"width"`
	Height           int             `json:"height"`
	DataSize         int64           `json:"data_size"`
	FilePath         string          `json:"file_path,omitempty"`
	Inline           bool            `json:"inline"`
	Mask             *ImageMask      `json:"mask,omitempty"`
}

// SizeFlow 尺寸流信息
type SizeFlow struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ImageMask 图像遮罩
type ImageMask struct {
	Type   string `json:"type"`
	Values []int  `json:"values"`
}

// GraphicsElementFlow 图形元素流
type GraphicsElementFlow struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"` // path, line, rect, circle, etc.
	Path        []PathCommand   `json:"path"`
	Style       GraphicsStyle   `json:"style"`
	Transform   TransformMatrix `json:"transform"`
	BoundingBox BoundingBox     `json:"bounding_box"`
	ClipPath    []PathCommand   `json:"clip_path,omitempty"`
}

// PathCommand 路径命令
type PathCommand struct {
	Command string    `json:"command"` // m, l, c, z, etc.
	Points  []float64 `json:"points"`
}

// GraphicsStyle 图形样式
type GraphicsStyle struct {
	StrokeColor ColorFlow `json:"stroke_color"`
	FillColor   ColorFlow `json:"fill_color"`
	LineWidth   float64   `json:"line_width"`
	LineCap     int       `json:"line_cap"`
	LineJoin    int       `json:"line_join"`
	MiterLimit  float64   `json:"miter_limit"`
	DashArray   []float64 `json:"dash_array"`
	DashPhase   float64   `json:"dash_phase"`
}

// AnnotationFlow 注释流
type AnnotationFlow struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Rect     BoundingBox `json:"rect"`
	Contents string      `json:"contents"`
	Author   string      `json:"author"`
	Subject  string      `json:"subject"`
	ModDate  time.Time   `json:"mod_date"`
}

// ContentStreamFlow 内容流
type ContentStreamFlow struct {
	ID           string         `json:"id"`
	StreamIndex  int            `json:"stream_index"`
	RawContent   string         `json:"raw_content"`
	ParsedOps    []PDFOperation `json:"parsed_ops"`
	Dependencies []string       `json:"dependencies"`
}

// PDFOperation PDF操作符
type PDFOperation struct {
	Operator string    `json:"operator"`
	Operands []string  `json:"operands"`
	Position int       `json:"position"`
	Context  OpContext `json:"context"`
}

// OpContext 操作符上下文
type OpContext struct {
	GraphicsState GraphicsState   `json:"graphics_state"`
	TextState     TextStateFlow   `json:"text_state"`
	Transform     TransformMatrix `json:"transform"`
}

// GraphicsState 图形状态
type GraphicsState struct {
	CTM         TransformMatrix `json:"ctm"`
	StrokeColor ColorFlow       `json:"stroke_color"`
	FillColor   ColorFlow       `json:"fill_color"`
	LineWidth   float64         `json:"line_width"`
	LineCap     int             `json:"line_cap"`
	LineJoin    int             `json:"line_join"`
	MiterLimit  float64         `json:"miter_limit"`
	DashArray   []float64       `json:"dash_array"`
	DashPhase   float64         `json:"dash_phase"`
	ClipPath    []PathCommand   `json:"clip_path"`
}

// PDFResourcesFlow PDF资源流
type PDFResourcesFlow struct {
	Fonts       map[string]FontResource       `json:"fonts"`
	Images      map[string]ImageResource      `json:"images"`
	XObjects    map[string]XObjectResource    `json:"xobjects"`
	ColorSpaces map[string]ColorSpaceResource `json:"color_spaces"`
	Patterns    map[string]PatternResource    `json:"patterns"`
	Shadings    map[string]ShadingResource    `json:"shadings"`
	ExtGStates  map[string]ExtGStateResource  `json:"ext_gstates"`
}

// FontResource 字体资源
type FontResource struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Subtype   string            `json:"subtype"`
	BaseFont  string            `json:"base_font"`
	Encoding  string            `json:"encoding"`
	FontFile  string            `json:"font_file,omitempty"`
	Embedded  bool              `json:"embedded"`
	Subset    bool              `json:"subset"`
	FirstChar int               `json:"first_char"`
	LastChar  int               `json:"last_char"`
	Widths    []float64         `json:"widths"`
	FontBBox  BoundingBox       `json:"font_bbox"`
	CharProcs map[string]string `json:"char_procs,omitempty"`
	ToUnicode string            `json:"to_unicode,omitempty"`
}

// ImageResource 图像资源
type ImageResource struct {
	Name             string      `json:"name"`
	Width            int         `json:"width"`
	Height           int         `json:"height"`
	BitsPerComponent int         `json:"bits_per_component"`
	ColorSpace       string      `json:"color_space"`
	Filter           []string    `json:"filter"`
	DecodeParms      interface{} `json:"decode_parms,omitempty"`
	Length           int64       `json:"length"`
	FilePath         string      `json:"file_path,omitempty"`
}

// XObjectResource XObject资源
type XObjectResource struct {
	Name     string          `json:"name"`
	Subtype  string          `json:"subtype"`
	BBox     BoundingBox     `json:"bbox"`
	Matrix   TransformMatrix `json:"matrix"`
	Content  string          `json:"content,omitempty"`
	FilePath string          `json:"file_path,omitempty"`
}

// ColorSpaceResource 颜色空间资源
type ColorSpaceResource struct {
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Components int       `json:"components"`
	Range      []float64 `json:"range,omitempty"`
	Gamma      float64   `json:"gamma,omitempty"`
	WhitePoint []float64 `json:"white_point,omitempty"`
	BlackPoint []float64 `json:"black_point,omitempty"`
	Matrix     []float64 `json:"matrix,omitempty"`
}

// PatternResource 图案资源
type PatternResource struct {
	Name     string          `json:"name"`
	Type     int             `json:"type"`
	BBox     BoundingBox     `json:"bbox"`
	XStep    float64         `json:"x_step"`
	YStep    float64         `json:"y_step"`
	Matrix   TransformMatrix `json:"matrix"`
	Content  string          `json:"content,omitempty"`
	FilePath string          `json:"file_path,omitempty"`
}

// ShadingResource 阴影资源
type ShadingResource struct {
	Name       string      `json:"name"`
	Type       int         `json:"type"`
	ColorSpace string      `json:"color_space"`
	BBox       BoundingBox `json:"bbox,omitempty"`
	Background []float64   `json:"background,omitempty"`
	Function   interface{} `json:"function,omitempty"`
}

// ExtGStateResource 扩展图形状态资源
type ExtGStateResource struct {
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties"`
}

// FontManager 字体管理器
type FontManager struct {
	systemFonts   map[string]string
	embeddedFonts map[string]*FontResource
	fontCache     map[string]*gofpdf.Fpdf
}

// NewPDFFlowProcessor 创建PDF流处理器
func NewPDFFlowProcessor(inputPath, outputPath string) (*PDFFlowProcessor, error) {
	// 创建工作目录 - 使用项目目录下的cache目录
	currentDir, _ := os.Getwd()
	cacheDir := filepath.Join(currentDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	// 在cache目录下创建具体的工作目录
	sessionID := fmt.Sprintf("session_%d", time.Now().Unix())
	workDir := filepath.Join(cacheDir, fmt.Sprintf("pdf_flow_%s", sessionID))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("创建工作目录失败: %w", err)
	}

	// 生成会话ID
	// sessionID := fmt.Sprintf("session_%d", time.Now().Unix()) // 已在上面生成

	// 创建PDF日志记录器（禁用控制台输出）
	logger, err := NewPDFLoggerWithConsole(workDir, sessionID, false)
	if err != nil {
		return nil, fmt.Errorf("创建PDF日志记录器失败: %w", err)
	}

	processor := &PDFFlowProcessor{
		workDir:     workDir,
		inputPath:   inputPath,
		outputPath:  outputPath,
		fontManager: NewFontManager(),
		logger:      logger,
		sessionID:   sessionID,
	}

	// 记录初始化信息
	processor.logger.Info("PDF流处理器已创建", map[string]interface{}{
		"输入文件": inputPath,
		"输出文件": outputPath,
		"工作目录": workDir,
		"会话ID": sessionID,
		"日志文件": logger.GetLogFilePath(),
	})

	// 记录系统信息
	processor.logSystemInfo()

	return processor, nil
}

// NewFontManager 创建字体管理器
func NewFontManager() *FontManager {
	return &FontManager{
		systemFonts:   make(map[string]string),
		embeddedFonts: make(map[string]*FontResource),
		fontCache:     make(map[string]*gofpdf.Fpdf),
	}
}

// ProcessPDF 处理PDF文件
func (p *PDFFlowProcessor) ProcessPDF() error {
	startTime := time.Now()
	p.logger.Info("开始处理PDF文件", map[string]interface{}{
		"输入文件": p.inputPath,
	})

	// 记录内存使用情况
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// 1. 解析PDF结构
	if err := p.parsePDFStructure(); err != nil {
		p.logger.LogError("解析PDF结构", err, map[string]interface{}{
			"输入文件": p.inputPath,
		})
		return fmt.Errorf("解析PDF结构失败: %w", err)
	}

	// 2. 保存流数据到临时目录
	if err := p.saveFlowData(); err != nil {
		p.logger.LogError("保存流数据", err, nil)
		return fmt.Errorf("保存流数据失败: %w", err)
	}

	// 3. 提取资源文件
	if err := p.extractResources(); err != nil {
		p.logger.LogError("提取资源文件", err, nil)
		return fmt.Errorf("提取资源文件失败: %w", err)
	}

	// 记录处理完成后的内存使用情况
	runtime.GC()
	runtime.ReadMemStats(&m2)
	p.logger.LogMemoryUsage("PDF结构解析",
		float64(m1.Alloc)/1024/1024,
		float64(m2.Alloc)/1024/1024)

	// 记录处理耗时和统计信息
	duration := time.Since(startTime)
	p.logger.LogOperationTiming("PDF结构解析", duration, map[string]interface{}{
		"页数": len(p.flowData.Pages),
	})

	p.logger.Info("PDF结构解析完成", map[string]interface{}{
		"总页数": len(p.flowData.Pages),
		"耗时":  duration.String(),
	})

	return nil
}

// ApplyTranslations 应用翻译
func (p *PDFFlowProcessor) ApplyTranslations(translations map[string]string) error {
	startTime := time.Now()
	p.logger.Info("开始应用翻译", map[string]interface{}{
		"翻译项数量": len(translations),
	})

	// 1. 加载流数据
	if err := p.loadFlowData(); err != nil {
		p.logger.LogError("加载流数据", err, nil)
		return fmt.Errorf("加载流数据失败: %w", err)
	}

	// 2. 预处理翻译映射 - 创建更多的匹配模式
	enhancedTranslations := p.enhanceTranslationMappings(translations)
	p.logger.Info("翻译映射增强完成", map[string]interface{}{
		"原始映射数": len(translations),
		"增强映射数": len(enhancedTranslations),
	})

	// 3. 应用翻译到文本元素
	translatedCount := 0
	totalElements := 0

	for pageIdx := range p.flowData.Pages {
		page := &p.flowData.Pages[pageIdx]
		pageStartTime := time.Now()
		pageTranslatedCount := 0

		for elemIdx := range page.TextElements {
			element := &page.TextElements[elemIdx]
			totalElements++

			// 跳过过短的文本或纯数字/符号
			if len(strings.TrimSpace(element.Content)) < 2 || p.isNumericOrSymbol(element.Content) {
				continue
			}

			if translation := p.findBestTranslation(element.Content, enhancedTranslations); translation != "" {
				// 记录翻译前的状态
				originalContent := element.Content
				originalBounds := element.BoundingBox

				// 计算新文本的尺寸
				newBounds, err := p.calculateTextBounds(translation, element.Font)
				if err != nil {
					p.logger.Warn("计算文本边界失败", map[string]interface{}{
						"页码":   page.PageNumber,
						"元素ID": element.ID,
						"错误":   err.Error(),
					})
					continue
				}

				// 更新文本内容和边界
				element.Content = translation
				element.BoundingBox = newBounds

				// 保持原始位置
				element.BoundingBox.X = originalBounds.X
				element.BoundingBox.Y = originalBounds.Y

				// 标记为已翻译
				element.Language = "zh"
				element.Confidence = 1.0

				// 记录翻译日志
				p.logger.LogTranslation(page.PageNumber, element.ID, originalContent, translation)

				// 记录边界变化
				p.logger.Debug("文本边界变化", map[string]interface{}{
					"页码":   page.PageNumber,
					"元素ID": element.ID,
					"原宽度":  fmt.Sprintf("%.2f", originalBounds.Width),
					"新宽度":  fmt.Sprintf("%.2f", newBounds.Width),
					"原高度":  fmt.Sprintf("%.2f", originalBounds.Height),
					"新高度":  fmt.Sprintf("%.2f", newBounds.Height),
					"宽度变化": fmt.Sprintf("%+.2f", newBounds.Width-originalBounds.Width),
					"高度变化": fmt.Sprintf("%+.2f", newBounds.Height-originalBounds.Height),
				})

				translatedCount++
				pageTranslatedCount++
			}
		}

		// 记录页面翻译完成
		pageTime := time.Since(pageStartTime)
		p.logger.Debug("页面翻译完成", map[string]interface{}{
			"页码":    page.PageNumber,
			"翻译元素数": pageTranslatedCount,
			"总元素数":  len(page.TextElements),
			"翻译率":   fmt.Sprintf("%.1f%%", float64(pageTranslatedCount)/float64(len(page.TextElements))*100),
			"耗时":    pageTime.String(),
		})
	}

	// 4. 重新计算布局
	layoutStartTime := time.Now()
	if err := p.recalculateLayout(); err != nil {
		p.logger.LogError("重新计算布局", err, nil)
		return fmt.Errorf("重新计算布局失败: %w", err)
	}
	layoutTime := time.Since(layoutStartTime)
	p.logger.LogOperationTiming("重新计算布局", layoutTime)

	// 5. 保存更新后的流数据
	saveStartTime := time.Now()
	if err := p.saveFlowData(); err != nil {
		p.logger.LogError("保存更新后的流数据", err, nil)
		return fmt.Errorf("保存更新后的流数据失败: %w", err)
	}
	saveTime := time.Since(saveStartTime)
	p.logger.LogOperationTiming("保存流数据", saveTime)

	// 记录翻译统计
	totalTime := time.Since(startTime)
	translationRate := float64(translatedCount) / float64(totalElements) * 100

	p.logger.LogStatistics(map[string]interface{}{
		"总元素数":  totalElements,
		"翻译元素数": translatedCount,
		"翻译率":   fmt.Sprintf("%.1f%%", translationRate),
		"总耗时":   totalTime.String(),
		"平均每元素": fmt.Sprintf("%.2fms", float64(totalTime.Milliseconds())/float64(totalElements)),
	})

	p.logger.Info("翻译应用完成", map[string]interface{}{
		"翻译元素数": translatedCount,
		"总元素数":  totalElements,
		"翻译率":   fmt.Sprintf("%.1f%%", translationRate),
		"总耗时":   totalTime.String(),
	})

	return nil
}

// enhanceTranslationMappings 增强翻译映射 - 改进版本
func (p *PDFFlowProcessor) enhanceTranslationMappings(translations map[string]string) map[string]string {
	enhanced := make(map[string]string)

	// 复制原始映射
	for k, v := range translations {
		enhanced[k] = v
	}

	p.logger.Debug("开始增强翻译映射", map[string]interface{}{
		"原始映射数": len(translations),
	})

	// 为每个翻译项创建变体
	for original, translation := range translations {
		// 1. 标准化版本（移除空格、标点）
		normalized := p.normalizeText(original)
		if normalized != original && normalized != "" && len(normalized) > 3 {
			enhanced[normalized] = translation
			p.logger.Debug("添加标准化映射", map[string]interface{}{
				"原始":  original,
				"标准化": normalized,
			})
		}

		// 2. 移除连字符版本
		withoutLigatures := p.removeLigatures(original)
		if withoutLigatures != original {
			enhanced[withoutLigatures] = translation
		}

		// 3. 句子分割版本（按句号、感叹号、问号分割）
		sentences := p.splitIntoSentences(original)
		if len(sentences) > 1 {
			for _, sentence := range sentences {
				sentence = strings.TrimSpace(sentence)
				if len(sentence) > 10 {
					enhanced[sentence] = translation
					p.logger.Debug("添加句子映射", map[string]interface{}{
						"句子": p.logger.truncateString(sentence, 50),
					})
				}
			}
		}

		// 4. 短语分割版本（按逗号、分号分割）
		phrases := p.splitIntoPhrases(original)
		if len(phrases) > 1 {
			for _, phrase := range phrases {
				phrase = strings.TrimSpace(phrase)
				if len(phrase) > 8 {
					enhanced[phrase] = translation
				}
			}
		}

		// 5. 单词组合版本（连续的重要单词）
		wordCombinations := p.extractWordCombinations(original)
		for _, combination := range wordCombinations {
			if len(combination) > 6 {
				enhanced[combination] = translation
			}
		}

		// 6. 去除空格版本（处理PDF解析时丢失空格的情况）
		noSpaces := strings.ReplaceAll(original, " ", "")
		if len(noSpaces) > 10 && noSpaces != original {
			enhanced[noSpaces] = translation
			p.logger.Debug("添加无空格映射", map[string]interface{}{
				"原始":  p.logger.truncateString(original, 50),
				"无空格": p.logger.truncateString(noSpaces, 50),
			})
		}

		// 7. 部分匹配版本（前缀和后缀）
		if len(original) > 20 {
			words := strings.Fields(original)
			if len(words) >= 4 {
				// 前75%的内容
				prefixWords := words[:len(words)*3/4]
				prefix := strings.Join(prefixWords, " ")
				if len(prefix) > 15 {
					enhanced[prefix] = translation
				}

				// 后75%的内容
				suffixWords := words[len(words)/4:]
				suffix := strings.Join(suffixWords, " ")
				if len(suffix) > 15 {
					enhanced[suffix] = translation
				}
			}
		}
	}

	p.logger.Debug("翻译映射增强完成", map[string]interface{}{
		"原始映射数": len(translations),
		"增强映射数": len(enhanced),
		"新增映射数": len(enhanced) - len(translations),
	})

	return enhanced
}

// removeLigatures 移除连字符
func (p *PDFFlowProcessor) removeLigatures(text string) string {
	result := text
	result = strings.ReplaceAll(result, "ﬁ", "fi")
	result = strings.ReplaceAll(result, "ﬂ", "fl")
	result = strings.ReplaceAll(result, "ﬀ", "ff")
	result = strings.ReplaceAll(result, "ﬃ", "ffi")
	result = strings.ReplaceAll(result, "ﬄ", "ffl")
	return result
}

// splitIntoSentences 将文本分割为句子
func (p *PDFFlowProcessor) splitIntoSentences(text string) []string {
	// 按句号、感叹号、问号分割
	sentences := []string{}
	current := ""

	for _, r := range text {
		current += string(r)
		if r == '.' || r == '!' || r == '?' {
			if len(strings.TrimSpace(current)) > 0 {
				sentences = append(sentences, strings.TrimSpace(current))
				current = ""
			}
		}
	}

	// 添加剩余部分
	if len(strings.TrimSpace(current)) > 0 {
		sentences = append(sentences, strings.TrimSpace(current))
	}

	return sentences
}

// splitIntoPhrases 将文本分割为短语
func (p *PDFFlowProcessor) splitIntoPhrases(text string) []string {
	// 按逗号、分号、冒号分割
	delimiters := []string{",", ";", ":"}
	phrases := []string{text}

	for _, delimiter := range delimiters {
		var newPhrases []string
		for _, phrase := range phrases {
			parts := strings.Split(phrase, delimiter)
			for _, part := range parts {
				if len(strings.TrimSpace(part)) > 0 {
					newPhrases = append(newPhrases, strings.TrimSpace(part))
				}
			}
		}
		phrases = newPhrases
	}

	return phrases
}

// extractWordCombinations 提取单词组合
func (p *PDFFlowProcessor) extractWordCombinations(text string) []string {
	words := strings.Fields(text)
	combinations := []string{}

	// 提取2-4个连续单词的组合
	for i := 0; i < len(words); i++ {
		for length := 2; length <= 4 && i+length <= len(words); length++ {
			combination := strings.Join(words[i:i+length], " ")
			if len(combination) > 6 && !p.isStopWordCombination(words[i:i+length]) {
				combinations = append(combinations, combination)
			}
		}
	}

	return combinations
}

// isStopWordCombination 检查是否为停用词组合
func (p *PDFFlowProcessor) isStopWordCombination(words []string) bool {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "have": true, "has": true, "had": true, "do": true,
		"does": true, "did": true, "will": true, "would": true, "could": true, "should": true,
	}

	stopWordCount := 0
	for _, word := range words {
		if stopWords[strings.ToLower(word)] {
			stopWordCount++
		}
	}

	// 如果超过一半是停用词，认为是停用词组合
	return float64(stopWordCount)/float64(len(words)) > 0.5
}

// extractKeywords 提取关键词
func (p *PDFFlowProcessor) extractKeywords(text string) []string {
	words := strings.Fields(text)
	var keywords []string

	// 常见停用词
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "have": true, "has": true, "had": true,
	}

	for _, word := range words {
		// 清理单词
		word = strings.Trim(word, ".,!?;:()[]{}\"'")
		word = strings.ToLower(word)

		// 过滤停用词和短词
		if len(word) > 3 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// isNumericOrSymbol 检查是否为纯数字或符号
func (p *PDFFlowProcessor) isNumericOrSymbol(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return true
	}

	// 检查是否为纯数字
	numericCount := 0
	symbolCount := 0
	letterCount := 0

	for _, r := range text {
		if r >= '0' && r <= '9' {
			numericCount++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= 0x4e00 && r <= 0x9fff) {
			letterCount++
		} else {
			symbolCount++
		}
	}

	totalChars := numericCount + symbolCount + letterCount
	if totalChars == 0 {
		return true
	}

	// 如果字母少于30%，认为是数字或符号
	letterRatio := float64(letterCount) / float64(totalChars)
	return letterRatio < 0.3
}

// GeneratePDF 生成新的PDF
func (p *PDFFlowProcessor) GeneratePDF() error {
	startTime := time.Now()
	p.logger.Info("开始生成新PDF", map[string]interface{}{
		"输出文件": p.outputPath,
	})

	// 1. 加载流数据
	if err := p.loadFlowData(); err != nil {
		p.logger.LogError("加载流数据", err, nil)
		return fmt.Errorf("加载流数据失败: %w", err)
	}

	// 2. 创建新的PDF文档
	pdf := gofpdf.New("P", "pt", "A4", "")

	// 3. 设置字体支持
	fontSetupStart := time.Now()
	if err := p.setupFonts(pdf); err != nil {
		p.logger.Warn("设置字体失败", map[string]interface{}{
			"错误": err.Error(),
		})
	}
	p.logger.LogOperationTiming("设置字体", time.Since(fontSetupStart))

	// 4. 逐页生成内容
	totalElements := 0
	for _, page := range p.flowData.Pages {
		pageStartTime := time.Now()

		if err := p.generatePage(pdf, page); err != nil {
			p.logger.LogError("生成页面", err, map[string]interface{}{
				"页码": page.PageNumber,
			})
			return fmt.Errorf("生成页面%d失败: %w", page.PageNumber, err)
		}

		pageElements := len(page.TextElements) + len(page.ImageElements) + len(page.GraphicsElements)
		totalElements += pageElements

		pageTime := time.Since(pageStartTime)
		p.logger.LogPageProcessing(page.PageNumber, len(p.flowData.Pages),
			len(page.TextElements), len(page.ImageElements), len(page.GraphicsElements))

		p.logger.Debug("页面生成耗时", map[string]interface{}{
			"页码":  page.PageNumber,
			"耗时":  pageTime.String(),
			"元素数": pageElements,
		})
	}

	// 5. 保存PDF文件
	saveStartTime := time.Now()
	if err := pdf.OutputFileAndClose(p.outputPath); err != nil {
		p.logger.LogError("保存PDF文件", err, map[string]interface{}{
			"输出文件": p.outputPath,
		})
		return fmt.Errorf("保存PDF文件失败: %w", err)
	}
	saveTime := time.Since(saveStartTime)
	p.logger.LogOperationTiming("保存PDF文件", saveTime)

	// 记录文件信息
	if info, err := os.Stat(p.outputPath); err == nil {
		p.logger.LogFileOperation("生成PDF", p.outputPath, info.Size())
	}

	// 记录生成统计
	totalTime := time.Since(startTime)
	p.logger.LogStatistics(map[string]interface{}{
		"总页数":  len(p.flowData.Pages),
		"总元素数": totalElements,
		"总耗时":  totalTime.String(),
		"平均每页": fmt.Sprintf("%.2fs", totalTime.Seconds()/float64(len(p.flowData.Pages))),
		"输出文件": p.outputPath,
	})

	p.logger.Info("PDF生成完成", map[string]interface{}{
		"输出文件": p.outputPath,
		"总页数":  len(p.flowData.Pages),
		"总耗时":  totalTime.String(),
	})

	return nil
}

// Cleanup 清理临时文件
func (p *PDFFlowProcessor) Cleanup() error {
	if p.logger != nil {
		p.logger.Info("开始清理临时文件", map[string]interface{}{
			"工作目录": p.workDir,
			"保留调试": true,
		})

		// 关闭日志记录器
		if err := p.logger.Close(); err != nil {
			log.Printf("警告：关闭日志记录器失败: %v", err)
		}
	}

	if p.workDir != "" {
		// 保留临时工作目录用于调试
		log.Printf("保留临时工作目录用于调试: %s", p.workDir)
		if p.logger != nil {
			log.Printf("日志文件位置: %s", p.logger.GetLogFilePath())
		}
		// 暂时不删除临时目录，用于调试
		// return os.RemoveAll(p.workDir)
	}
	return nil
}

// parsePDFStructure 解析PDF结构
func (p *PDFFlowProcessor) parsePDFStructure() error {
	startTime := time.Now()
	p.logger.Info("开始解析PDF结构", nil)

	// 使用pdfcpu解析PDF
	ctx, err := api.ReadContextFile(p.inputPath)
	if err != nil {
		return fmt.Errorf("读取PDF上下文失败: %w", err)
	}

	// 初始化流数据
	p.flowData = &PDFFlowData{
		ProcessTime: time.Now(),
		Pages:       make([]PDFPageFlow, 0),
		Resources: PDFResourcesFlow{
			Fonts:       make(map[string]FontResource),
			Images:      make(map[string]ImageResource),
			XObjects:    make(map[string]XObjectResource),
			ColorSpaces: make(map[string]ColorSpaceResource),
			Patterns:    make(map[string]PatternResource),
			Shadings:    make(map[string]ShadingResource),
			ExtGStates:  make(map[string]ExtGStateResource),
		},
	}

	// 提取文档元数据
	metadataStart := time.Now()
	if err := p.extractMetadata(ctx); err != nil {
		p.logger.Warn("提取元数据失败", map[string]interface{}{
			"错误": err.Error(),
		})
	}
	p.logger.LogOperationTiming("提取元数据", time.Since(metadataStart))

	// 获取文件大小
	if info, err := os.Stat(p.inputPath); err == nil {
		p.flowData.OriginalSize = info.Size()
		p.logger.LogFileOperation("读取输入文件", p.inputPath, info.Size())
	}

	// 解析每一页
	pageCount := ctx.PageCount
	p.flowData.Metadata.PageCount = pageCount

	p.logger.Info("开始解析页面", map[string]interface{}{
		"总页数": pageCount,
	})

	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		pageStartTime := time.Now()

		pageFlow, err := p.parsePage(ctx, pageNum)
		if err != nil {
			p.logger.Warn("解析页面失败", map[string]interface{}{
				"页码": pageNum,
				"错误": err.Error(),
			})
			continue
		}

		p.flowData.Pages = append(p.flowData.Pages, *pageFlow)

		pageTime := time.Since(pageStartTime)
		p.logger.Debug("页面解析完成", map[string]interface{}{
			"页码": pageNum,
			"耗时": pageTime.String(),
		})
	}

	totalTime := time.Since(startTime)
	p.logger.LogOperationTiming("PDF结构解析", totalTime, map[string]interface{}{
		"页数": len(p.flowData.Pages),
	})

	p.logger.Info("PDF结构解析完成", map[string]interface{}{
		"解析页数": len(p.flowData.Pages),
		"总页数":  pageCount,
		"总耗时":  totalTime.String(),
	})

	return nil
}

// extractMetadata 提取文档元数据
func (p *PDFFlowProcessor) extractMetadata(ctx *model.Context) error {
	// 从PDF上下文中提取元数据
	if ctx.RootDict != nil {
		// 提取基本信息
		if info, found := ctx.RootDict.Find("Info"); found {
			if infoDict, ok := info.(types.Dict); ok {
				p.extractInfoDict(infoDict)
			}
		}
	}

	return nil
}

// extractInfoDict 提取信息字典
func (p *PDFFlowProcessor) extractInfoDict(infoDict types.Dict) {
	if title, found := infoDict.Find("Title"); found {
		if titleStr, ok := title.(types.StringLiteral); ok {
			p.flowData.Metadata.Title = string(titleStr)
		}
	}

	if author, found := infoDict.Find("Author"); found {
		if authorStr, ok := author.(types.StringLiteral); ok {
			p.flowData.Metadata.Author = string(authorStr)
		}
	}

	if subject, found := infoDict.Find("Subject"); found {
		if subjectStr, ok := subject.(types.StringLiteral); ok {
			p.flowData.Metadata.Subject = string(subjectStr)
		}
	}

	if creator, found := infoDict.Find("Creator"); found {
		if creatorStr, ok := creator.(types.StringLiteral); ok {
			p.flowData.Metadata.Creator = string(creatorStr)
		}
	}

	if producer, found := infoDict.Find("Producer"); found {
		if producerStr, ok := producer.(types.StringLiteral); ok {
			p.flowData.Metadata.Producer = string(producerStr)
		}
	}
}

// parsePage 解析单个页面
func (p *PDFFlowProcessor) parsePage(ctx *model.Context, pageNum int) (*PDFPageFlow, error) {
	p.logger.Debug("开始解析页面", map[string]interface{}{
		"页码": pageNum,
	})

	// 获取页面字典
	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		return nil, fmt.Errorf("获取页面字典失败: %w", err)
	}

	pageFlow := &PDFPageFlow{
		PageNumber:       pageNum,
		TextElements:     make([]TextElementFlow, 0),
		ImageElements:    make([]ImageElementFlow, 0),
		GraphicsElements: make([]GraphicsElementFlow, 0),
		Annotations:      make([]AnnotationFlow, 0),
		ContentStreams:   make([]ContentStreamFlow, 0),
	}

	// 提取页面边界
	if err := p.extractPageBounds(pageDict, pageFlow); err != nil {
		p.logger.Warn("提取页面边界失败", map[string]interface{}{
			"页码": pageNum,
			"错误": err.Error(),
		})
	}

	// 提取内容流
	streamStart := time.Now()
	if err := p.extractContentStreams(ctx, pageDict, pageFlow); err != nil {
		p.logger.Warn("提取内容流失败", map[string]interface{}{
			"页码": pageNum,
			"错误": err.Error(),
		})
	}
	p.logger.Debug("内容流提取完成", map[string]interface{}{
		"页码":  pageNum,
		"流数量": len(pageFlow.ContentStreams),
		"耗时":  time.Since(streamStart).String(),
	})

	// 解析内容流中的元素
	parseStart := time.Now()
	if err := p.parseContentElements(pageFlow); err != nil {
		p.logger.Warn("解析内容元素失败", map[string]interface{}{
			"页码": pageNum,
			"错误": err.Error(),
		})
	}
	p.logger.Debug("内容元素解析完成", map[string]interface{}{
		"页码":   pageNum,
		"文本元素": len(pageFlow.TextElements),
		"图像元素": len(pageFlow.ImageElements),
		"图形元素": len(pageFlow.GraphicsElements),
		"耗时":   time.Since(parseStart).String(),
	})

	// 提取注释
	if err := p.extractAnnotations(ctx, pageDict, pageFlow); err != nil {
		p.logger.Warn("提取注释失败", map[string]interface{}{
			"页码": pageNum,
			"错误": err.Error(),
		})
	}

	p.logger.Debug("页面解析完成", map[string]interface{}{
		"页码":    pageNum,
		"文本元素":  len(pageFlow.TextElements),
		"图像元素":  len(pageFlow.ImageElements),
		"图形元素":  len(pageFlow.GraphicsElements),
		"注释数量":  len(pageFlow.Annotations),
		"内容流数量": len(pageFlow.ContentStreams),
	})

	return pageFlow, nil
}

// logSystemInfo 记录系统信息
func (p *PDFFlowProcessor) logSystemInfo() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	p.logger.Info("系统信息", map[string]interface{}{
		"Go版本":       runtime.Version(),
		"操作系统":       runtime.GOOS,
		"架构":         runtime.GOARCH,
		"CPU核心数":     runtime.NumCPU(),
		"Goroutine数": runtime.NumGoroutine(),
		"内存分配":       p.logger.formatBytes(int64(m.Alloc)),
		"总分配":        p.logger.formatBytes(int64(m.TotalAlloc)),
		"系统内存":       p.logger.formatBytes(int64(m.Sys)),
		"GC次数":       m.NumGC,
	})
}

// 其他方法的实现将在后续部分继续...

// truncateString 截断字符串用于日志
func (p *PDFFlowProcessor) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractPageBounds 提取页面边界
func (p *PDFFlowProcessor) extractPageBounds(pageDict types.Dict, pageFlow *PDFPageFlow) error {
	// 提取MediaBox
	if mediaBoxObj, found := pageDict.Find("MediaBox"); found {
		if mediaBox, ok := mediaBoxObj.(types.Array); ok && len(mediaBox) >= 4 {
			pageFlow.MediaBox = BoundingBox{
				X:      p.getFloatValue(mediaBox[0]),
				Y:      p.getFloatValue(mediaBox[1]),
				Width:  p.getFloatValue(mediaBox[2]) - p.getFloatValue(mediaBox[0]),
				Height: p.getFloatValue(mediaBox[3]) - p.getFloatValue(mediaBox[1]),
			}
		}
	}

	// 提取CropBox
	if cropBoxObj, found := pageDict.Find("CropBox"); found {
		if cropBox, ok := cropBoxObj.(types.Array); ok && len(cropBox) >= 4 {
			pageFlow.CropBox = &BoundingBox{
				X:      p.getFloatValue(cropBox[0]),
				Y:      p.getFloatValue(cropBox[1]),
				Width:  p.getFloatValue(cropBox[2]) - p.getFloatValue(cropBox[0]),
				Height: p.getFloatValue(cropBox[3]) - p.getFloatValue(cropBox[1]),
			}
		}
	}

	// 提取Rotation
	if rotateObj, found := pageDict.Find("Rotate"); found {
		pageFlow.Rotation = int(p.getFloatValue(rotateObj))
	}

	return nil
}

// getFloatValue 获取浮点值
func (p *PDFFlowProcessor) getFloatValue(obj types.Object) float64 {
	switch v := obj.(type) {
	case types.Float:
		return float64(v)
	case types.Integer:
		return float64(v)
	default:
		return 0.0
	}
}

// extractContentStreams 提取内容流
func (p *PDFFlowProcessor) extractContentStreams(ctx *model.Context, pageDict types.Dict, pageFlow *PDFPageFlow) error {
	contentsObj, found := pageDict.Find("Contents")
	if !found {
		return nil
	}

	switch obj := contentsObj.(type) {
	case types.IndirectRef:
		// 单个内容流
		stream, err := p.extractSingleContentStream(ctx, obj, 0)
		if err != nil {
			return err
		}
		pageFlow.ContentStreams = append(pageFlow.ContentStreams, *stream)

	case types.Array:
		// 多个内容流
		for i, item := range obj {
			if ref, ok := item.(types.IndirectRef); ok {
				stream, err := p.extractSingleContentStream(ctx, ref, i)
				if err != nil {
					log.Printf("警告：提取内容流%d失败: %v", i, err)
					continue
				}
				pageFlow.ContentStreams = append(pageFlow.ContentStreams, *stream)
			}
		}
	}

	return nil
}

// extractSingleContentStream 提取单个内容流
func (p *PDFFlowProcessor) extractSingleContentStream(ctx *model.Context, ref types.IndirectRef, index int) (*ContentStreamFlow, error) {
	streamDict, _, err := ctx.DereferenceStreamDict(ref)
	if err != nil {
		return nil, fmt.Errorf("解引用内容流失败: %w", err)
	}

	// 解码流内容
	content, err := p.decodeStreamContent(streamDict)
	if err != nil {
		return nil, fmt.Errorf("解码流内容失败: %w", err)
	}

	// 解析PDF操作符
	ops, err := p.parseOperations(content)
	if err != nil {
		log.Printf("警告：解析操作符失败: %v", err)
		ops = []PDFOperation{} // 使用空操作符列表
	}

	stream := &ContentStreamFlow{
		ID:           fmt.Sprintf("stream_%d", index),
		StreamIndex:  index,
		RawContent:   content,
		ParsedOps:    ops,
		Dependencies: make([]string, 0),
	}

	return stream, nil
}

// decodeStreamContent 解码流内容
func (p *PDFFlowProcessor) decodeStreamContent(streamDict *types.StreamDict) (string, error) {
	if streamDict.Content == nil {
		if streamDict.Dict != nil {
			if err := streamDict.Decode(); err != nil {
				return "", fmt.Errorf("解码流字典失败: %w", err)
			}
			if streamDict.Content != nil {
				return string(streamDict.Content), nil
			}
		}
		return "", fmt.Errorf("流内容为空且无法解码")
	}
	return string(streamDict.Content), nil
}

// parseOperations 解析PDF操作符
func (p *PDFFlowProcessor) parseOperations(content string) ([]PDFOperation, error) {
	var operations []PDFOperation

	// 使用更智能的方式解析PDF操作符
	// 不是按行分割，而是按操作符分割
	ops := p.tokenizePDFOperations(content)

	p.logger.Debug("开始解析内容流", map[string]interface{}{
		"操作符数量": len(ops),
		"内容长度":  len(content),
	})

	textOperatorCount := 0
	for i, op := range ops {
		// 检查是否是文本操作符
		if op.Operator == "Tj" || op.Operator == "TJ" || op.Operator == "'" || op.Operator == "\"" {
			textOperatorCount++
			p.logger.Debug("发现文本操作符", map[string]interface{}{
				"操作符": op.Operator,
				"操作数": fmt.Sprintf("%v", op.Operands),
				"位置":  i,
			})
		}

		op.Position = i
		operations = append(operations, op)
	}

	p.logger.Debug("内容流解析完成", map[string]interface{}{
		"总操作符数": len(operations),
		"文本操作符": textOperatorCount,
		"文本比例":  fmt.Sprintf("%.1f%%", float64(textOperatorCount)/float64(len(operations))*100),
	})

	return operations, nil
}

// tokenizePDFOperations 将PDF内容流标记化为操作符
func (p *PDFFlowProcessor) tokenizePDFOperations(content string) []PDFOperation {
	var operations []PDFOperation
	var tokens []string

	// 首先将内容标记化
	tokens = p.tokenizePDFContent(content)

	// 然后将标记组合成操作符
	i := 0
	for i < len(tokens) {
		// 查找下一个操作符
		opIndex := p.findNextOperator(tokens, i)
		if opIndex == -1 {
			break
		}

		operator := tokens[opIndex]
		operands := tokens[i:opIndex]

		op := PDFOperation{
			Operator: operator,
			Operands: operands,
			Position: len(operations),
			Context: OpContext{
				GraphicsState: GraphicsState{},
				TextState:     TextStateFlow{},
				Transform:     TransformMatrix{A: 1, D: 1},
			},
		}

		operations = append(operations, op)
		i = opIndex + 1
	}

	return operations
}

// tokenizePDFContent 将PDF内容标记化
func (p *PDFFlowProcessor) tokenizePDFContent(content string) []string {
	var tokens []string
	var current strings.Builder
	inParens := 0
	inBrackets := 0
	inAngleBrackets := 0

	i := 0
	for i < len(content) {
		char := content[i]

		switch char {
		case '(':
			inParens++
			current.WriteByte(char)
		case ')':
			inParens--
			current.WriteByte(char)
			// 如果括号闭合，这可能是一个完整的标记
			if inParens == 0 && inBrackets == 0 && inAngleBrackets == 0 {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			}
		case '[':
			inBrackets++
			current.WriteByte(char)
		case ']':
			inBrackets--
			current.WriteByte(char)
			// 如果方括号闭合，这可能是一个完整的标记
			if inParens == 0 && inBrackets == 0 && inAngleBrackets == 0 {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			}
		case '<':
			inAngleBrackets++
			current.WriteByte(char)
		case '>':
			inAngleBrackets--
			current.WriteByte(char)
			// 如果尖括号闭合，这可能是一个完整的标记
			if inParens == 0 && inBrackets == 0 && inAngleBrackets == 0 {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			}
		case ' ', '\t', '\n', '\r':
			// 空白字符分隔标记（如果不在括号内）
			if inParens == 0 && inBrackets == 0 && inAngleBrackets == 0 {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			} else {
				current.WriteByte(char)
			}
		default:
			current.WriteByte(char)
		}
		i++
	}

	// 添加最后一个标记
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// findNextOperator 查找下一个PDF操作符
func (p *PDFFlowProcessor) findNextOperator(tokens []string, start int) int {
	// PDF操作符列表
	operators := map[string]bool{
		// 文本操作符
		"Tj": true, "TJ": true, "'": true, "\"": true,
		"Td": true, "TD": true, "Tm": true, "T*": true,
		"Tc": true, "Tw": true, "Tz": true, "TL": true, "Tf": true,
		"Tr": true, "Ts": true, "BT": true, "ET": true,

		// 图形操作符
		"m": true, "l": true, "c": true, "v": true, "y": true, "h": true,
		"re": true, "S": true, "s": true, "f": true, "F": true, "f*": true,
		"B": true, "B*": true, "b": true, "b*": true, "n": true,
		"W": true, "W*": true,

		// 颜色操作符
		"CS": true, "cs": true, "SC": true, "SCN": true, "sc": true, "scn": true,
		"G": true, "g": true, "RG": true, "rg": true, "K": true, "k": true,

		// 变换操作符
		"cm": true, "q": true, "Q": true,

		// 图像操作符
		"Do": true, "BI": true, "ID": true, "EI": true,

		// 其他操作符
		"w": true, "J": true, "j": true, "M": true, "d": true, "ri": true,
		"i": true, "gs": true, "sh": true,
	}

	for i := start; i < len(tokens); i++ {
		if operators[tokens[i]] {
			return i
		}
	}

	return -1
}

// parseOperation 解析单个操作符
func (p *PDFFlowProcessor) parseOperation(line string, position int) (*PDFOperation, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	// 处理复杂的PDF操作符行，可能包含括号、数组等
	parts := p.splitPDFOperationLine(line)
	if len(parts) == 0 {
		return nil, nil
	}

	// 最后一个部分是操作符
	operator := parts[len(parts)-1]
	operands := parts[:len(parts)-1]

	op := &PDFOperation{
		Operator: operator,
		Operands: operands,
		Position: position,
		Context: OpContext{
			GraphicsState: GraphicsState{},
			TextState:     TextStateFlow{},
			Transform:     TransformMatrix{A: 1, D: 1}, // 单位矩阵
		},
	}

	return op, nil
}

// splitPDFOperationLine 分割PDF操作符行，正确处理括号和数组
func (p *PDFFlowProcessor) splitPDFOperationLine(line string) []string {
	var parts []string
	var current strings.Builder
	inParens := 0
	inBrackets := 0
	inAngleBrackets := 0

	i := 0
	for i < len(line) {
		char := line[i]

		switch char {
		case '(':
			inParens++
			current.WriteByte(char)
		case ')':
			inParens--
			current.WriteByte(char)
		case '[':
			inBrackets++
			current.WriteByte(char)
		case ']':
			inBrackets--
			current.WriteByte(char)
		case '<':
			inAngleBrackets++
			current.WriteByte(char)
		case '>':
			inAngleBrackets--
			current.WriteByte(char)
		case ' ', '\t':
			if inParens == 0 && inBrackets == 0 && inAngleBrackets == 0 {
				// 在括号外的空格，分割
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			} else {
				current.WriteByte(char)
			}
		default:
			current.WriteByte(char)
		}
		i++
	}

	// 添加最后一部分
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseContentElements 解析内容元素
func (p *PDFFlowProcessor) parseContentElements(pageFlow *PDFPageFlow) error {
	textElementID := 0
	imageElementID := 0
	graphicsElementID := 0

	// 当前状态
	currentTransform := TransformMatrix{A: 1, D: 1} // 单位矩阵
	currentTextState := TextStateFlow{Scale: 1.0}
	currentFont := FontFlow{Name: "default", Size: 12}
	currentColor := ColorFlow{Space: "RGB", Values: []float64{0, 0, 0}, Alpha: 1.0}

	for _, stream := range pageFlow.ContentStreams {
		for _, op := range stream.ParsedOps {
			switch op.Operator {
			case "Tj", "TJ", "'", "\"":
				// 文本显示操作符
				element, err := p.parseTextElement(op, textElementID, currentTransform, currentTextState, currentFont, currentColor)
				if err != nil {
					log.Printf("警告：解析文本元素失败: %v", err)
					continue
				}
				if element != nil {
					pageFlow.TextElements = append(pageFlow.TextElements, *element)
					textElementID++
				}

			case "Do":
				// XObject操作符（通常是图像）
				element, err := p.parseImageElement(op, imageElementID, currentTransform)
				if err != nil {
					log.Printf("警告：解析图像元素失败: %v", err)
					continue
				}
				if element != nil {
					pageFlow.ImageElements = append(pageFlow.ImageElements, *element)
					imageElementID++
				}

			case "m", "l", "c", "v", "y", "h", "re", "S", "s", "f", "F", "f*", "B", "B*", "b", "b*", "n":
				// 图形操作符
				element, err := p.parseGraphicsElement(op, graphicsElementID, currentTransform)
				if err != nil {
					log.Printf("警告：解析图形元素失败: %v", err)
					continue
				}
				if element != nil {
					pageFlow.GraphicsElements = append(pageFlow.GraphicsElements, *element)
					graphicsElementID++
				}

			case "cm":
				// 变换矩阵
				if len(op.Operands) >= 6 {
					currentTransform = p.parseTransformMatrix(op.Operands)
				}

			case "Tf":
				// 字体设置
				if len(op.Operands) >= 2 {
					currentFont.Name = op.Operands[0]
					if size, err := p.parseFloat(op.Operands[1]); err == nil {
						currentFont.Size = size
					}
				}

			case "Tc":
				// 字符间距
				if len(op.Operands) >= 1 {
					if charSpace, err := p.parseFloat(op.Operands[0]); err == nil {
						currentTextState.CharSpace = charSpace
					}
				}

			case "Tw":
				// 词间距
				if len(op.Operands) >= 1 {
					if wordSpace, err := p.parseFloat(op.Operands[0]); err == nil {
						currentTextState.WordSpace = wordSpace
					}
				}

			case "Tz":
				// 水平缩放
				if len(op.Operands) >= 1 {
					if scale, err := p.parseFloat(op.Operands[0]); err == nil {
						currentTextState.Scale = scale / 100.0
					}
				}

			case "TL":
				// 行间距
				if len(op.Operands) >= 1 {
					if leading, err := p.parseFloat(op.Operands[0]); err == nil {
						currentTextState.Leading = leading
					}
				}

			case "rg", "RG":
				// RGB颜色
				if len(op.Operands) >= 3 {
					r, _ := p.parseFloat(op.Operands[0])
					g, _ := p.parseFloat(op.Operands[1])
					b, _ := p.parseFloat(op.Operands[2])
					currentColor = ColorFlow{
						Space:  "RGB",
						Values: []float64{r, g, b},
						Alpha:  1.0,
					}
				}
			}
		}
	}

	// 合并相邻的文本元素以减少过度分割
	p.mergeAdjacentTextElements(pageFlow)

	return nil
}

// parseTextElement 解析文本元素
func (p *PDFFlowProcessor) parseTextElement(op PDFOperation, id int, transform TransformMatrix, textState TextStateFlow, font FontFlow, color ColorFlow) (*TextElementFlow, error) {
	if len(op.Operands) == 0 {
		return nil, fmt.Errorf("文本操作符缺少操作数")
	}

	// 提取文本内容
	content := p.extractTextFromOperands(op.Operands, op.Operator)

	// 记录文本提取日志
	p.logger.Debug("解析文本元素", map[string]interface{}{
		"操作符":  op.Operator,
		"操作数":  fmt.Sprintf("%v", op.Operands),
		"提取内容": p.logger.truncateString(content, 100),
		"内容长度": len(content),
		"字体":   font.Name,
		"字体大小": font.Size,
	})

	if content == "" {
		p.logger.Debug("文本内容为空，跳过元素创建", map[string]interface{}{
			"操作符": op.Operator,
			"操作数": fmt.Sprintf("%v", op.Operands),
		})
		return nil, nil
	}

	// 计算文本边界
	bounds, err := p.calculateTextBounds(content, font)
	if err != nil {
		p.logger.Warn("计算文本边界失败", map[string]interface{}{
			"内容": p.logger.truncateString(content, 50),
			"错误": err.Error(),
		})
		bounds = BoundingBox{Width: float64(len(content)) * font.Size * 0.6, Height: font.Size}
	}

	// 修复位置计算 - 确保位置不为0,0（除非确实应该在原点）
	posX := transform.E
	posY := transform.F

	// 如果变换矩阵的位置为0,0，尝试从其他信息推断位置
	if posX == 0 && posY == 0 {
		// 使用元素ID来分散位置，避免所有文本堆叠在同一位置
		posX = 72 + float64(id%5)*100 // 水平分散：72, 172, 272, 372, 472
		posY = 720 - float64(id/5)*20 // 垂直分散：每5个元素下移20点

		p.logger.Debug("使用分散的默认位置", map[string]interface{}{
			"元素ID": id,
			"原位置X": transform.E,
			"原位置Y": transform.F,
			"新位置X": posX,
			"新位置Y": posY,
		})
	} else {
		p.logger.Debug("使用原始变换位置", map[string]interface{}{
			"元素ID": id,
			"位置X":  posX,
			"位置Y":  posY,
			"变换矩阵": fmt.Sprintf("A:%.2f B:%.2f C:%.2f D:%.2f E:%.2f F:%.2f",
				transform.A, transform.B, transform.C, transform.D, transform.E, transform.F),
		})
	}

	element := &TextElementFlow{
		ID:      fmt.Sprintf("text_%d", id),
		Content: content,
		Position: PositionFlow{
			X: posX,
			Y: posY,
		},
		Font:         font,
		Color:        color,
		Transform:    transform,
		BoundingBox:  bounds,
		TextState:    textState,
		IsFormula:    p.isFormula(content, font.Name),
		Language:     p.detectLanguage(content),
		Confidence:   1.0,
		OriginalOps:  []string{fmt.Sprintf("%s %s", strings.Join(op.Operands, " "), op.Operator)},
		Dependencies: make([]string, 0),
	}

	// 更新边界框的位置
	element.BoundingBox.X = posX
	element.BoundingBox.Y = posY

	p.logger.Debug("成功创建文本元素", map[string]interface{}{
		"ID":   element.ID,
		"内容":   p.logger.truncateString(element.Content, 50),
		"位置X":  fmt.Sprintf("%.2f", element.Position.X),
		"位置Y":  fmt.Sprintf("%.2f", element.Position.Y),
		"宽度":   fmt.Sprintf("%.2f", element.BoundingBox.Width),
		"高度":   fmt.Sprintf("%.2f", element.BoundingBox.Height),
		"语言":   element.Language,
		"是否公式": element.IsFormula,
	})

	return element, nil
}

// parseImageElement 解析图像元素
func (p *PDFFlowProcessor) parseImageElement(op PDFOperation, id int, transform TransformMatrix) (*ImageElementFlow, error) {
	if len(op.Operands) == 0 {
		return nil, fmt.Errorf("图像操作符缺少操作数")
	}

	imageName := op.Operands[0]

	// 修复图像位置和尺寸计算
	posX := transform.E
	posY := transform.F

	// 从变换矩阵中获取实际的图像尺寸
	width := transform.A  // 通常A表示X方向的缩放
	height := transform.D // 通常D表示Y方向的缩放

	// 如果尺寸为0或负数，使用默认值
	if width <= 0 {
		width = 100
	}
	if height <= 0 {
		height = 100
	}

	// 如果位置为0,0，尝试使用合理的默认位置
	if posX == 0 && posY == 0 {
		posX = 72  // 1英寸边距
		posY = 720 // 页面顶部附近
	}

	element := &ImageElementFlow{
		ID:   fmt.Sprintf("image_%d", id),
		Name: imageName,
		Position: PositionFlow{
			X: posX,
			Y: posY,
		},
		Size: SizeFlow{
			Width:  width,
			Height: height,
		},
		Transform:   transform,
		BoundingBox: BoundingBox{X: posX, Y: posY, Width: width, Height: height},
		Format:      "unknown",
		Inline:      false,
	}

	p.logger.Debug("创建图像元素", map[string]interface{}{
		"ID":  element.ID,
		"名称":  element.Name,
		"位置X": fmt.Sprintf("%.2f", element.Position.X),
		"位置Y": fmt.Sprintf("%.2f", element.Position.Y),
		"宽度":  fmt.Sprintf("%.2f", element.Size.Width),
		"高度":  fmt.Sprintf("%.2f", element.Size.Height),
	})

	return element, nil
}

// parseGraphicsElement 解析图形元素
func (p *PDFFlowProcessor) parseGraphicsElement(op PDFOperation, id int, transform TransformMatrix) (*GraphicsElementFlow, error) {
	element := &GraphicsElementFlow{
		ID:        fmt.Sprintf("graphics_%d", id),
		Type:      p.getGraphicsType(op.Operator),
		Transform: transform,
		Path:      p.parsePathFromOperands(op.Operands, op.Operator),
		Style: GraphicsStyle{
			StrokeColor: ColorFlow{Space: "RGB", Values: []float64{0, 0, 0}, Alpha: 1.0},
			FillColor:   ColorFlow{Space: "RGB", Values: []float64{0, 0, 0}, Alpha: 1.0},
			LineWidth:   1.0,
		},
		BoundingBox: BoundingBox{}, // 需要从路径计算
	}

	return element, nil
}

// extractTextFromOperands 从操作数中提取文本
func (p *PDFFlowProcessor) extractTextFromOperands(operands []string, operator string) string {
	if len(operands) == 0 {
		p.logger.Debug("操作数为空", map[string]interface{}{
			"操作符": operator,
		})
		return ""
	}

	p.logger.Debug("提取文本", map[string]interface{}{
		"操作符":   operator,
		"操作数数量": len(operands),
	})

	for i, operand := range operands {
		p.logger.Debug("操作数详情", map[string]interface{}{
			"索引": i,
			"内容": p.logger.truncateString(operand, 200),
		})
	}

	var result string
	switch operator {
	case "Tj":
		// 简单文本显示: (text) Tj
		result = p.cleanPDFText(operands[0])
		p.logger.Debug("Tj操作符提取结果", map[string]interface{}{
			"原始":  p.logger.truncateString(operands[0], 100),
			"清理后": p.logger.truncateString(result, 100),
		})

	case "TJ":
		// 数组文本显示: [(text1) offset (text2) ...] TJ
		result = p.extractTextFromTJArray(operands[0])
		p.logger.Debug("TJ操作符提取结果", map[string]interface{}{
			"原始":  p.logger.truncateString(operands[0], 100),
			"清理后": p.logger.truncateString(result, 100),
		})

	case "'":
		// 移动到下一行并显示文本: (text) '
		result = p.cleanPDFText(operands[0])
		p.logger.Debug("'操作符提取结果", map[string]interface{}{
			"原始":  p.logger.truncateString(operands[0], 100),
			"清理后": p.logger.truncateString(result, 100),
		})

	case "\"":
		// 设置词间距、字符间距并显示文本: aw ac (text) "
		if len(operands) >= 3 {
			result = p.cleanPDFText(operands[2])
			p.logger.Debug("\"操作符提取结果", map[string]interface{}{
				"词间距":  operands[0],
				"字符间距": operands[1],
				"原始文本": p.logger.truncateString(operands[2], 100),
				"清理后":  p.logger.truncateString(result, 100),
			})
		}
	}

	if result == "" {
		p.logger.Debug("未识别的操作符或无法提取文本", map[string]interface{}{
			"操作符": operator,
		})
	}

	return result
}

// cleanPDFText 清理PDF文本 - 改进版本
func (p *PDFFlowProcessor) cleanPDFText(text string) string {
	if text == "" {
		return ""
	}

	p.logger.Debug("清理PDF文本", map[string]interface{}{
		"输入": p.logger.truncateString(text, 100),
	})

	originalText := text

	// 不要移除外层括号，因为它们可能是文本内容的一部分
	// 只有当文本被完整的括号包围时才移除
	if strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") && p.isCompletelyWrapped(text, '(', ')') {
		text = text[1 : len(text)-1]
		p.logger.Debug("移除外层圆括号", map[string]interface{}{
			"处理后": p.logger.truncateString(text, 100),
		})
	}

	if strings.HasPrefix(text, "<") && strings.HasSuffix(text, ">") && p.isCompletelyWrapped(text, '<', '>') {
		// 处理十六进制编码
		hexText := text[1 : len(text)-1]
		if decoded := p.hexToText(hexText); decoded != "" {
			p.logger.Debug("十六进制解码", map[string]interface{}{
				"十六进制": p.logger.truncateString(hexText, 50),
				"解码结果": p.logger.truncateString(decoded, 100),
			})
			return decoded
		}
		text = text[1 : len(text)-1]
		p.logger.Debug("移除外层尖括号", map[string]interface{}{
			"处理后": p.logger.truncateString(text, 100),
		})
	}

	// 处理转义字符
	text = strings.ReplaceAll(text, "\\(", "(")
	text = strings.ReplaceAll(text, "\\)", ")")
	text = strings.ReplaceAll(text, "\\\\", "\\")
	text = strings.ReplaceAll(text, "\\n", "\n")
	text = strings.ReplaceAll(text, "\\r", "\r")
	text = strings.ReplaceAll(text, "\\t", "\t")

	// 处理特殊字符编码 - 扩展更多常见的PDF字符编码
	replacements := map[string]string{
		"\\002": "ﬁ",  // fi连字
		"\\003": "ﬂ",  // fl连字
		"\\001": "ﬀ",  // ff连字
		"\\004": "ﬃ",  // ffi连字
		"\\005": "ﬄ",  // ffl连字
		"\\013": "–",  // en dash
		"\\014": "—",  // em dash
		"\\015": "'",  // left single quotation mark
		"\\016": "'",  // right single quotation mark
		"\\017": "\"", // left double quotation mark
		"\\020": "\"", // right double quotation mark
		"\\021": "•",  // bullet
		"\\022": "…",  // horizontal ellipsis
		"\\050": "(",  // left parenthesis
		"\\051": ")",  // right parenthesis
		"\\052": "*",  // asterisk
		"\\053": "+",  // plus sign
		"\\054": ",",  // comma
		"\\055": "-",  // hyphen-minus
		"\\056": ".",  // full stop
		"\\057": "/",  // solidus
	}

	for encoded, decoded := range replacements {
		text = strings.ReplaceAll(text, encoded, decoded)
	}

	// 处理八进制编码
	text = p.decodeOctalEscapes(text)

	// 处理Unicode编码
	text = p.decodeUnicodeEscapes(text)

	// 清理多余的空白字符，但保留必要的空格
	text = p.normalizeWhitespace(text)

	if text != originalText {
		p.logger.Debug("文本清理完成", map[string]interface{}{
			"原始":   p.logger.truncateString(originalText, 50),
			"清理后":  p.logger.truncateString(text, 50),
			"长度变化": fmt.Sprintf("%d -> %d", len(originalText), len(text)),
		})
	}

	return text
}

// isCompletelyWrapped 检查字符串是否被完整的括号包围
func (p *PDFFlowProcessor) isCompletelyWrapped(text string, open, close rune) bool {
	if len(text) < 2 {
		return false
	}

	if rune(text[0]) != open || rune(text[len(text)-1]) != close {
		return false
	}

	// 检查内部是否有未配对的括号
	count := 0
	for i, char := range text {
		if i == 0 || i == len(text)-1 {
			continue // 跳过首尾字符
		}

		if char == open {
			count++
		} else if char == close {
			count--
			if count < 0 {
				return false // 有未配对的关闭括号
			}
		}
	}

	return count == 0 // 所有括号都配对
}

// extractTextFromTJArray 从TJ数组中提取文本 - 改进版本，正确处理间距
func (p *PDFFlowProcessor) extractTextFromTJArray(arrayStr string) string {
	if arrayStr == "" {
		return ""
	}

	p.logger.Debug("TJ数组解析", map[string]interface{}{
		"输入": p.logger.truncateString(arrayStr, 200),
	})

	// 移除外层方括号
	arrayStr = strings.Trim(arrayStr, "[]")
	arrayStr = strings.TrimSpace(arrayStr)

	var result strings.Builder
	var current strings.Builder
	inParens := 0
	inAngleBrackets := 0
	i := 0
	textFragments := 0
	lastWasText := false
	lastOffset := 0.0 // 跟踪上一个偏移量

	for i < len(arrayStr) {
		char := arrayStr[i]

		switch char {
		case '(':
			inParens++
			if inParens == 1 {
				// 开始新的文本片段
				current.Reset()
			} else {
				current.WriteByte(char)
			}
		case ')':
			inParens--
			if inParens == 0 {
				// 结束文本片段，处理括号内的内容
				textContent := current.String()
				cleanedText := p.cleanPDFText(textContent)
				if cleanedText != "" {
					// 改进的间距逻辑：基于偏移量和文本内容决定是否添加空格
					shouldAddSpace := p.shouldAddSpaceBetweenTexts(result.String(), cleanedText, lastOffset, lastWasText)
					if shouldAddSpace {
						result.WriteString(" ")
					}
					result.WriteString(cleanedText)
					textFragments++
					lastWasText = true
					p.logger.Debug("提取文本片段", map[string]interface{}{
						"片段":   textFragments,
						"原始":   p.logger.truncateString(textContent, 50),
						"清理后":  p.logger.truncateString(cleanedText, 50),
						"添加空格": shouldAddSpace,
						"上次偏移": lastOffset,
					})
				}
				current.Reset()
			} else {
				current.WriteByte(char)
			}
		case '<':
			inAngleBrackets++
			if inAngleBrackets == 1 {
				current.Reset()
			} else {
				current.WriteByte(char)
			}
		case '>':
			inAngleBrackets--
			if inAngleBrackets == 0 {
				// 处理十六进制文本
				hexText := current.String()
				if decoded := p.hexToText(hexText); decoded != "" {
					shouldAddSpace := p.shouldAddSpaceBetweenTexts(result.String(), decoded, lastOffset, lastWasText)
					if shouldAddSpace {
						result.WriteString(" ")
					}
					result.WriteString(decoded)
					textFragments++
					lastWasText = true
					p.logger.Debug("提取十六进制文本", map[string]interface{}{
						"片段":   textFragments,
						"十六进制": p.logger.truncateString(hexText, 50),
						"解码结果": p.logger.truncateString(decoded, 50),
						"添加空格": shouldAddSpace,
					})
				}
				current.Reset()
			} else {
				current.WriteByte(char)
			}
		case ' ', '\t', '\n', '\r':
			// 跳过空白字符（在括号外）
			if inParens > 0 || inAngleBrackets > 0 {
				current.WriteByte(char)
			}
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			// 处理数字调整值（在括号外）
			if inParens > 0 || inAngleBrackets > 0 {
				current.WriteByte(char)
			} else {
				// 解析并记录偏移量
				offsetStr := ""
				for i < len(arrayStr) && (arrayStr[i] == '-' || (arrayStr[i] >= '0' && arrayStr[i] <= '9') || arrayStr[i] == '.') {
					offsetStr += string(arrayStr[i])
					i++
				}
				i-- // 因为循环末尾会i++

				// 尝试解析偏移量
				if offset := p.parseOffset(offsetStr); offset != 0 {
					lastOffset = offset
					p.logger.Debug("解析偏移量", map[string]interface{}{
						"偏移字符串": offsetStr,
						"偏移值":   offset,
					})
				}
				lastWasText = false
			}
		default:
			if inParens > 0 || inAngleBrackets > 0 {
				current.WriteByte(char)
			}
		}
		i++
	}

	finalResult := result.String()
	p.logger.Debug("TJ数组解析完成", map[string]interface{}{
		"文本片段数": textFragments,
		"最终结果":  p.logger.truncateString(finalResult, 100),
		"结果长度":  len(finalResult),
	})

	return finalResult
}

// shouldAddSpaceBetweenTexts 智能判断是否应该在两个文本片段之间添加空格
func (p *PDFFlowProcessor) shouldAddSpaceBetweenTexts(previousText, currentText string, lastOffset float64, lastWasText bool) bool {
	if !lastWasText || len(previousText) == 0 || len(currentText) == 0 {
		return false
	}

	// 获取前一个文本的最后一个字符和当前文本的第一个字符
	prevRunes := []rune(previousText)
	currRunes := []rune(currentText)

	if len(prevRunes) == 0 || len(currRunes) == 0 {
		return false
	}

	lastChar := prevRunes[len(prevRunes)-1]
	firstChar := currRunes[0]

	// 1. 如果偏移量很大（通常表示需要空格），添加空格
	if lastOffset < -100 { // 负偏移量通常表示间距
		return true
	}

	// 2. 标点符号规则
	if p.isPunctuationChar(lastChar) {
		// 标点符号后通常需要空格（除非下一个也是标点）
		return !p.isPunctuationChar(firstChar)
	}

	if p.isPunctuationChar(firstChar) {
		// 标点符号前通常不需要空格
		return false
	}

	// 3. 字母和数字之间通常需要空格
	if p.isAlphaNumeric(lastChar) && p.isAlphaNumeric(firstChar) {
		return true
	}

	// 4. 中英文混合的情况
	if p.isCJK(lastChar) && p.isLatin(firstChar) {
		return true
	}
	if p.isLatin(lastChar) && p.isCJK(firstChar) {
		return true
	}

	// 5. 默认情况：如果都是字母，添加空格
	if p.isLetter(lastChar) && p.isLetter(firstChar) {
		return true
	}

	return false
}

// parseOffset 解析偏移量字符串
func (p *PDFFlowProcessor) parseOffset(offsetStr string) float64 {
	if offsetStr == "" {
		return 0
	}

	// 简单的数字解析
	var result float64
	var sign float64 = 1
	i := 0

	if i < len(offsetStr) && offsetStr[i] == '-' {
		sign = -1
		i++
	}

	for i < len(offsetStr) && offsetStr[i] >= '0' && offsetStr[i] <= '9' {
		result = result*10 + float64(offsetStr[i]-'0')
		i++
	}

	if i < len(offsetStr) && offsetStr[i] == '.' {
		i++
		decimal := 0.1
		for i < len(offsetStr) && offsetStr[i] >= '0' && offsetStr[i] <= '9' {
			result += float64(offsetStr[i]-'0') * decimal
			decimal *= 0.1
			i++
		}
	}

	return result * sign
}

// isPunctuationChar 检查单个字符是否为标点符号
func (p *PDFFlowProcessor) isPunctuationChar(r rune) bool {
	// 定义标点符号字符
	switch r {
	case '.', ',', ';', ':', '!', '?', '(', ')', '[', ']', '{', '}', '`', '~', '@', '#', '$', '%', '^', '&', '*', '-', '+', '=', '|', '\\', '/', '<', '>':
		return true
	case '，', '。', '；', '：', '！', '？', '（', '）', '【', '】', '「', '」', '『', '』', '"', '\'', '…', '—', '–':
		return true
	default:
		return false
	}
}

// isAlphaNumeric 检查字符是否为字母或数字
func (p *PDFFlowProcessor) isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// isLetter 检查字符是否为字母
func (p *PDFFlowProcessor) isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isLatin 检查字符是否为拉丁字母
func (p *PDFFlowProcessor) isLatin(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isCJK 检查字符是否为中日韩文字
func (p *PDFFlowProcessor) isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK统一汉字
		(r >= 0x3400 && r <= 0x4DBF) || // CJK扩展A
		(r >= 0x20000 && r <= 0x2A6DF) || // CJK扩展B
		(r >= 0x3040 && r <= 0x309F) || // 平假名
		(r >= 0x30A0 && r <= 0x30FF) || // 片假名
		(r >= 0xAC00 && r <= 0xD7AF) // 韩文
}

// isPunctuation 检查是否为标点符号
func (p *PDFFlowProcessor) isPunctuation(text string) bool {
	if len(text) == 0 {
		return false
	}

	punctuations := []string{",", ".", ":", ";", "!", "?", "(", ")", "[", "]", "{", "}", "，", "。", "：", "；", "！", "？", "（", "）", "【", "】"}
	firstChar := string([]rune(text)[0])

	for _, punct := range punctuations {
		if firstChar == punct {
			return true
		}
	}
	return false
}

// decodeOctalEscapes 解码八进制转义序列
func (p *PDFFlowProcessor) decodeOctalEscapes(text string) string {
	result := strings.Builder{}
	i := 0

	for i < len(text) {
		if text[i] == '\\' && i+1 < len(text) {
			// 检查是否是八进制转义
			if text[i+1] >= '0' && text[i+1] <= '7' {
				// 读取最多3位八进制数字
				octalStr := ""
				j := i + 1
				for j < len(text) && j < i+4 && text[j] >= '0' && text[j] <= '7' {
					octalStr += string(text[j])
					j++
				}

				if octalStr != "" {
					// 转换八进制到字符
					var charCode int
					if n, err := fmt.Sscanf(octalStr, "%o", &charCode); err == nil && n == 1 {
						if charCode > 0 && charCode < 256 {
							result.WriteByte(byte(charCode))
						}
					}
					i = j
					continue
				}
			}
		}

		result.WriteByte(text[i])
		i++
	}

	return result.String()
}

// hexToText 将十六进制转换为文本
func (p *PDFFlowProcessor) hexToText(hex string) string {
	if len(hex)%2 != 0 {
		return ""
	}

	var result strings.Builder
	for i := 0; i < len(hex); i += 2 {
		if i+1 < len(hex) {
			hexByte := hex[i : i+2]
			var charCode int
			if n, err := fmt.Sscanf(hexByte, "%02x", &charCode); err == nil && n == 1 {
				if charCode > 0 {
					result.WriteByte(byte(charCode))
				}
			}
		}
	}

	return result.String()
}

// decodeUnicodeEscapes 解码Unicode转义序列
func (p *PDFFlowProcessor) decodeUnicodeEscapes(text string) string {
	result := strings.Builder{}
	i := 0

	for i < len(text) {
		if i+5 < len(text) && text[i:i+2] == "\\u" {
			// Unicode转义序列 \uXXXX
			unicodeStr := text[i+2 : i+6]
			var charCode int
			if n, err := fmt.Sscanf(unicodeStr, "%04x", &charCode); err == nil && n == 1 {
				result.WriteRune(rune(charCode))
				i += 6
				continue
			}
		} else if i+9 < len(text) && text[i:i+2] == "\\U" {
			// 扩展Unicode转义序列 \UXXXXXXXX
			unicodeStr := text[i+2 : i+10]
			var charCode int
			if n, err := fmt.Sscanf(unicodeStr, "%08x", &charCode); err == nil && n == 1 {
				result.WriteRune(rune(charCode))
				i += 10
				continue
			}
		}

		result.WriteByte(text[i])
		i++
	}

	return result.String()
}

// normalizeWhitespace 规范化空白字符
func (p *PDFFlowProcessor) normalizeWhitespace(text string) string {
	// 将多个连续的空白字符替换为单个空格
	// 但保留换行符的语义
	result := strings.Builder{}
	lastWasSpace := false

	for _, r := range text {
		if r == ' ' || r == '\t' {
			if !lastWasSpace {
				result.WriteRune(' ')
				lastWasSpace = true
			}
		} else if r == '\n' || r == '\r' {
			// 保留换行符，但避免重复
			if !lastWasSpace {
				result.WriteRune(' ') // 将换行转换为空格，保持文本连续性
				lastWasSpace = true
			}
		} else {
			result.WriteRune(r)
			lastWasSpace = false
		}
	}

	return strings.TrimSpace(result.String())
}

// parseTransformMatrix 解析变换矩阵
func (p *PDFFlowProcessor) parseTransformMatrix(operands []string) TransformMatrix {
	if len(operands) < 6 {
		return TransformMatrix{A: 1, D: 1} // 单位矩阵
	}

	a, _ := p.parseFloat(operands[0])
	b, _ := p.parseFloat(operands[1])
	c, _ := p.parseFloat(operands[2])
	d, _ := p.parseFloat(operands[3])
	e, _ := p.parseFloat(operands[4])
	f, _ := p.parseFloat(operands[5])

	return TransformMatrix{A: a, B: b, C: c, D: d, E: e, F: f}
}

// parseFloat 解析浮点数
func (p *PDFFlowProcessor) parseFloat(s string) (float64, error) {
	// 移除可能的括号和其他字符
	s = strings.Trim(s, "() ")

	// 尝试解析
	var result float64
	if n, err := fmt.Sscanf(s, "%f", &result); err == nil && n == 1 {
		return result, nil
	}

	return 0.0, fmt.Errorf("无法解析浮点数: %s", s)
}

// getGraphicsType 获取图形类型
func (p *PDFFlowProcessor) getGraphicsType(operator string) string {
	switch operator {
	case "m":
		return "moveto"
	case "l":
		return "lineto"
	case "c":
		return "curveto"
	case "re":
		return "rectangle"
	case "S", "s":
		return "stroke"
	case "f", "F", "f*":
		return "fill"
	case "B", "B*", "b", "b*":
		return "fill_stroke"
	default:
		return "unknown"
	}
}

// parsePathFromOperands 从操作数解析路径
func (p *PDFFlowProcessor) parsePathFromOperands(operands []string, operator string) []PathCommand {
	var commands []PathCommand

	switch operator {
	case "m":
		if len(operands) >= 2 {
			x, _ := p.parseFloat(operands[0])
			y, _ := p.parseFloat(operands[1])
			commands = append(commands, PathCommand{
				Command: "m",
				Points:  []float64{x, y},
			})
		}
	case "l":
		if len(operands) >= 2 {
			x, _ := p.parseFloat(operands[0])
			y, _ := p.parseFloat(operands[1])
			commands = append(commands, PathCommand{
				Command: "l",
				Points:  []float64{x, y},
			})
		}
	case "re":
		if len(operands) >= 4 {
			x, _ := p.parseFloat(operands[0])
			y, _ := p.parseFloat(operands[1])
			w, _ := p.parseFloat(operands[2])
			h, _ := p.parseFloat(operands[3])
			commands = append(commands, PathCommand{
				Command: "re",
				Points:  []float64{x, y, w, h},
			})
		}
	}

	return commands
}

// isFormula 检测是否为数学公式
func (p *PDFFlowProcessor) isFormula(text, fontName string) bool {
	// 检查字体名称
	if strings.Contains(strings.ToLower(fontName), "math") ||
		strings.Contains(strings.ToLower(fontName), "symbol") {
		return true
	}

	// 检查数学符号
	mathSymbols := []string{
		"∫", "∑", "∏", "√", "∞", "α", "β", "γ", "δ", "ε", "θ", "λ", "μ", "π", "σ", "φ", "ψ", "ω",
		"≤", "≥", "≠", "≈", "∈", "∉", "⊂", "⊃", "∪", "∩", "∧", "∨", "¬", "→", "↔", "∀", "∃",
		"±", "×", "÷", "∂", "∇", "∆", "∝", "∴", "∵", "⊥", "∥", "°", "′", "″",
	}

	for _, symbol := range mathSymbols {
		if strings.Contains(text, symbol) {
			return true
		}
	}

	return false
}

// detectLanguage 检测语言
func (p *PDFFlowProcessor) detectLanguage(text string) string {
	// 简单的语言检测
	UniCount := 0
	totalCount := 0

	for _, r := range text {
		totalCount++
		if r >= 0x4e00 && r <= 0x9fff {
			UniCount++
		}
	}

	if totalCount > 0 && float64(UniCount)/float64(totalCount) > 0.3 {
		return "zh"
	}

	return "en"
}

// extractAnnotations 提取注释
func (p *PDFFlowProcessor) extractAnnotations(ctx *model.Context, pageDict types.Dict, pageFlow *PDFPageFlow) error {
	// 简化实现，实际需要更复杂的注释解析
	return nil
}

// saveFlowData 保存流数据到临时目录
func (p *PDFFlowProcessor) saveFlowData() error {
	startTime := time.Now()
	flowDataPath := filepath.Join(p.workDir, "flow_data.json")

	data, err := json.MarshalIndent(p.flowData, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化流数据失败: %w", err)
	}

	if err := os.WriteFile(flowDataPath, data, 0644); err != nil {
		return fmt.Errorf("保存流数据文件失败: %w", err)
	}

	duration := time.Since(startTime)
	p.logger.LogOperationTiming("保存流数据", duration)
	p.logger.LogFileOperation("保存流数据", flowDataPath, int64(len(data)))

	p.logger.Info("流数据已保存", map[string]interface{}{
		"文件路径": flowDataPath,
		"文件大小": p.logger.formatBytes(int64(len(data))),
		"耗时":   duration.String(),
	})

	return nil
}

// loadFlowData 从临时目录加载流数据
func (p *PDFFlowProcessor) loadFlowData() error {
	startTime := time.Now()
	flowDataPath := filepath.Join(p.workDir, "flow_data.json")

	data, err := os.ReadFile(flowDataPath)
	if err != nil {
		return fmt.Errorf("读取流数据文件失败: %w", err)
	}

	if err := json.Unmarshal(data, &p.flowData); err != nil {
		return fmt.Errorf("反序列化流数据失败: %w", err)
	}

	duration := time.Since(startTime)
	p.logger.LogOperationTiming("加载流数据", duration)
	p.logger.LogFileOperation("加载流数据", flowDataPath, int64(len(data)))

	p.logger.Info("流数据已加载", map[string]interface{}{
		"文件路径": flowDataPath,
		"文件大小": p.logger.formatBytes(int64(len(data))),
		"页数":   len(p.flowData.Pages),
		"耗时":   duration.String(),
	})

	return nil
}

// extractResources 提取资源文件
func (p *PDFFlowProcessor) extractResources() error {
	startTime := time.Now()
	resourcesDir := filepath.Join(p.workDir, "resources")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		return fmt.Errorf("创建资源目录失败: %w", err)
	}

	// 这里可以提取字体、图像等资源文件到临时目录
	// 简化实现，实际需要从PDF中提取嵌入的资源

	duration := time.Since(startTime)
	p.logger.LogOperationTiming("提取资源文件", duration)

	p.logger.Info("资源文件提取完成", map[string]interface{}{
		"资源目录": resourcesDir,
		"耗时":   duration.String(),
	})

	return nil
}

// findBestTranslation 查找最佳翻译 - 改进版本
func (p *PDFFlowProcessor) findBestTranslation(text string, translations map[string]string) string {
	// 跳过空文本或过短的文本
	cleanText := strings.TrimSpace(text)
	if len(cleanText) < 3 {
		return ""
	}

	// 1. 精确匹配
	if translation, exists := translations[text]; exists {
		p.logger.Debug("找到精确匹配", map[string]interface{}{
			"原文": text,
			"翻译": translation,
		})
		return translation
	}

	// 2. 清理后的精确匹配
	cleanText = p.normalizeText(text)
	for original, translation := range translations {
		if p.normalizeText(original) == cleanText {
			p.logger.Debug("找到标准化匹配", map[string]interface{}{
				"原文":  text,
				"标准化": cleanText,
				"翻译":  translation,
			})
			return translation
		}
	}

	// 3. 改进的相似度匹配 - 更严格的匹配
	bestMatch := ""
	bestScore := 0.0
	bestOriginal := ""

	for original, translation := range translations {
		// 只对长度相近的文本进行相似度计算
		lenRatio := float64(len(cleanText)) / float64(len(original))
		if lenRatio < 0.3 || lenRatio > 3.0 {
			continue // 长度差异太大，跳过
		}

		score := p.calculateSimilarity(cleanText, original)
		if score > bestScore && score > 0.8 { // 提高阈值以确保准确性
			bestScore = score
			bestMatch = translation
			bestOriginal = original
		}
	}

	if bestMatch != "" {
		p.logger.Debug("找到相似度匹配", map[string]interface{}{
			"原文":  text,
			"匹配源": bestOriginal,
			"翻译":  bestMatch,
			"相似度": fmt.Sprintf("%.2f", bestScore),
		})
		return bestMatch
	}

	// 4. 包含关系匹配 - 更严格的条件
	for original, translation := range translations {
		// 检查当前文本是否是原文的主要部分
		if len(cleanText) > 10 && len(original) > 10 {
			if strings.Contains(original, cleanText) {
				// 确保匹配的部分占原文的主要部分
				if float64(len(cleanText))/float64(len(original)) > 0.6 {
					p.logger.Debug("找到主要部分匹配", map[string]interface{}{
						"原文":  text,
						"源文本": original,
						"翻译":  translation,
						"匹配率": fmt.Sprintf("%.1f%%", float64(len(cleanText))/float64(len(original))*100),
					})
					return translation
				}
			}
		}
	}

	// 5. 关键短语匹配 - 检查是否包含相同的关键短语
	for original, translation := range translations {
		if p.hasSignificantOverlap(cleanText, original) {
			p.logger.Debug("找到关键短语匹配", map[string]interface{}{
				"原文":   text,
				"关键词源": original,
				"翻译":   translation,
			})
			return translation
		}
	}

	return ""
}

// calculateSimilarity 计算文本相似度
func (p *PDFFlowProcessor) calculateSimilarity(text1, text2 string) float64 {
	norm1 := p.normalizeText(text1)
	norm2 := p.normalizeText(text2)

	if norm1 == norm2 {
		return 1.0
	}

	// 计算最长公共子序列的相似度
	lcs := p.longestCommonSubsequence(norm1, norm2)
	maxLen := len(norm1)
	if len(norm2) > maxLen {
		maxLen = len(norm2)
	}

	if maxLen == 0 {
		return 0.0
	}

	return float64(lcs) / float64(maxLen)
}

// longestCommonSubsequence 计算最长公共子序列长度
func (p *PDFFlowProcessor) longestCommonSubsequence(s1, s2 string) int {
	m, n := len(s1), len(s2)
	if m == 0 || n == 0 {
		return 0
	}

	// 动态规划表
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if s1[i-1] == s2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	return dp[m][n]
}

// containsKeywords 检查是否包含关键词
func (p *PDFFlowProcessor) containsKeywords(text, source string) bool {
	// 提取关键词（长度大于3的单词）
	words := strings.Fields(source)
	for _, word := range words {
		if len(word) > 3 && strings.Contains(strings.ToLower(text), strings.ToLower(word)) {
			return true
		}
	}
	return false
}

// extractWords 提取文本中的单词
func (p *PDFFlowProcessor) extractWords(text string) []string {
	// 标准化文本
	normalized := p.normalizeText(text)

	// 分割单词（支持英文和通用）
	var words []string
	var currentWord strings.Builder

	for _, r := range normalized {
		if p.isWordChar(r) {
			currentWord.WriteRune(r)
		} else {
			if currentWord.Len() > 0 {
				word := currentWord.String()
				if len(word) > 1 { // 只保留长度大于1的单词
					words = append(words, word)
				}
				currentWord.Reset()
			}
		}
	}

	// 处理最后一个单词
	if currentWord.Len() > 0 {
		word := currentWord.String()
		if len(word) > 1 {
			words = append(words, word)
		}
	}

	return words
}

// isWordChar 检查字符是否为单词字符
func (p *PDFFlowProcessor) isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		p.isCJK(r)
}

// countCommonWords 计算两个单词列表的共同单词数量
func (p *PDFFlowProcessor) countCommonWords(words1, words2 []string) int {
	wordSet := make(map[string]bool)
	for _, word := range words2 {
		wordSet[strings.ToLower(word)] = true
	}

	count := 0
	for _, word := range words1 {
		if wordSet[strings.ToLower(word)] {
			count++
		}
	}

	return count
}

// max 返回两个整数的最大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// normalizeText 标准化文本
func (p *PDFFlowProcessor) normalizeText(text string) string {
	// 移除空白字符
	text = strings.ReplaceAll(text, " ", "")
	text = strings.ReplaceAll(text, "\t", "")
	text = strings.ReplaceAll(text, "\n", "")
	text = strings.ReplaceAll(text, "\r", "")

	// 统一连字符
	text = strings.ReplaceAll(text, "ﬁ", "fi")
	text = strings.ReplaceAll(text, "ﬂ", "fl")
	text = strings.ReplaceAll(text, "ﬀ", "ff")

	return strings.ToLower(text)
}

// hasSignificantOverlap 检查两个文本是否有显著重叠
func (p *PDFFlowProcessor) hasSignificantOverlap(text1, text2 string) bool {
	// 提取关键词（长度大于3的单词）
	words1 := p.extractSignificantWords(text1)
	words2 := p.extractSignificantWords(text2)

	if len(words1) == 0 || len(words2) == 0 {
		return false
	}

	// 计算重叠的关键词数量
	overlap := 0
	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if strings.EqualFold(word1, word2) {
				overlap++
				break
			}
		}
	}

	// 如果重叠的关键词占较小集合的50%以上，认为有显著重叠
	minWords := len(words1)
	if len(words2) < minWords {
		minWords = len(words2)
	}

	return float64(overlap)/float64(minWords) > 0.5
}

// extractSignificantWords 提取有意义的单词
func (p *PDFFlowProcessor) extractSignificantWords(text string) []string {
	// 分割单词
	words := strings.FieldsFunc(text, func(c rune) bool {
		return !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9'))
	})

	var significantWords []string
	for _, word := range words {
		// 只保留长度大于3的单词，过滤常见停用词
		if len(word) > 3 && !p.isStopWord(word) {
			significantWords = append(significantWords, strings.ToLower(word))
		}
	}

	return significantWords
}

// isStopWord 检查是否为停用词
func (p *PDFFlowProcessor) isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true,
		"not": true, "you": true, "all": true, "can": true, "had": true,
		"was": true, "one": true, "our": true, "out": true,
		"day": true, "get": true, "has": true, "him": true, "his": true,
		"how": true, "its": true, "may": true, "new": true, "now": true,
		"old": true, "see": true, "two": true, "who": true, "boy": true,
		"did": true, "she": true, "use": true, "way": true,
		"many": true, "then": true, "them": true, "were": true,
		"with": true, "have": true, "this": true, "will": true, "your": true,
		"from": true, "they": true, "know": true, "want": true, "been": true,
		"good": true, "much": true, "some": true, "time": true, "very": true,
		"when": true, "come": true, "here": true, "just": true, "like": true,
		"long": true, "make": true, "said": true, "take": true, "than": true,
		"only": true, "over": true, "think": true, "also": true, "back": true,
		"after": true, "first": true, "year": true,
	}
	return stopWords[strings.ToLower(word)]
}

// calculateTextBounds 计算文本边界
func (p *PDFFlowProcessor) calculateTextBounds(text string, font FontFlow) (BoundingBox, error) {
	// 改进的文本边界计算

	// 基础字符宽度（根据字体大小调整）
	var charWidth float64
	if p.containsUni(text) {
		// 通用字符通常更宽
		charWidth = font.Size * 0.8
	} else {
		// 英文字符
		charWidth = font.Size * 0.6
	}

	// 计算文本长度，考虑换行
	lines := strings.Split(text, "\n")
	maxLineLength := 0
	for _, line := range lines {
		if len(line) > maxLineLength {
			maxLineLength = len(line)
		}
	}

	// 限制最大宽度，避免文本过宽
	width := float64(maxLineLength) * charWidth
	maxAllowedWidth := 500.0 // 最大允许宽度
	if width > maxAllowedWidth {
		width = maxAllowedWidth
	}

	// 高度计算，考虑行数
	lineHeight := font.Size * 1.2 // 行高
	height := float64(len(lines)) * lineHeight

	// 最小高度
	if height < font.Size {
		height = font.Size
	}

	p.logger.Debug("计算文本边界", map[string]interface{}{
		"文本长度": len(text),
		"行数":   len(lines),
		"最长行":  maxLineLength,
		"字体大小": font.Size,
		"计算宽度": fmt.Sprintf("%.2f", width),
		"计算高度": fmt.Sprintf("%.2f", height),
	})

	return BoundingBox{
		Width:  width,
		Height: height,
	}, nil
}

// recalculateLayout 重新计算布局
func (p *PDFFlowProcessor) recalculateLayout() error {
	startTime := time.Now()
	p.logger.Info("开始重新计算布局", nil)

	totalElements := 0
	recalculatedElements := 0

	for pageIdx := range p.flowData.Pages {
		page := &p.flowData.Pages[pageIdx]
		pageElements := 0
		pageRecalculated := 0

		// 重新计算每个文本元素的位置
		for elemIdx := range page.TextElements {
			element := &page.TextElements[elemIdx]
			totalElements++
			pageElements++

			// 根据新的文本内容重新计算边界
			newBounds, err := p.calculateTextBounds(element.Content, element.Font)
			if err != nil {
				p.logger.Warn("重新计算文本边界失败", map[string]interface{}{
					"页码":   page.PageNumber,
					"元素ID": element.ID,
					"错误":   err.Error(),
				})
				continue
			}

			// 检查边界是否发生变化
			if newBounds.Width != element.BoundingBox.Width || newBounds.Height != element.BoundingBox.Height {
				p.logger.Debug("文本边界发生变化", map[string]interface{}{
					"页码":   page.PageNumber,
					"元素ID": element.ID,
					"原宽度":  fmt.Sprintf("%.2f", element.BoundingBox.Width),
					"新宽度":  fmt.Sprintf("%.2f", newBounds.Width),
					"原高度":  fmt.Sprintf("%.2f", element.BoundingBox.Height),
					"新高度":  fmt.Sprintf("%.2f", newBounds.Height),
				})
				recalculatedElements++
				pageRecalculated++
			}

			element.BoundingBox = newBounds
		}

		p.logger.Debug("页面布局重新计算完成", map[string]interface{}{
			"页码":    page.PageNumber,
			"总元素数":  pageElements,
			"重新计算数": pageRecalculated,
		})
	}

	duration := time.Since(startTime)
	p.logger.LogOperationTiming("重新计算布局", duration, map[string]interface{}{
		"总元素数":  totalElements,
		"重新计算数": recalculatedElements,
	})

	p.logger.Info("布局重新计算完成", map[string]interface{}{
		"总元素数":  totalElements,
		"重新计算数": recalculatedElements,
		"变化率":   fmt.Sprintf("%.1f%%", float64(recalculatedElements)/float64(totalElements)*100),
		"耗时":    duration.String(),
	})

	return nil
}

// setupFonts 设置字体支持
func (p *PDFFlowProcessor) setupFonts(pdf *gofpdf.Fpdf) error {
	// 添加通用字体支持
	fontDetector := NewSystemFontDetector()
	fontPath := fontDetector.GetSystemFontPath("zh")

	if fontPath != "" {
		fontName := strings.TrimSuffix(filepath.Base(fontPath), filepath.Ext(fontPath))

		// 确保字体名称是有效的
		if fontName == "" {
			fontName = "SimHei"
		}

		// 添加UTF8字体
		pdf.AddUTF8Font(fontName, "", fontPath)

		if err := pdf.Error(); err != nil {
			log.Printf("警告：添加通用字体失败: %v", err)
			// 尝试使用内置字体作为备用
			p.UniFontName = "Arial"
		} else {
			log.Printf("成功添加通用字体: %s", fontName)
			// 保存字体名称供后续使用
			p.UniFontName = fontName
		}
	} else {
		log.Printf("警告：未找到系统字体，使用默认字体")
		p.UniFontName = "Arial"
	}

	return nil
}

// generatePage 生成页面
func (p *PDFFlowProcessor) generatePage(pdf *gofpdf.Fpdf, page PDFPageFlow) error {
	pdf.AddPage()

	// 设置页面尺寸
	if page.MediaBox.Width > 0 && page.MediaBox.Height > 0 {
		// 这里可以设置自定义页面尺寸，但gofpdf的API有限制
	}

	// 按Y坐标排序文本元素，确保正确的渲染顺序
	sortedTextElements := make([]TextElementFlow, len(page.TextElements))
	copy(sortedTextElements, page.TextElements)

	// 按Y坐标排序（从上到下）- Y值大的在前面
	// 注意：PDF坐标系是从底部开始的，Y值越大越靠上
	for i := 0; i < len(sortedTextElements)-1; i++ {
		for j := i + 1; j < len(sortedTextElements); j++ {
			if sortedTextElements[i].Position.Y < sortedTextElements[j].Position.Y {
				sortedTextElements[i], sortedTextElements[j] = sortedTextElements[j], sortedTextElements[i]
			}
		}
	}

	// 记录排序后的位置信息
	p.logger.Debug("文本元素排序完成", map[string]interface{}{
		"页码":  page.PageNumber,
		"元素数": len(sortedTextElements),
		"前3个元素位置": func() string {
			if len(sortedTextElements) >= 3 {
				return fmt.Sprintf("Y1:%.1f Y2:%.1f Y3:%.1f",
					sortedTextElements[0].Position.Y,
					sortedTextElements[1].Position.Y,
					sortedTextElements[2].Position.Y)
			}
			return "元素不足3个"
		}(),
	})

	// 渲染文本元素
	for i, element := range sortedTextElements {
		if err := p.renderTextElement(pdf, element, i); err != nil {
			log.Printf("警告：渲染文本元素失败: %v", err)
		}
	}

	// 渲染图像元素
	for _, element := range page.ImageElements {
		if err := p.renderImageElement(pdf, element); err != nil {
			log.Printf("警告：渲染图像元素失败: %v", err)
		}
	}

	// 渲染图形元素
	for _, element := range page.GraphicsElements {
		if err := p.renderGraphicsElement(pdf, element); err != nil {
			log.Printf("警告：渲染图形元素失败: %v", err)
		}
	}

	return nil
}

// renderTextElement 渲染文本元素
func (p *PDFFlowProcessor) renderTextElement(pdf *gofpdf.Fpdf, element TextElementFlow, index int) error {
	// 设置字体
	fontName := "Arial"
	fontSize := element.Font.Size

	// 确保字体大小合理
	if fontSize <= 0 {
		fontSize = 12
	}
	if fontSize > 72 {
		fontSize = 72
	}

	if p.containsUni(element.Content) {
		// 使用已添加的通用字体
		if p.UniFontName != "" && p.UniFontName != "Arial" {
			fontName = p.UniFontName
		} else {
			// 如果没有通用字体，尝试使用Arial作为备用
			fontName = "Arial"
		}
	}

	pdf.SetFont(fontName, "", fontSize)

	// 设置颜色
	if element.Color.Space == "RGB" && len(element.Color.Values) >= 3 {
		r := int(element.Color.Values[0] * 255)
		g := int(element.Color.Values[1] * 255)
		b := int(element.Color.Values[2] * 255)
		pdf.SetTextColor(r, g, b)
	} else {
		// 默认黑色
		pdf.SetTextColor(0, 0, 0)
	}

	// 智能处理文本内容
	content := strings.TrimSpace(element.Content)
	if content == "" {
		return nil // 跳过空内容
	}

	// 处理过长的文本
	maxWidth := element.BoundingBox.Width
	if maxWidth <= 0 {
		maxWidth = 500 // 默认最大宽度
	}

	// 如果文本太长，进行智能截断或分行处理
	if len(content) > 200 { // 如果文本超过200个字符
		// 尝试在合适的位置截断
		if strings.Contains(content, "\n") {
			// 如果包含换行符，只取第一行
			lines := strings.Split(content, "\n")
			content = lines[0]
		} else {
			// 智能截断：优先在句号、逗号等标点处截断
			truncatePos := 150
			for i := 100; i < len(content) && i < 200; i++ {
				char := rune(content[i])
				if char == '。' || char == '，' || char == '.' || char == ',' || char == ' ' || char == '；' {
					truncatePos = i + 1
					break
				}
			}
			if truncatePos < len(content) {
				content = content[:truncatePos] + "..."
			}
		}
	}

	// 检查文本宽度是否超出边界
	textWidth := pdf.GetStringWidth(content)
	if textWidth > maxWidth && maxWidth > 50 {
		// 如果文本宽度超出，尝试缩小字体
		newSize := fontSize * (maxWidth / textWidth) * 0.85 // 留15%边距
		if newSize < 8 {                                    // 最小字体大小
			newSize = 8
		}
		if newSize < fontSize {
			pdf.SetFont(fontName, "", newSize)
			p.logger.Debug("调整字体大小", map[string]interface{}{
				"原始大小": fontSize,
				"新大小":  newSize,
				"文本宽度": textWidth,
				"最大宽度": maxWidth,
			})
		}
	}

	// 智能位置调整 - 避免文本重叠
	posX := element.Position.X
	posY := element.Position.Y

	// 确保位置在合理范围内
	if posX < 0 {
		posX = 50
	}
	if posY < 0 {
		posY = 50
	}
	if posX > 500 {
		posX = 500
	}
	if posY > 750 {
		posY = 750
	}

	// 如果位置看起来不合理（比如都堆叠在同一位置），进行调整
	if (posX == 72 && posY == 720) || (posX == 108 && posY > 700) {
		// 这是默认位置，需要根据索引进行调整
		posX = 50 + float64(index%2)*250 // 水平分布：50, 300
		posY = 750 - float64(index/2)*20 // 垂直分布：每2个元素下移20点

		p.logger.Debug("调整重叠位置", map[string]interface{}{
			"索引":   index,
			"原位置X": element.Position.X,
			"原位置Y": element.Position.Y,
			"新位置X": posX,
			"新位置Y": posY,
		})
	}

	// 设置位置并输出文本
	pdf.SetXY(posX, posY)

	// 计算合适的单元格尺寸
	cellWidth := element.BoundingBox.Width
	if cellWidth <= 0 {
		cellWidth = pdf.GetStringWidth(content) + 10
	}
	cellHeight := element.BoundingBox.Height
	if cellHeight <= 0 {
		cellHeight = fontSize * 1.2
	}

	pdf.Cell(cellWidth, cellHeight, content)

	return nil
}

// renderImageElement 渲染图像元素
func (p *PDFFlowProcessor) renderImageElement(pdf *gofpdf.Fpdf, element ImageElementFlow) error {
	// 尝试渲染图像元素
	p.logger.Debug("尝试渲染图像元素", map[string]interface{}{
		"图像名称": element.Name,
		"位置X":  element.Position.X,
		"位置Y":  element.Position.Y,
		"宽度":   element.Size.Width,
		"高度":   element.Size.Height,
	})

	// 确保图像有合理的尺寸
	width := element.Size.Width
	height := element.Size.Height

	if width <= 0 {
		width = 50
	}
	if height <= 0 {
		height = 50
	}

	// 限制最大尺寸，避免图像过大
	maxWidth := 200.0
	maxHeight := 200.0

	if width > maxWidth {
		height = height * (maxWidth / width)
		width = maxWidth
	}
	if height > maxHeight {
		width = width * (maxHeight / height)
		height = maxHeight
	}

	// 简化实现：添加占位符文本表示图像
	if width > 10 && height > 10 {
		// 设置边框颜色
		pdf.SetDrawColor(200, 200, 200)
		pdf.SetLineWidth(0.5)

		// 绘制图像占位框
		pdf.Rect(element.Position.X, element.Position.Y, width, height, "D")

		// 添加图像标识文本
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(128, 128, 128)
		pdf.SetXY(element.Position.X+2, element.Position.Y+10)
		pdf.Cell(0, 0, fmt.Sprintf("[图像: %s]", element.Name))
	}

	return nil
}

// renderGraphicsElement 渲染图形元素
func (p *PDFFlowProcessor) renderGraphicsElement(pdf *gofpdf.Fpdf, element GraphicsElementFlow) error {
	// 尝试渲染基本图形元素
	p.logger.Debug("尝试渲染图形元素", map[string]interface{}{
		"类型":    element.Type,
		"路径命令数": len(element.Path),
	})

	// 设置绘图样式
	if element.Style.LineWidth > 0 {
		pdf.SetLineWidth(element.Style.LineWidth)
	} else {
		pdf.SetLineWidth(0.5)
	}

	// 设置描边颜色
	if element.Style.StrokeColor.Space == "RGB" && len(element.Style.StrokeColor.Values) >= 3 {
		r := int(element.Style.StrokeColor.Values[0] * 255)
		g := int(element.Style.StrokeColor.Values[1] * 255)
		b := int(element.Style.StrokeColor.Values[2] * 255)
		pdf.SetDrawColor(r, g, b)
	} else {
		pdf.SetDrawColor(0, 0, 0) // 默认黑色
	}

	// 设置填充颜色
	if element.Style.FillColor.Space == "RGB" && len(element.Style.FillColor.Values) >= 3 {
		r := int(element.Style.FillColor.Values[0] * 255)
		g := int(element.Style.FillColor.Values[1] * 255)
		b := int(element.Style.FillColor.Values[2] * 255)
		pdf.SetFillColor(r, g, b)
	}

	// 根据类型进行简化渲染
	switch element.Type {
	case "rectangle":
		// 使用边界框绘制矩形
		if element.BoundingBox.Width > 0 && element.BoundingBox.Height > 0 {
			pdf.Rect(element.BoundingBox.X, element.BoundingBox.Y,
				element.BoundingBox.Width, element.BoundingBox.Height, "D")
		}
	case "line":
		// 处理路径命令绘制线条
		for _, pathCmd := range element.Path {
			if pathCmd.Command == "l" || pathCmd.Command == "L" {
				if len(pathCmd.Points) >= 4 {
					x1, y1, x2, y2 := pathCmd.Points[0], pathCmd.Points[1], pathCmd.Points[2], pathCmd.Points[3]
					pdf.Line(x1, y1, x2, y2)
				}
			}
		}
	default:
		p.logger.Debug("渲染基本图形元素", map[string]interface{}{
			"类型": element.Type,
			"边界": fmt.Sprintf("%.1f,%.1f,%.1f,%.1f",
				element.BoundingBox.X, element.BoundingBox.Y,
				element.BoundingBox.Width, element.BoundingBox.Height),
		})
	}

	return nil
}

// containsUni 检测是否包含通用通用
func (p *PDFFlowProcessor) containsUni(text string) bool {
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

// LayoutManager 布局管理器
type LayoutManager struct {
	pageWidth  float64
	pageHeight float64
	usedAreas  []BoundingBox
	currentY   float64
	margin     float64
}

// NewLayoutManager 创建布局管理器
func NewLayoutManager(width, height float64) *LayoutManager {
	return &LayoutManager{
		pageWidth:  width,
		pageHeight: height,
		usedAreas:  make([]BoundingBox, 0),
		currentY:   height - 72, // 从页面顶部开始，留72点边距
		margin:     72,          // 页面边距
	}
}

// AdjustTextPosition 调整文本位置避免重叠
func (lm *LayoutManager) AdjustTextPosition(element TextElementFlow, index int) TextElementFlow {
	adjustedElement := element

	// 如果原始位置无效或者会导致重叠，重新计算位置
	if element.Position.X == 0 && element.Position.Y == 0 {
		// 使用流式布局
		adjustedElement.Position.X = lm.margin
		adjustedElement.Position.Y = lm.currentY

		// 更新当前Y位置
		lm.currentY -= element.BoundingBox.Height + 5 // 5点行间距

		// 如果超出页面底部，重置到顶部
		if lm.currentY < lm.margin {
			lm.currentY = lm.pageHeight - lm.margin
		}
	} else {
		// 检查是否与已有区域重叠
		elementBounds := BoundingBox{
			X:      element.Position.X,
			Y:      element.Position.Y,
			Width:  element.BoundingBox.Width,
			Height: element.BoundingBox.Height,
		}

		if lm.isOverlapping(elementBounds) {
			// 如果重叠，使用流式布局
			adjustedElement.Position.X = lm.margin
			adjustedElement.Position.Y = lm.currentY
			lm.currentY -= element.BoundingBox.Height + 5
		}
	}

	// 记录使用的区域
	usedArea := BoundingBox{
		X:      adjustedElement.Position.X,
		Y:      adjustedElement.Position.Y,
		Width:  adjustedElement.BoundingBox.Width,
		Height: adjustedElement.BoundingBox.Height,
	}
	lm.usedAreas = append(lm.usedAreas, usedArea)

	return adjustedElement
}

// isOverlapping 检查是否与已有区域重叠
func (lm *LayoutManager) isOverlapping(bounds BoundingBox) bool {
	for _, used := range lm.usedAreas {
		if lm.boundsOverlap(bounds, used) {
			return true
		}
	}
	return false
}

// boundsOverlap 检查两个边界框是否重叠
func (lm *LayoutManager) boundsOverlap(a, b BoundingBox) bool {
	return !(a.X+a.Width < b.X || b.X+b.Width < a.X ||
		a.Y+a.Height < b.Y || b.Y+b.Height < a.Y)
}

// mergeAdjacentTextElements 合并相邻的文本元素
func (p *PDFFlowProcessor) mergeAdjacentTextElements(pageFlow *PDFPageFlow) {
	if len(pageFlow.TextElements) <= 1 {
		return
	}

	originalCount := len(pageFlow.TextElements)
	var merged []TextElementFlow
	current := pageFlow.TextElements[0]

	for i := 1; i < len(pageFlow.TextElements); i++ {
		next := pageFlow.TextElements[i]

		// 检查是否应该合并
		if p.shouldMergeTextElements(current, next) {
			// 合并文本内容
			separator := ""

			// 智能添加分隔符
			if p.needsSeparator(current.Content, next.Content) {
				separator = " "
			}

			current.Content += separator + next.Content

			// 更新边界框
			current.BoundingBox.Width = next.BoundingBox.X + next.BoundingBox.Width - current.BoundingBox.X
			if next.BoundingBox.Y < current.BoundingBox.Y {
				current.BoundingBox.Height += current.BoundingBox.Y - next.BoundingBox.Y
				current.BoundingBox.Y = next.BoundingBox.Y
			}
		} else {
			merged = append(merged, current)
			current = next
		}
	}

	merged = append(merged, current)
	pageFlow.TextElements = merged

	p.logger.Debug("文本元素合并完成", map[string]interface{}{
		"页码":    pageFlow.PageNumber,
		"原始数量":  originalCount,
		"合并后数量": len(merged),
		"减少比例":  fmt.Sprintf("%.1f%%", float64(originalCount-len(merged))/float64(originalCount)*100),
	})
}

// shouldMergeTextElements 检查是否应该合并两个文本元素
func (p *PDFFlowProcessor) shouldMergeTextElements(a, b TextElementFlow) bool {
	// 检查字体是否相似
	if !p.isSimilarFont(a.Font, b.Font) {
		return false
	}

	// 检查颜色是否相同
	if !p.isSimilarColor(a.Color, b.Color) {
		return false
	}

	// 检查位置是否相邻
	if !p.isAdjacentPosition(a, b) {
		return false
	}

	// 检查文本内容是否适合合并
	if !p.isContentMergeable(a.Content, b.Content) {
		return false
	}

	return true
}

// isSimilarFont 检查字体是否相似
func (p *PDFFlowProcessor) isSimilarFont(a, b FontFlow) bool {
	// 字体名称必须相同
	if a.Name != b.Name {
		return false
	}

	// 字体大小差异不能太大
	sizeDiff := a.Size - b.Size
	if sizeDiff < 0 {
		sizeDiff = -sizeDiff
	}

	return sizeDiff <= 1.0
}

// isSimilarColor 检查颜色是否相似
func (p *PDFFlowProcessor) isSimilarColor(a, b ColorFlow) bool {
	if a.Space != b.Space {
		return false
	}

	if len(a.Values) != len(b.Values) {
		return false
	}

	// 检查颜色值差异
	for i := 0; i < len(a.Values); i++ {
		diff := a.Values[i] - b.Values[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.1 { // 允许10%的颜色差异
			return false
		}
	}

	return true
}

// isAdjacentPosition 检查位置是否相邻
func (p *PDFFlowProcessor) isAdjacentPosition(a, b TextElementFlow) bool {
	// 垂直距离检查
	yDiff := a.Position.Y - b.Position.Y
	if yDiff < 0 {
		yDiff = -yDiff
	}

	// 如果垂直距离太大，不合并
	if yDiff > a.Font.Size*1.5 {
		return false
	}

	// 水平距离检查
	xGap := b.Position.X - (a.Position.X + a.BoundingBox.Width)
	if xGap < 0 {
		xGap = -xGap
	}

	// 如果水平距离太大，不合并
	if xGap > a.Font.Size*2 {
		return false
	}

	return true
}

// isContentMergeable 检查内容是否适合合并
func (p *PDFFlowProcessor) isContentMergeable(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)

	// 如果任一文本为空，不合并
	if a == "" || b == "" {
		return false
	}

	// 如果任一文本很短，倾向于合并
	if len(a) < 5 || len(b) < 5 {
		return true
	}

	// 如果第一个文本以连字符结尾，合并
	if strings.HasSuffix(a, "-") {
		return true
	}

	// 如果第二个文本以标点符号开始，合并
	if len(b) > 0 {
		firstChar := b[0]
		if firstChar == ',' || firstChar == '.' || firstChar == ';' ||
			firstChar == ':' || firstChar == ')' || firstChar == ']' ||
			firstChar == '}' {
			return true
		}
	}

	// 如果第二个文本以小写字母开始，可能是同一句话的延续
	if len(b) > 0 && b[0] >= 'a' && b[0] <= 'z' {
		return true
	}

	return false
}

// needsSeparator 检查是否需要分隔符
func (p *PDFFlowProcessor) needsSeparator(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)

	if a == "" || b == "" {
		return false
	}

	// 如果第一个文本以连字符结尾，不需要分隔符
	if strings.HasSuffix(a, "-") {
		return false
	}

	// 如果第二个文本以标点符号开始，不需要分隔符
	if len(b) > 0 {
		firstChar := b[0]
		if firstChar == ',' || firstChar == '.' || firstChar == ';' ||
			firstChar == ':' || firstChar == ')' || firstChar == ']' ||
			firstChar == '}' {
			return false
		}
	}

	// 其他情况需要空格分隔符
	return true
}
