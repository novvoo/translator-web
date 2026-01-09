package translator

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFParser PDF解析器
type PDFParser struct {
	FormulaFontRegex *regexp.Regexp
	FormulaCharRegex *regexp.Regexp
}

// TextBlock 文本块
type TextBlock struct {
	Text      string  `json:"text"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	FontSize  float64 `json:"font_size"`
	FontName  string  `json:"font_name"`
	IsFormula bool    `json:"is_formula"`
	PageNum   int     `json:"page_num"`
}

// PDFContent PDF内容
type PDFContent struct {
	TextBlocks []TextBlock       `json:"text_blocks"`
	PageCount  int               `json:"page_count"`
	Metadata   map[string]string `json:"metadata"`
}

// NewPDFParser 创建PDF解析器
func NewPDFParser(formulaFont, formulaChar string) *PDFParser {
	parser := &PDFParser{}

	if formulaFont != "" {
		parser.FormulaFontRegex = regexp.MustCompile(formulaFont)
	}

	if formulaChar != "" {
		parser.FormulaCharRegex = regexp.MustCompile(formulaChar)
	}

	return parser
}

// ParsePDF 解析PDF文件
func (p *PDFParser) ParsePDF(filePath string) (*PDFContent, error) {
	log.Printf("开始解析PDF文件: %s", filePath)

	// 打开PDF文件
	file, reader, err := pdf.Open(filePath)
	if err != nil {
		// 提供更友好的错误信息
		if strings.Contains(err.Error(), "stream not present") {
			return nil, fmt.Errorf("PDF文件格式不受支持或已损坏。此PDF可能使用了特殊编码、加密或压缩方式。建议：1) 尝试使用其他PDF工具重新保存该文件 2) 确保PDF未加密 3) 使用标准PDF格式")
		}
		return nil, fmt.Errorf("打开PDF文件失败: %w", err)
	}
	defer file.Close()

	content := &PDFContent{
		TextBlocks: make([]TextBlock, 0),
		PageCount:  reader.NumPage(),
		Metadata:   make(map[string]string),
	}

	// 提取元数据
	if info := reader.Trailer().Key("Info"); !info.IsNull() {
		content.Metadata = p.extractMetadata(info)
	}

	// 逐页解析
	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		blocks, err := p.extractTextBlocks(page, pageNum)
		if err != nil {
			log.Printf("警告：解析第%d页失败: %v", pageNum, err)
			continue
		}

		content.TextBlocks = append(content.TextBlocks, blocks...)
	}

	log.Printf("PDF解析完成，共%d页，提取%d个文本块", content.PageCount, len(content.TextBlocks))
	return content, nil
}

// extractTextBlocks 提取页面文本块
func (p *PDFParser) extractTextBlocks(page pdf.Page, pageNum int) ([]TextBlock, error) {
	var blocks []TextBlock

	// 获取页面内容，添加错误处理
	defer func() {
		if r := recover(); r != nil {
			log.Printf("警告：提取第%d页文本块时发生panic: %v", pageNum, r)
		}
	}()

	content := page.Content()
	if content.Text == nil {
		return blocks, nil
	}

	// 遍历文本对象
	for _, text := range content.Text {
		block := TextBlock{
			Text:     strings.TrimSpace(text.S),
			X:        text.X,
			Y:        text.Y,
			FontSize: text.FontSize,
			FontName: text.Font,
			PageNum:  pageNum,
		}

		// 跳过空文本
		if block.Text == "" {
			continue
		}

		// 检测是否为数学公式
		block.IsFormula = p.isFormula(block.Text, block.FontName)

		blocks = append(blocks, block)
	}

	// 合并相邻的文本块
	return p.mergeTextBlocks(blocks), nil
}

// isFormula 检测是否为数学公式
func (p *PDFParser) isFormula(text, fontName string) bool {
	// 检查字体名称
	if p.FormulaFontRegex != nil && p.FormulaFontRegex.MatchString(fontName) {
		return true
	}

	// 检查字符内容
	if p.FormulaCharRegex != nil && p.FormulaCharRegex.MatchString(text) {
		return true
	}

	// 常见数学符号检测
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

	// 检测数学表达式模式
	mathPatterns := []string{
		`\d+\s*[+\-*/=]\s*\d+`,           // 简单算术表达式
		`[a-zA-Z]\s*[+\-*/=]\s*[a-zA-Z]`, // 代数表达式
		`\d+\^\d+`,                       // 指数
		`\d+_\d+`,                        // 下标
		`\([^)]*\)\s*[+\-*/=]`,           // 括号表达式
	}

	for _, pattern := range mathPatterns {
		if matched, _ := regexp.MatchString(pattern, text); matched {
			return true
		}
	}

	return false
}

// mergeTextBlocks 合并相邻的文本块
func (p *PDFParser) mergeTextBlocks(blocks []TextBlock) []TextBlock {
	if len(blocks) <= 1 {
		return blocks
	}

	var merged []TextBlock
	current := blocks[0]

	for i := 1; i < len(blocks); i++ {
		next := blocks[i]

		// 改进合并逻辑：更宽松的合并条件
		if p.canMerge(current, next) {
			// 智能添加空格：如果两个文本块之间有明显间隔，添加空格
			separator := ""
			if next.X > current.X+current.Width+2 { // 如果有明显间隔
				separator = " "
			}
			current.Text += separator + next.Text
			current.Width = next.X + next.Width - current.X
		} else {
			merged = append(merged, current)
			current = next
		}
	}

	merged = append(merged, current)

	// 进行第二轮合并：合并语义相关的文本块
	return p.mergeSemanticBlocks(merged)
}

// canMerge 检查两个文本块是否可以合并
func (p *PDFParser) canMerge(a, b TextBlock) bool {
	// 必须在同一页
	if a.PageNum != b.PageNum {
		return false
	}

	// 必须是相同类型（都是公式或都不是公式）
	if a.IsFormula != b.IsFormula {
		return false
	}

	// Y坐标相近（同一行）
	yDiff := a.Y - b.Y
	if yDiff < 0 {
		yDiff = -yDiff
	}
	if yDiff > a.FontSize*0.5 {
		return false
	}

	// X坐标相邻（水平距离不超过一个字符宽度）
	xGap := b.X - (a.X + a.Width)
	if xGap < 0 || xGap > a.FontSize {
		return false
	}

	// 字体大小相近
	sizeDiff := a.FontSize - b.FontSize
	if sizeDiff < 0 {
		sizeDiff = -sizeDiff
	}
	if sizeDiff > 1.0 {
		return false
	}

	return true
}

// extractMetadata 提取PDF元数据
func (p *PDFParser) extractMetadata(info pdf.Value) map[string]string {
	metadata := make(map[string]string)

	if title := info.Key("Title"); !title.IsNull() {
		if titleText := title.Text(); titleText != "" {
			metadata["title"] = titleText
		}
	}

	if author := info.Key("Author"); !author.IsNull() {
		if authorText := author.Text(); authorText != "" {
			metadata["author"] = authorText
		}
	}

	if subject := info.Key("Subject"); !subject.IsNull() {
		if subjectText := subject.Text(); subjectText != "" {
			metadata["subject"] = subjectText
		}
	}

	if creator := info.Key("Creator"); !creator.IsNull() {
		if creatorText := creator.Text(); creatorText != "" {
			metadata["creator"] = creatorText
		}
	}

	return metadata
}

// GetTextForTranslation 获取用于翻译的文本
func (p *PDFParser) GetTextForTranslation(content *PDFContent) []string {
	var texts []string

	for _, block := range content.TextBlocks {
		// 跳过数学公式（可选择性翻译）
		if block.IsFormula {
			continue
		}

		// 过滤掉太短的文本
		if len(strings.TrimSpace(block.Text)) < 3 {
			continue
		}

		texts = append(texts, block.Text)
	}

	return texts
}

// ApplyTranslations 应用翻译结果
func (p *PDFParser) ApplyTranslations(content *PDFContent, translations map[string]string) {
	for i := range content.TextBlocks {
		block := &content.TextBlocks[i]
		if translation, exists := translations[block.Text]; exists {
			block.Text = translation
		}
	}
}

// mergeSemanticBlocks 合并语义相关的文本块
func (p *PDFParser) mergeSemanticBlocks(blocks []TextBlock) []TextBlock {
	if len(blocks) <= 1 {
		return blocks
	}

	var merged []TextBlock
	current := blocks[0]

	for i := 1; i < len(blocks); i++ {
		next := blocks[i]

		// 检查是否应该合并为一个语义单元
		if p.shouldMergeSemantically(current, next) {
			// 合并文本，保持适当的间距
			separator := " "
			if strings.HasSuffix(current.Text, "-") ||
				strings.HasPrefix(next.Text, "-") ||
				len(current.Text) < 3 || len(next.Text) < 3 {
				separator = ""
			}
			current.Text += separator + next.Text
			current.Width = next.X + next.Width - current.X
		} else {
			merged = append(merged, current)
			current = next
		}
	}

	merged = append(merged, current)
	return merged
}

// shouldMergeSemantically 检查是否应该语义合并
func (p *PDFParser) shouldMergeSemantically(a, b TextBlock) bool {
	// 必须在同一页
	if a.PageNum != b.PageNum {
		return false
	}

	// 必须是相同类型
	if a.IsFormula != b.IsFormula {
		return false
	}

	// 垂直距离检查：如果在合理的行间距内
	yDiff := a.Y - b.Y
	if yDiff < 0 {
		yDiff = -yDiff
	}

	// 如果垂直距离太大，不合并
	if yDiff > a.FontSize*2 {
		return false
	}

	// 水平距离检查：如果距离合理
	xGap := b.X - (a.X + a.Width)
	if xGap < 0 {
		xGap = -xGap
	}

	// 如果水平距离太大，不合并
	if xGap > a.FontSize*3 {
		return false
	}

	// 文本长度检查：如果任一文本块很短，倾向于合并
	if len(a.Text) < 10 || len(b.Text) < 10 {
		return true
	}

	// 连字符检查：如果前一个文本以连字符结尾，合并
	if strings.HasSuffix(strings.TrimSpace(a.Text), "-") {
		return true
	}

	// 标点符号检查：如果后一个文本以标点开始，合并
	nextText := strings.TrimSpace(b.Text)
	if len(nextText) > 0 {
		firstChar := nextText[0]
		if firstChar == ',' || firstChar == '.' || firstChar == ';' ||
			firstChar == ':' || firstChar == ')' || firstChar == ']' ||
			firstChar == '}' {
			return true
		}
	}

	// 大小写检查：如果后一个文本以小写字母开始，可能是同一句话的延续
	if len(nextText) > 0 && nextText[0] >= 'a' && nextText[0] <= 'z' {
		return true
	}

	return false
}
