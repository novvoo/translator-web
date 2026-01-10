package translator

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PDFImageExtractor PDF图片提取器
type PDFImageExtractor struct {
	ctx       *model.Context
	outputDir string
	logger    *PDFLogger
}

// NewPDFImageExtractor 创建图片提取器
func NewPDFImageExtractor(pdfPath, outputDir string, logger *PDFLogger) (*PDFImageExtractor, error) {
	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("读取PDF上下文失败: %w", err)
	}

	return &PDFImageExtractor{
		ctx:       ctx,
		outputDir: outputDir,
		logger:    logger,
	}, nil
}

// ExtractAllImages 提取所有图片
func (e *PDFImageExtractor) ExtractAllImages() (map[string]string, error) {
	imageMapping := make(map[string]string)
	imageCount := 0

	e.logger.Info("开始提取图片", map[string]interface{}{
		"总页数": e.ctx.PageCount,
	})

	// 遍历每一页
	for pageNum := 1; pageNum <= e.ctx.PageCount; pageNum++ {
		pageDict, _, _, err := e.ctx.PageDict(pageNum, false)
		if err != nil {
			e.logger.Warn("获取页面字典失败", map[string]interface{}{
				"页码": pageNum,
				"错误": err.Error(),
			})
			continue
		}

		// 提取页面中的图片
		images, err := e.extractImagesFromPage(pageDict, pageNum)
		if err != nil {
			e.logger.Warn("提取页面图片失败", map[string]interface{}{
				"页码": pageNum,
				"错误": err.Error(),
			})
			continue
		}

		// 保存图片并建立映射
		for name, img := range images {
			filename := fmt.Sprintf("page%d_%s.png", pageNum, name)
			filepath := filepath.Join(e.outputDir, filename)

			if err := e.saveImage(img, filepath); err != nil {
				e.logger.Warn("保存图片失败", map[string]interface{}{
					"图片名称": name,
					"文件路径": filepath,
					"错误":   err.Error(),
				})
				continue
			}

			imageMapping[name] = filepath
			imageCount++

			e.logger.Debug("图片提取成功", map[string]interface{}{
				"页码":   pageNum,
				"图片名称": name,
				"文件路径": filepath,
			})
		}
	}

	e.logger.Info("图片提取完成", map[string]interface{}{
		"提取数量": imageCount,
	})

	return imageMapping, nil
}

// extractImagesFromPage 从页面中提取图片
func (e *PDFImageExtractor) extractImagesFromPage(pageDict types.Dict, pageNum int) (map[string]image.Image, error) {
	images := make(map[string]image.Image)

	// 获取Resources字典
	resourcesObj, found := pageDict.Find("Resources")
	if !found {
		return images, nil
	}

	resourcesDict, ok := resourcesObj.(types.Dict)
	if !ok {
		return images, nil
	}

	// 获取XObject字典
	xobjectObj, found := resourcesDict.Find("XObject")
	if !found {
		return images, nil
	}

	xobjectDict, ok := xobjectObj.(types.Dict)
	if !ok {
		return images, nil
	}

	// 遍历XObject
	for key, value := range xobjectDict {
		// 解引用
		indRef, ok := value.(types.IndirectRef)
		if !ok {
			continue
		}

		streamDict, _, err := e.ctx.DereferenceStreamDict(indRef)
		if err != nil {
			log.Printf("解引用流失败: %v", err)
			continue
		}

		// 检查是否为图片
		if !e.isImageXObject(streamDict) {
			continue
		}

		// 提取图片
		img, err := e.extractImage(streamDict)
		if err != nil {
			log.Printf("提取图片失败 %s: %v", key, err)
			continue
		}

		if img != nil {
			images[key] = img
		}
	}

	return images, nil
}

// isImageXObject 检查XObject是否为图片
func (e *PDFImageExtractor) isImageXObject(streamDict *types.StreamDict) bool {
	if streamDict == nil || streamDict.Dict == nil {
		return false
	}

	subtypeObj, found := streamDict.Dict.Find("Subtype")
	if !found {
		return false
	}

	subtype, ok := subtypeObj.(types.Name)
	if !ok {
		return false
	}

	return subtype == "Image"
}

// extractImage 提取图片数据
func (e *PDFImageExtractor) extractImage(streamDict *types.StreamDict) (image.Image, error) {
	// 解码流
	if err := streamDict.Decode(); err != nil {
		return nil, fmt.Errorf("解码流失败: %w", err)
	}

	if streamDict.Content == nil {
		return nil, fmt.Errorf("流内容为空")
	}

	// 获取图片属性
	width, _ := e.getIntValue(streamDict.Dict, "Width")
	height, _ := e.getIntValue(streamDict.Dict, "Height")
	bitsPerComponent, _ := e.getIntValue(streamDict.Dict, "BitsPerComponent")
	
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("无效的图片尺寸: %dx%d", width, height)
	}

	// 获取颜色空间
	colorSpace := e.getColorSpace(streamDict.Dict)

	// 根据颜色空间创建图片
	img := e.createImage(streamDict.Content, width, height, bitsPerComponent, colorSpace)
	
	return img, nil
}

// getIntValue 获取整数值
func (e *PDFImageExtractor) getIntValue(dict types.Dict, key string) (int, bool) {
	obj, found := dict.Find(key)
	if !found {
		return 0, false
	}

	switch v := obj.(type) {
	case types.Integer:
		return int(v), true
	case types.Float:
		return int(v), true
	default:
		return 0, false
	}
}

// getColorSpace 获取颜色空间
func (e *PDFImageExtractor) getColorSpace(dict types.Dict) string {
	obj, found := dict.Find("ColorSpace")
	if !found {
		return "DeviceRGB"
	}

	switch v := obj.(type) {
	case types.Name:
		return string(v)
	default:
		return "DeviceRGB"
	}
}

// createImage 创建图片
func (e *PDFImageExtractor) createImage(data []byte, width, height, bitsPerComponent int, colorSpace string) image.Image {
	// 创建RGBA图片
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 简化实现：假设是RGB格式
	bytesPerPixel := 3
	if colorSpace == "DeviceGray" {
		bytesPerPixel = 1
	} else if colorSpace == "DeviceCMYK" {
		bytesPerPixel = 4
	}

	// 填充像素数据
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * bytesPerPixel
			if offset+bytesPerPixel > len(data) {
				break
			}

			var r, g, b uint8
			if colorSpace == "DeviceGray" {
				r = data[offset]
				g = data[offset]
				b = data[offset]
			} else if colorSpace == "DeviceRGB" {
				r = data[offset]
				g = data[offset+1]
				b = data[offset+2]
			} else if colorSpace == "DeviceCMYK" {
				// 简化的CMYK到RGB转换
				c := float64(data[offset]) / 255.0
				m := float64(data[offset+1]) / 255.0
				y := float64(data[offset+2]) / 255.0
				k := float64(data[offset+3]) / 255.0

				r = uint8((1 - c) * (1 - k) * 255)
				g = uint8((1 - m) * (1 - k) * 255)
				b = uint8((1 - y) * (1 - k) * 255)
			}

			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}

// saveImage 保存图片
func (e *PDFImageExtractor) saveImage(img image.Image, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// 根据文件扩展名选择编码格式
	ext := filepath[len(filepath)-4:]
	if ext == ".jpg" || ext == "jpeg" {
		return jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
	}

	return png.Encode(file, img)
}
