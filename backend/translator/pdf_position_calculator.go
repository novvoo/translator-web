package translator

import (
	"fmt"
	"math"
)

// TextPositionCalculator 精确的文本位置计算器
// 实现pdf2zh的坐标级还原技术
type TextPositionCalculator struct {
	// 当前文本矩阵 (Tm操作符设置)
	textMatrix TransformMatrix
	
	// 当前行矩阵 (Td, TD, T*操作符修改)
	lineMatrix TransformMatrix
	
	// 当前变换矩阵 (cm操作符修改)
	ctm TransformMatrix
	
	// 文本状态
	textState TextStateFlow
	
	// 当前字体
	currentFont FontFlow
	
	// 是否在文本对象内 (BT...ET)
	inTextObject bool
}

// NewTextPositionCalculator 创建新的位置计算器
func NewTextPositionCalculator() *TextPositionCalculator {
	return &TextPositionCalculator{
		textMatrix: IdentityMatrix(),
		lineMatrix: IdentityMatrix(),
		ctm:        IdentityMatrix(),
		textState: TextStateFlow{
			Scale: 1.0,
		},
		currentFont: FontFlow{
			Size: 12,
		},
		inTextObject: false,
	}
}

// IdentityMatrix 返回单位矩阵
func IdentityMatrix() TransformMatrix {
	return TransformMatrix{
		A: 1, B: 0,
		C: 0, D: 1,
		E: 0, F: 0,
	}
}

// ProcessOperator 处理PDF操作符并更新状态
func (calc *TextPositionCalculator) ProcessOperator(op PDFOperation) {
	switch op.Operator {
	case "BT":
		// 开始文本对象
		calc.inTextObject = true
		calc.textMatrix = IdentityMatrix()
		calc.lineMatrix = IdentityMatrix()
		
	case "ET":
		// 结束文本对象
		calc.inTextObject = false
		
	case "Tm":
		// 设置文本矩阵 - 最重要的定位操作符
		if len(op.Operands) >= 6 {
			calc.textMatrix = ParseTransformMatrix(op.Operands)
			calc.lineMatrix = calc.textMatrix // 重置行矩阵
		}
		
	case "Td":
		// 移动到下一行的起点
		if len(op.Operands) >= 2 {
			tx := ParseFloat(op.Operands[0])
			ty := ParseFloat(op.Operands[1])
			
			// Td等价于: Tm = Tlm * [1 0 0 1 tx ty]
			translation := TransformMatrix{
				A: 1, B: 0,
				C: 0, D: 1,
				E: tx, F: ty,
			}
			calc.lineMatrix = MultiplyMatrices(calc.lineMatrix, translation)
			calc.textMatrix = calc.lineMatrix
		}
		
	case "TD":
		// 移动到下一行并设置行距
		if len(op.Operands) >= 2 {
			tx := ParseFloat(op.Operands[0])
			ty := ParseFloat(op.Operands[1])
			
			// TD等价于: -ty TL, tx ty Td
			calc.textState.Leading = -ty
			
			translation := TransformMatrix{
				A: 1, B: 0,
				C: 0, D: 1,
				E: tx, F: ty,
			}
			calc.lineMatrix = MultiplyMatrices(calc.lineMatrix, translation)
			calc.textMatrix = calc.lineMatrix
		}
		
	case "T*":
		// 移动到下一行（使用当前行距）
		// T*等价于: 0 -Tl Td
		translation := TransformMatrix{
			A: 1, B: 0,
			C: 0, D: 1,
			E: 0, F: -calc.textState.Leading,
		}
		calc.lineMatrix = MultiplyMatrices(calc.lineMatrix, translation)
		calc.textMatrix = calc.lineMatrix
		
	case "Tf":
		// 设置字体和大小
		if len(op.Operands) >= 2 {
			calc.currentFont.Name = op.Operands[0]
			calc.currentFont.Size = ParseFloat(op.Operands[1])
		}
		
	case "Tc":
		// 字符间距
		if len(op.Operands) >= 1 {
			calc.textState.CharSpace = ParseFloat(op.Operands[0])
		}
		
	case "Tw":
		// 词间距
		if len(op.Operands) >= 1 {
			calc.textState.WordSpace = ParseFloat(op.Operands[0])
		}
		
	case "Tz":
		// 水平缩放
		if len(op.Operands) >= 1 {
			calc.textState.Scale = ParseFloat(op.Operands[0]) / 100.0
		}
		
	case "TL":
		// 行距
		if len(op.Operands) >= 1 {
			calc.textState.Leading = ParseFloat(op.Operands[0])
		}
		
	case "Ts":
		// 文本上升
		if len(op.Operands) >= 1 {
			calc.textState.Rise = ParseFloat(op.Operands[0])
		}
		
	case "cm":
		// 修改当前变换矩阵
		if len(op.Operands) >= 6 {
			newCTM := ParseTransformMatrix(op.Operands)
			calc.ctm = MultiplyMatrices(calc.ctm, newCTM)
		}
		
	case "q":
		// 保存图形状态 - 在调用者处理
		
	case "Q":
		// 恢复图形状态 - 在调用者处理
	}
}

// CalculateTextPosition 计算文本的精确位置
// 返回: (x, y, width, height)
func (calc *TextPositionCalculator) CalculateTextPosition(text string) (float64, float64, float64, float64) {
	if !calc.inTextObject {
		return 0, 0, 0, 0
	}
	
	// 组合所有变换矩阵: CTM * TextMatrix
	combined := MultiplyMatrices(calc.ctm, calc.textMatrix)
	
	// 文本起点位置
	x := combined.E
	y := combined.F
	
	// 计算文本宽度（考虑字符间距、词间距、水平缩放）
	width := calc.CalculateTextWidth(text)
	
	// 文本高度（字体大小 + 上升值）
	height := calc.currentFont.Size + calc.textState.Rise
	
	return x, y, width, height
}

// CalculateTextWidth 计算文本宽度
func (calc *TextPositionCalculator) CalculateTextWidth(text string) float64 {
	if text == "" {
		return 0
	}
	
	// 基础宽度估算（每个字符）
	baseWidth := 0.0
	spaceCount := 0
	
	for _, r := range text {
		if r == ' ' {
			spaceCount++
			baseWidth += calc.currentFont.Size * 0.25 // 空格宽度
		} else if r >= 0x4e00 && r <= 0x9fff {
			// 中文字符（等宽）
			baseWidth += calc.currentFont.Size
		} else {
			// 英文字符（平均宽度）
			baseWidth += calc.currentFont.Size * 0.55
		}
	}
	
	// 添加字符间距
	charCount := float64(len([]rune(text)))
	baseWidth += calc.textState.CharSpace * charCount
	
	// 添加词间距
	baseWidth += calc.textState.WordSpace * float64(spaceCount)
	
	// 应用水平缩放
	baseWidth *= calc.textState.Scale
	
	// 应用文本矩阵的缩放
	baseWidth *= math.Abs(calc.textMatrix.A)
	
	return baseWidth
}

// UpdateTextPosition 更新文本位置（在显示文本后）
func (calc *TextPositionCalculator) UpdateTextPosition(text string) {
	if !calc.inTextObject {
		return
	}
	
	// 计算文本宽度
	width := calc.CalculateTextWidth(text)
	
	// 更新文本矩阵的X位置
	calc.textMatrix.E += width
}

// GetCurrentState 获取当前状态（用于调试）
func (calc *TextPositionCalculator) GetCurrentState() map[string]interface{} {
	return map[string]interface{}{
		"inTextObject": calc.inTextObject,
		"textMatrix":   calc.textMatrix,
		"lineMatrix":   calc.lineMatrix,
		"ctm":          calc.ctm,
		"fontSize":     calc.currentFont.Size,
		"fontName":     calc.currentFont.Name,
		"charSpace":    calc.textState.CharSpace,
		"wordSpace":    calc.textState.WordSpace,
		"scale":        calc.textState.Scale,
		"leading":      calc.textState.Leading,
		"rise":         calc.textState.Rise,
	}
}

// Clone 克隆计算器状态（用于状态栈）
func (calc *TextPositionCalculator) Clone() *TextPositionCalculator {
	return &TextPositionCalculator{
		textMatrix:   calc.textMatrix,
		lineMatrix:   calc.lineMatrix,
		ctm:          calc.ctm,
		textState:    calc.textState,
		currentFont:  calc.currentFont,
		inTextObject: calc.inTextObject,
	}
}

// Restore 恢复计算器状态
func (calc *TextPositionCalculator) Restore(saved *TextPositionCalculator) {
	calc.textMatrix = saved.textMatrix
	calc.lineMatrix = saved.lineMatrix
	calc.ctm = saved.ctm
	calc.textState = saved.textState
	calc.currentFont = saved.currentFont
	calc.inTextObject = saved.inTextObject
}

// MultiplyMatrices 矩阵乘法
func MultiplyMatrices(m1, m2 TransformMatrix) TransformMatrix {
	return TransformMatrix{
		A: m1.A*m2.A + m1.B*m2.C,
		B: m1.A*m2.B + m1.B*m2.D,
		C: m1.C*m2.A + m1.D*m2.C,
		D: m1.C*m2.B + m1.D*m2.D,
		E: m1.E*m2.A + m1.F*m2.C + m2.E,
		F: m1.E*m2.B + m1.F*m2.D + m2.F,
	}
}

// ParseTransformMatrix 从操作数解析变换矩阵
func ParseTransformMatrix(operands []string) TransformMatrix {
	if len(operands) < 6 {
		return IdentityMatrix()
	}
	
	return TransformMatrix{
		A: ParseFloat(operands[0]),
		B: ParseFloat(operands[1]),
		C: ParseFloat(operands[2]),
		D: ParseFloat(operands[3]),
		E: ParseFloat(operands[4]),
		F: ParseFloat(operands[5]),
	}
}

// ParseFloat 解析浮点数
func ParseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// TransformPoint 变换一个点
func TransformPoint(m TransformMatrix, x, y float64) (float64, float64) {
	newX := m.A*x + m.C*y + m.E
	newY := m.B*x + m.D*y + m.F
	return newX, newY
}

// InverseMatrix 计算逆矩阵
func InverseMatrix(m TransformMatrix) (TransformMatrix, error) {
	det := m.A*m.D - m.B*m.C
	if math.Abs(det) < 1e-10 {
		return IdentityMatrix(), fmt.Errorf("矩阵不可逆")
	}
	
	return TransformMatrix{
		A: m.D / det,
		B: -m.B / det,
		C: -m.C / det,
		D: m.A / det,
		E: (m.C*m.F - m.D*m.E) / det,
		F: (m.B*m.E - m.A*m.F) / det,
	}, nil
}
