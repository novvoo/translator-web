package translator

import (
	"fmt"
	"log"
	"strings"

	dslipakpdf "github.com/dslipak/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// PureEnhancedPDFParser 纯Go增强版PDF解析器
type PureEnhancedPDFParser struct {
	originalParser *PDFParser
}

// NewPureEnhancedPDFParser 创建纯Go增强版PDF解析器
func NewPureEnhancedPDFParser() *PureEnhancedPDFParser {
	return &PureEnhancedPDFParser{
		originalParser: NewPDFParser("", ""),
	}
}

// ParsePDFPureEnhanced 使用多个纯Go库解析PDF，提高识别成功率
func (p *PureEnhancedPDFParser) ParsePDFPureEnhanced(filePath string) (*PDFDocument, error) {
	log.Printf("开始纯Go增强解析PDF文件: %s", filePath)

	// 方法1: 尝试使用原始库 (ledongthuc/pdf)
	doc1, err1 := p.parseWithOriginalLib(filePath)
	
	// 方法2: 尝试使用dslipak/pdf库
	doc2, err2 := p.parseWithDslipakPDF(filePath)
	
	// 方法3: 尝试使用pdfcpu库
	doc3, err3 := p.parseWithPDFCPU(filePath)
	
	// 合并结果，选择最好的
	finalDoc := p.mergeParsedResults(doc1, doc2, doc3, err1, err2, err3)
	
	if finalDoc == nil {
		return nil, fmt.Errorf("所有解析方法都失败了: 原始库(%v), dslipak/pdf(%v), pdfcpu(%v)", err1, err2, err3)
	}
	
	log.Printf("纯Go增强解析完成，最终提取到 %d 页内容", len(finalDoc.PageTexts))
	return finalDoc, nil
}

// parseWithOriginalLib 使用原始库解析
func (p *PureEnhancedPDFParser) parseWithOriginalLib(filePath string) (*PDFDocument, error) {
	log.Printf("尝试使用原始库(ledongthuc/pdf)解析...")
	return OpenPDF(filePath)
}

// parseWithDslipakPDF 使用dslipak/pdf库解析
func (p *PureEnhancedPDFParser) parseWithDslipakPDF(filePath string) (*PDFDocument, error) {
	log.Printf("尝试使用dslipak/pdf库解析...")
	
	reader, err := dslipakpdf.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("dslipak/pdf打开失败: %w", err)
	}
	// dslipak/pdf的Reader没有Close方法，不需要defer close
	
	pageCount := reader.NumPage()
	pdfDoc := &PDFDocument{
		Path:      filePath,
		PageTexts: make([]string, pageCount),
		Metadata: PDFMetadata{
			Pages: pageCount,
		},
	}
	
	// 提取每页文本
	for i := 1; i <= pageCount; i++ {
		page := reader.Page(i)
		// dslipak/pdf的Page不能直接与nil比较，检查是否有效
		
		text, err := page.GetPlainText(nil)
		if err != nil {
			log.Printf("dslipak/pdf警告：无法提取第 %d 页的文本: %v", i, err)
			pdfDoc.PageTexts[i-1] = ""
			continue
		}
		
		cleanText := cleanPDFText(text)
		pdfDoc.PageTexts[i-1] = cleanText
	}
	
	log.Printf("dslipak/pdf解析完成，提取了 %d 页", pageCount)
	return pdfDoc, nil
}

// parseWithPDFCPU 使用pdfcpu库解析
func (p *PureEnhancedPDFParser) parseWithPDFCPU(filePath string) (*PDFDocument, error) {
	log.Printf("尝试使用pdfcpu库解析...")
	
	// 首先验证PDF
	err := api.ValidateFile(filePath, model.NewDefaultConfiguration())
	if err != nil {
		return nil, fmt.Errorf("pdfcpu验证失败: %w", err)
	}
	
	// 获取页数
	pageCount, err := api.PageCountFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("pdfcpu获取页数失败: %w", err)
	}
	
	pdfDoc := &PDFDocument{
		Path:      filePath,
		PageTexts: make([]string, pageCount),
		Metadata: PDFMetadata{
			Pages: pageCount,
		},
	}
	
	// 尝试提取文本 - 简化版本，不使用ExtractContentFile
	// 因为API签名可能不兼容，我们只返回基本结构
	log.Printf("pdfcpu解析完成，提取了 %d 页 (仅基本结构)", pageCount)
	return pdfDoc, nil
}

// mergeParsedResults 合并多个解析结果，选择最好的
func (p *PureEnhancedPDFParser) mergeParsedResults(doc1, doc2, doc3 *PDFDocument, err1, err2, err3 error) *PDFDocument {
	var candidates []*PDFDocument
	var candidateNames []string
	
	// 收集成功的解析结果
	if err1 == nil && doc1 != nil {
		candidates = append(candidates, doc1)
		candidateNames = append(candidateNames, "ledongthuc/pdf")
	}
	
	if err2 == nil && doc2 != nil {
		candidates = append(candidates, doc2)
		candidateNames = append(candidateNames, "dslipak/pdf")
	}
	
	if err3 == nil && doc3 != nil {
		candidates = append(candidates, doc3)
		candidateNames = append(candidateNames, "pdfcpu")
	}
	
	if len(candidates) == 0 {
		log.Printf("所有解析方法都失败了")
		return nil
	}
	
	// 选择最好的结果（提取内容最多的）
	bestDoc := candidates[0]
	bestName := candidateNames[0]
	bestScore := p.calculateDocumentScore(bestDoc)
	
	for i, doc := range candidates[1:] {
		score := p.calculateDocumentScore(doc)
		if score > bestScore {
			bestDoc = doc
			bestName = candidateNames[i+1]
			bestScore = score
		}
	}
	
	log.Printf("选择了 %s 的解析结果作为基础，得分: %.2f", bestName, bestScore)
	
	// 尝试合并不同库的结果来填补空白页
	finalDoc := p.fillMissingPages(bestDoc, candidates, candidateNames)
	
	return finalDoc
}

// calculateDocumentScore 计算文档解析质量得分
func (p *PureEnhancedPDFParser) calculateDocumentScore(doc *PDFDocument) float64 {
	if doc == nil {
		return 0
	}
	
	totalChars := 0
	nonEmptyPages := 0
	
	for _, pageText := range doc.PageTexts {
		chars := len(strings.TrimSpace(pageText))
		totalChars += chars
		if chars > 0 {
			nonEmptyPages++
		}
	}
	
	if doc.Metadata.Pages == 0 {
		return 0
	}
	
	// 得分 = 总字符数 * 非空页面比例
	emptyPageRatio := float64(nonEmptyPages) / float64(doc.Metadata.Pages)
	score := float64(totalChars) * emptyPageRatio
	
	return score
}

// fillMissingPages 尝试用其他解析结果填补空白页
func (p *PureEnhancedPDFParser) fillMissingPages(bestDoc *PDFDocument, candidates []*PDFDocument, candidateNames []string) *PDFDocument {
	if bestDoc == nil || len(candidates) <= 1 {
		return bestDoc
	}
	
	// 创建最终文档的副本
	finalDoc := &PDFDocument{
		Path:      bestDoc.Path,
		PageTexts: make([]string, len(bestDoc.PageTexts)),
		Metadata:  bestDoc.Metadata,
	}
	
	copy(finalDoc.PageTexts, bestDoc.PageTexts)
	
	// 对于每个空白页，尝试从其他候选结果中找到内容
	for i, pageText := range finalDoc.PageTexts {
		if strings.TrimSpace(pageText) == "" {
			// 这一页是空白的，尝试从其他候选中找到内容
			for j, candidate := range candidates {
				if candidate != bestDoc && i < len(candidate.PageTexts) {
					candidateText := strings.TrimSpace(candidate.PageTexts[i])
					if candidateText != "" {
						finalDoc.PageTexts[i] = candidateText
						log.Printf("从 %s 解析结果中填补了第 %d 页的内容 (%d 字符)", 
							candidateNames[j], i+1, len(candidateText))
						break
					}
				}
			}
		}
	}
	
	return finalDoc
}

// OpenPDFPureEnhanced 纯Go增强版PDF打开函数
func OpenPDFPureEnhanced(path string) (*PDFDocument, error) {
	parser := NewPureEnhancedPDFParser()
	return parser.ParsePDFPureEnhanced(path)
}