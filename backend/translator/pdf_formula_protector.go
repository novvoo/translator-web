package translator

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// FormulaProtector 公式保护器
// 实现pdf2zh的公式占位符技术
type FormulaProtector struct {
	placeholders map[string]*FormulaPlaceholder
	detector     *EnhancedFormulaDetector
	nextID       int
}

// FormulaPlaceholder 公式占位符
type FormulaPlaceholder struct {
	ID          string          // "formula_0"
	Placeholder string          // "{v0}"
	Original    TextElementFlow // 原始文本元素
	BoundingBox BoundingBox     // 边界框
	Type        string          // "inline", "display", "equation"
}

// EnhancedFormulaDetector 增强的公式检测器
type EnhancedFormulaDetector struct {
	// 数学字体模式
	mathFontPatterns []*regexp.Regexp
	
	// 数学符号集合
	mathSymbols map[rune]bool
	
	// 符号密度阈值
	symbolDensityThreshold float64
	
	// LaTeX命令模式
	latexCommandPattern *regexp.Regexp
}

// NewFormulaProtector 创建公式保护器
func NewFormulaProtector() *FormulaProtector {
	return &FormulaProtector{
		placeholders: make(map[string]*FormulaPlaceholder),
		detector:     NewEnhancedFormulaDetector(),
		nextID:       0,
	}
}

// NewEnhancedFormulaDetector 创建增强的公式检测器
func NewEnhancedFormulaDetector() *EnhancedFormulaDetector {
	detector := &EnhancedFormulaDetector{
		mathFontPatterns:       make([]*regexp.Regexp, 0),
		mathSymbols:            make(map[rune]bool),
		symbolDensityThreshold: 0.3, // 30%的字符是数学符号
	}
	
	// 编译数学字体模式
	mathFontNames := []string{
		`(?i)cmmi`,      // Computer Modern Math Italic
		`(?i)cmsy`,      // Computer Modern Math Symbols
		`(?i)cmex`,      // Computer Modern Math Extension
		`(?i)msam`,      // AMS Math Symbols A
		`(?i)msbm`,      // AMS Math Symbols B
		`(?i)eufm`,      // Euler Fraktur Medium
		`(?i)math`,      // 通用数学字体
		`(?i)symbol`,    // Symbol字体
		`(?i)mtextra`,   // Math Extra
		`(?i)stix`,      // STIX字体
		`(?i)cambria.*math`, // Cambria Math
	}
	
	for _, pattern := range mathFontNames {
		if re, err := regexp.Compile(pattern); err == nil {
			detector.mathFontPatterns = append(detector.mathFontPatterns, re)
		}
	}
	
	// 初始化数学符号集合
	detector.initMathSymbols()
	
	// LaTeX命令模式
	detector.latexCommandPattern = regexp.MustCompile(`\\[a-zA-Z]+`)
	
	return detector
}

// initMathSymbols 初始化数学符号集合
func (efd *EnhancedFormulaDetector) initMathSymbols() {
	// 希腊字母
	greekLetters := []rune{
		'α', 'β', 'γ', 'δ', 'ε', 'ζ', 'η', 'θ', 'ι', 'κ', 'λ', 'μ',
		'ν', 'ξ', 'ο', 'π', 'ρ', 'σ', 'τ', 'υ', 'φ', 'χ', 'ψ', 'ω',
		'Α', 'Β', 'Γ', 'Δ', 'Ε', 'Ζ', 'Η', 'Θ', 'Ι', 'Κ', 'Λ', 'Μ',
		'Ν', 'Ξ', 'Ο', 'Π', 'Ρ', 'Σ', 'Τ', 'Υ', 'Φ', 'Χ', 'Ψ', 'Ω',
	}
	
	// 数学运算符
	mathOperators := []rune{
		'∫', '∑', '∏', '∐', '∂', '∇', '√', '∛', '∜',
		'∞', '∝', '∴', '∵', '∀', '∃', '∄', '∅', '∈', '∉', '∋', '∌',
		'⊂', '⊃', '⊄', '⊅', '⊆', '⊇', '⊈', '⊉', '⊊', '⊋',
		'∪', '∩', '∧', '∨', '¬', '⊕', '⊗', '⊙',
		'≤', '≥', '≠', '≈', '≡', '≢', '≃', '≄', '≅', '≆', '≇', '≉',
		'→', '←', '↔', '⇒', '⇐', '⇔', '↦', '↪', '↩',
		'⊥', '∥', '∦', '∠', '∡', '∢', '⊿',
		'±', '∓', '×', '÷', '∗', '∘', '∙', '⋅',
		'°', '′', '″', '‴',
	}
	
	// 数学括号和分隔符
	mathBrackets := []rune{
		'⟨', '⟩', '⟪', '⟫', '⟬', '⟭', '⟮', '⟯',
		'⌈', '⌉', '⌊', '⌋', '⌜', '⌝', '⌞', '⌟',
		'⎰', '⎱', '⎲', '⎳',
	}
	
	// 添加所有符号到集合
	for _, r := range greekLetters {
		efd.mathSymbols[r] = true
	}
	for _, r := range mathOperators {
		efd.mathSymbols[r] = true
	}
	for _, r := range mathBrackets {
		efd.mathSymbols[r] = true
	}
}

// IsFormula 检测文本元素是否为公式
func (efd *EnhancedFormulaDetector) IsFormula(element TextElementFlow) (bool, string) {
	// 1. 字体检测
	if efd.isMathFont(element.Font.Name) {
		return true, "math_font"
	}
	
	// 2. 符号密度检测
	if efd.hasHighSymbolDensity(element.Content) {
		return true, "symbol_density"
	}
	
	// 3. LaTeX命令检测
	if efd.hasLaTeXCommands(element.Content) {
		return true, "latex_command"
	}
	
	// 4. 数学表达式模式检测
	if efd.matchesMathPattern(element.Content) {
		return true, "math_pattern"
	}
	
	// 5. 布局特征检测（上下标、分数）
	// 这需要相邻元素的信息，暂时跳过
	
	return false, ""
}

// isMathFont 检查是否为数学字体
func (efd *EnhancedFormulaDetector) isMathFont(fontName string) bool {
	for _, pattern := range efd.mathFontPatterns {
		if pattern.MatchString(fontName) {
			return true
		}
	}
	return false
}

// hasHighSymbolDensity 检查是否有高数学符号密度
func (efd *EnhancedFormulaDetector) hasHighSymbolDensity(text string) bool {
	if len(text) == 0 {
		return false
	}
	
	runes := []rune(text)
	symbolCount := 0
	
	for _, r := range runes {
		if efd.mathSymbols[r] {
			symbolCount++
		}
	}
	
	density := float64(symbolCount) / float64(len(runes))
	return density >= efd.symbolDensityThreshold
}

// hasLaTeXCommands 检查是否包含LaTeX命令
func (efd *EnhancedFormulaDetector) hasLaTeXCommands(text string) bool {
	return efd.latexCommandPattern.MatchString(text)
}

// matchesMathPattern 检查是否匹配数学表达式模式
func (efd *EnhancedFormulaDetector) matchesMathPattern(text string) bool {
	patterns := []string{
		`\d+\s*[+\-*/=]\s*\d+`,                    // 算术表达式: 1 + 2
		`[a-zA-Z]\s*[+\-*/=]\s*[a-zA-Z]`,          // 代数表达式: x + y
		`[a-zA-Z]\s*[+\-*/=]\s*\d+`,               // 混合表达式: x = 5
		`\d+\^\d+`,                                // 指数: 2^3
		`\d+_\d+`,                                 // 下标: x_1
		`[a-zA-Z]_\{[^}]+\}`,                      // LaTeX下标: x_{12}
		`[a-zA-Z]\^\{[^}]+\}`,                     // LaTeX上标: x^{12}
		`\\frac\{[^}]+\}\{[^}]+\}`,                // LaTeX分数
		`\\sqrt(\[[^\]]+\])?\{[^}]+\}`,            // LaTeX根号
		`\([^)]*\)\s*[+\-*/=]`,                    // 括号表达式
		`[a-zA-Z]\([a-zA-Z0-9,\s]+\)`,             // 函数调用: f(x)
		`\d+\.\d+([eE][+-]?\d+)?`,                 // 科学计数法
		`[a-zA-Z]'`,                               // 导数符号: f'
		`\|[^|]+\|`,                               // 绝对值
		`\[[^\]]+\]`,                              // 矩阵/向量
	}
	
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, text); matched {
			return true
		}
	}
	
	return false
}

// ProtectFormulas 保护页面中的所有公式
func (fp *FormulaProtector) ProtectFormulas(page *PDFPageFlow) int {
	protectedCount := 0
	
	for i := range page.TextElements {
		element := &page.TextElements[i]
		
		// 检测是否为公式
		isFormula, reason := fp.detector.IsFormula(*element)
		if isFormula {
			// 创建占位符
			placeholder := fp.createPlaceholder(*element, reason)
			
			// 替换文本内容
			element.Content = placeholder.Placeholder
			element.IsFormula = true
			
			// 保存原始信息
			fp.placeholders[placeholder.Placeholder] = placeholder
			
			protectedCount++
		}
	}
	
	return protectedCount
}

// createPlaceholder 创建占位符
func (fp *FormulaProtector) createPlaceholder(element TextElementFlow, reason string) *FormulaPlaceholder {
	placeholder := &FormulaPlaceholder{
		ID:          fmt.Sprintf("formula_%d", fp.nextID),
		Placeholder: fmt.Sprintf("{v%d}", fp.nextID),
		Original:    element,
		BoundingBox: element.BoundingBox,
		Type:        fp.detectFormulaType(element),
	}
	
	fp.nextID++
	return placeholder
}

// detectFormulaType 检测公式类型
func (fp *FormulaProtector) detectFormulaType(element TextElementFlow) string {
	// 根据字体大小和位置判断
	if element.Font.Size > 14 {
		return "display" // 显示公式（独立行）
	}
	
	// 检查是否包含编号
	if strings.Contains(element.Content, "(") && strings.Contains(element.Content, ")") {
		return "equation" // 带编号的方程
	}
	
	return "inline" // 行内公式
}

// RestoreFormulas 恢复公式（在翻译后）
func (fp *FormulaProtector) RestoreFormulas(translatedText string) string {
	result := translatedText
	
	// 替换所有占位符为原始公式
	for placeholder, info := range fp.placeholders {
		result = strings.ReplaceAll(result, placeholder, info.Original.Content)
	}
	
	return result
}

// GetPlaceholder 获取占位符信息
func (fp *FormulaProtector) GetPlaceholder(placeholder string) (*FormulaPlaceholder, bool) {
	info, ok := fp.placeholders[placeholder]
	return info, ok
}

// GetAllPlaceholders 获取所有占位符
func (fp *FormulaProtector) GetAllPlaceholders() map[string]*FormulaPlaceholder {
	return fp.placeholders
}

// Clear 清空占位符
func (fp *FormulaProtector) Clear() {
	fp.placeholders = make(map[string]*FormulaPlaceholder)
	fp.nextID = 0
}

// ExtractFormulasFromText 从文本中提取公式占位符
func (fp *FormulaProtector) ExtractFormulasFromText(text string) []string {
	pattern := regexp.MustCompile(`\{v\d+\}`)
	return pattern.FindAllString(text, -1)
}

// IsPlaceholder 检查字符串是否为占位符
func (fp *FormulaProtector) IsPlaceholder(text string) bool {
	matched, _ := regexp.MatchString(`^\{v\d+\}$`, text)
	return matched
}

// GetStatistics 获取统计信息
func (fp *FormulaProtector) GetStatistics() map[string]interface{} {
	typeCount := make(map[string]int)
	
	for _, placeholder := range fp.placeholders {
		typeCount[placeholder.Type]++
	}
	
	return map[string]interface{}{
		"total":       len(fp.placeholders),
		"inline":      typeCount["inline"],
		"display":     typeCount["display"],
		"equation":    typeCount["equation"],
		"next_id":     fp.nextID,
	}
}

// ShouldProtectElement 判断元素是否应该被保护（不翻译）
func (fp *FormulaProtector) ShouldProtectElement(element TextElementFlow) bool {
	// 1. 已标记为公式
	if element.IsFormula {
		return true
	}
	
	// 2. 纯数字
	if isNumericOnly(element.Content) {
		return true
	}
	
	// 3. 纯符号
	if isSymbolOnly(element.Content) {
		return true
	}
	
	// 4. 太短（可能是标点或编号）
	if len(strings.TrimSpace(element.Content)) < 3 {
		return true
	}
	
	return false
}

// isNumericOnly 检查是否仅包含数字
func isNumericOnly(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return false
	}
	
	for _, r := range text {
		if !unicode.IsDigit(r) && r != '.' && r != ',' && r != '-' && r != '+' {
			return false
		}
	}
	return true
}

// isSymbolOnly 检查是否仅包含符号
func isSymbolOnly(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return false
	}
	
	letterCount := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letterCount++
		}
	}
	
	// 如果字母少于30%，认为是符号
	return float64(letterCount)/float64(len([]rune(text))) < 0.3
}
