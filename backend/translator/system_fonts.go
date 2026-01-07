package translator

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SystemFontDetector 系统字体检测器
type SystemFontDetector struct{}

// NewSystemFontDetector 创建系统字体检测器
func NewSystemFontDetector() *SystemFontDetector {
	return &SystemFontDetector{}
}

// GetSystemFontPath 根据语言获取系统字体路径
func (sfd *SystemFontDetector) GetSystemFontPath(language string) string {
	switch runtime.GOOS {
	case "windows":
		return sfd.getWindowsFont(language)
	case "darwin":
		return sfd.getMacFont(language)
	case "linux":
		return sfd.getLinuxFont(language)
	default:
		log.Printf("不支持的操作系统: %s", runtime.GOOS)
		return ""
	}
}

// getWindowsFont 获取 Windows 系统字体
func (sfd *SystemFontDetector) getWindowsFont(language string) string {
	windowsFontsDir := filepath.Join(os.Getenv("WINDIR"), "Fonts")

	// 根据语言选择字体
	var fontCandidates []string
	switch strings.ToLower(language) {
	case "zh", "chinese", "zh-cn", "zh-tw", "zh-hk":
		fontCandidates = []string{
			"msyh.ttc",     // 微软雅黑
			"msyhbd.ttc",   // 微软雅黑 Bold
			"simsun.ttc",   // 宋体
			"simhei.ttf",   // 黑体
			"simkai.ttf",   // 楷体
			"STZHONGS.TTF", // 华文中宋
			"STFANGSO.TTF", // 华文仿宋
			"STKAITI.TTF",  // 华文楷体
			"STSONG.TTF",   // 华文宋体
			"STXIHEI.TTF",  // 华文细黑
		}
	case "ja", "japanese":
		fontCandidates = []string{
			"msgothic.ttc", // MS Gothic
			"msmincho.ttc", // MS Mincho
			"YuGothM.ttc",  // Yu Gothic Medium
			"YuGothR.ttc",  // Yu Gothic Regular
			"YuGothL.ttc",  // Yu Gothic Light
			"YuMincho.ttc", // Yu Mincho
			"meiryo.ttc",   // Meiryo
			"meiryob.ttc",  // Meiryo Bold
		}
	case "ko", "korean":
		fontCandidates = []string{
			"malgun.ttf",   // Malgun Gothic
			"malgunbd.ttf", // Malgun Gothic Bold
			"gulim.ttc",    // Gulim
			"batang.ttc",   // Batang
			"dotum.ttc",    // Dotum
			"gungsuh.ttc",  // Gungsuh
		}
	case "ar", "arabic":
		fontCandidates = []string{
			"tahoma.ttf",   // Tahoma (支持阿拉伯语)
			"tahomabd.ttf", // Tahoma Bold
			"arial.ttf",    // Arial (部分支持)
			"arialbd.ttf",  // Arial Bold
			"calibri.ttf",  // Calibri
			"calibrib.ttf", // Calibri Bold
		}
	case "hi", "hindi", "devanagari":
		fontCandidates = []string{
			"mangal.ttf", // Mangal
			"utsaah.ttf", // Utsaah
			"aparaj.ttf", // Aparajita
			"kokila.ttf", // Kokila
			"khand.ttf",  // Khand
		}
	case "th", "thai":
		fontCandidates = []string{
			"tahoma.ttf",   // Tahoma (支持泰语)
			"tahomabd.ttf", // Tahoma Bold
			"cordia.ttf",   // Cordia New
			"cordiau.ttf",  // Cordia UPC
			"angsana.ttc",  // Angsana New
		}
	case "ru", "russian", "cyrillic":
		fontCandidates = []string{
			"arial.ttf",    // Arial
			"arialbd.ttf",  // Arial Bold
			"calibri.ttf",  // Calibri
			"calibrib.ttf", // Calibri Bold
			"tahoma.ttf",   // Tahoma
			"tahomabd.ttf", // Tahoma Bold
			"times.ttf",    // Times New Roman
			"timesbd.ttf",  // Times New Roman Bold
		}
	case "he", "hebrew":
		fontCandidates = []string{
			"arial.ttf",    // Arial
			"arialbd.ttf",  // Arial Bold
			"tahoma.ttf",   // Tahoma
			"tahomabd.ttf", // Tahoma Bold
			"calibri.ttf",  // Calibri
		}
	case "vi", "vietnamese":
		fontCandidates = []string{
			"arial.ttf",    // Arial
			"arialbd.ttf",  // Arial Bold
			"tahoma.ttf",   // Tahoma
			"tahomabd.ttf", // Tahoma Bold
			"calibri.ttf",  // Calibri
			"times.ttf",    // Times New Roman
		}
	case "tr", "turkish":
		fontCandidates = []string{
			"arial.ttf",    // Arial
			"arialbd.ttf",  // Arial Bold
			"calibri.ttf",  // Calibri
			"calibrib.ttf", // Calibri Bold
			"tahoma.ttf",   // Tahoma
			"times.ttf",    // Times New Roman
		}
	case "pt", "portuguese", "es", "spanish", "fr", "french", "de", "german", "it", "italian":
		fontCandidates = []string{
			"arial.ttf",    // Arial
			"arialbd.ttf",  // Arial Bold
			"calibri.ttf",  // Calibri
			"calibrib.ttf", // Calibri Bold
			"times.ttf",    // Times New Roman
			"timesbd.ttf",  // Times New Roman Bold
			"tahoma.ttf",   // Tahoma
			"verdana.ttf",  // Verdana
		}
	default:
		// 默认使用支持多语言的字体
		fontCandidates = []string{
			"arial.ttf",    // Arial
			"arialbd.ttf",  // Arial Bold
			"calibri.ttf",  // Calibri
			"calibrib.ttf", // Calibri Bold
			"tahoma.ttf",   // Tahoma
			"times.ttf",    // Times New Roman
			"verdana.ttf",  // Verdana
		}
	}

	return sfd.findFirstExistingFont(windowsFontsDir, fontCandidates)
}

// getMacFont 获取 macOS 系统字体
func (sfd *SystemFontDetector) getMacFont(language string) string {
	macFontsDir := "/System/Library/Fonts"

	var fontCandidates []string
	switch strings.ToLower(language) {
	case "zh", "chinese", "zh-cn", "zh-tw", "zh-hk":
		fontCandidates = []string{
			"PingFang.ttc",       // 苹方
			"STHeiti Medium.ttc", // 华文黑体
			"STHeiti Light.ttc",  // 华文黑体 Light
			"Songti.ttc",         // 宋体
			"STSong.ttc",         // 华文宋体
			"STKaiti.ttc",        // 华文楷体
			"STFangsong.ttc",     // 华文仿宋
		}
	case "ja", "japanese":
		fontCandidates = []string{
			"ヒラギノ角ゴシック W3.ttc", // Hiragino Sans
			"ヒラギノ角ゴシック W6.ttc", // Hiragino Sans W6
			"ヒラギノ明朝 ProN.ttc",  // Hiragino Mincho ProN
			"Hiragino Sans GB.ttc",
			"YuGothic.ttc", // Yu Gothic
			"YuMincho.ttc", // Yu Mincho
		}
	case "ko", "korean":
		fontCandidates = []string{
			"AppleSDGothicNeo.ttc", // Apple SD Gothic Neo
			"AppleMyungjo.ttc",     // Apple Myungjo
		}
	case "ar", "arabic":
		fontCandidates = []string{
			"GeezaPro.ttc",      // Geeza Pro
			"Baghdad.ttc",       // Baghdad
			"DecoTypeNaskh.ttc", // DecoType Naskh
			"Helvetica.ttc",     // Helvetica (部分支持)
		}
	case "hi", "hindi", "devanagari":
		fontCandidates = []string{
			"DevanagariSangamMN.ttc", // Devanagari Sangam MN
			"KohinoorDevanagari.ttc", // Kohinoor Devanagari
			"ITFDevanagari.ttc",      // ITF Devanagari
		}
	case "th", "thai":
		fontCandidates = []string{
			"Thonburi.ttc",  // Thonburi
			"Sathu.ttc",     // Sathu
			"Krungthep.ttc", // Krungthep
		}
	case "ru", "russian", "cyrillic":
		fontCandidates = []string{
			"Helvetica.ttc", // Helvetica
			"Times.ttc",     // Times
			"Palatino.ttc",  // Palatino
			"Geneva.ttf",    // Geneva
		}
	case "he", "hebrew":
		fontCandidates = []string{
			"ArialHB.ttc",   // Arial Hebrew
			"Helvetica.ttc", // Helvetica
			"Times.ttc",     // Times
		}
	case "vi", "vietnamese", "tr", "turkish", "pt", "portuguese", "es", "spanish", "fr", "french", "de", "german", "it", "italian":
		fontCandidates = []string{
			"Helvetica.ttc",     // Helvetica
			"Times.ttc",         // Times
			"Palatino.ttc",      // Palatino
			"Geneva.ttf",        // Geneva
			"Lucida Grande.ttc", // Lucida Grande
		}
	default:
		fontCandidates = []string{
			"Helvetica.ttc", // Helvetica
			"Times.ttc",     // Times
			"Arial.ttf",     // Arial
			"Palatino.ttc",  // Palatino
		}
	}

	return sfd.findFirstExistingFont(macFontsDir, fontCandidates)
}

// getLinuxFont 获取 Linux 系统字体
func (sfd *SystemFontDetector) getLinuxFont(language string) string {
	linuxFontsDirs := []string{
		"/usr/share/fonts",
		"/usr/local/share/fonts",
		filepath.Join(os.Getenv("HOME"), ".fonts"),
	}

	var fontCandidates []string
	switch strings.ToLower(language) {
	case "zh", "chinese", "zh-cn", "zh-tw", "zh-hk":
		fontCandidates = []string{
			"truetype/wqy/wqy-microhei.ttc",         // 文泉驿微米黑
			"truetype/wqy/wqy-zenhei.ttc",           // 文泉驿正黑
			"truetype/droid/DroidSansFallback.ttf",  // Droid Sans Fallback
			"opentype/noto/NotoSansCJK-Regular.ttc", // Noto Sans CJK
			"truetype/arphic/ukai.ttc",              // AR PL UKai CN
			"truetype/arphic/uming.ttc",             // AR PL UMing CN
		}
	case "ja", "japanese":
		fontCandidates = []string{
			"opentype/noto/NotoSansCJK-Regular.ttc",
			"truetype/takao-gothic/TakaoPGothic.ttf",
			"truetype/takao-mincho/TakaoPMincho.ttf",
			"truetype/vlgothic/VL-Gothic-Regular.ttf",
			"truetype/ipa/ipag.ttf", // IPA Gothic
			"truetype/ipa/ipam.ttf", // IPA Mincho
		}
	case "ko", "korean":
		fontCandidates = []string{
			"opentype/noto/NotoSansCJK-Regular.ttc",
			"truetype/nanum/NanumGothic.ttf",
			"truetype/nanum/NanumMyeongjo.ttf",
			"truetype/baekmuk/batang.ttf",
			"truetype/baekmuk/gulim.ttf",
		}
	case "ar", "arabic":
		fontCandidates = []string{
			"truetype/kacst/KacstBook.ttf",
			"truetype/kacst/KacstOffice.ttf",
			"opentype/noto/NotoSansArabic-Regular.ttf",
			"truetype/dejavu/DejaVuSans.ttf", // 部分支持
		}
	case "hi", "hindi", "devanagari":
		fontCandidates = []string{
			"opentype/noto/NotoSansDevanagari-Regular.ttf",
			"truetype/lohit-devanagari/Lohit-Devanagari.ttf",
			"truetype/gargi/Gargi.ttf",
			"truetype/sarai/sarai.ttf",
		}
	case "th", "thai":
		fontCandidates = []string{
			"opentype/noto/NotoSansThai-Regular.ttf",
			"truetype/tlwg/Garuda.ttf",
			"truetype/tlwg/Kinnari.ttf",
			"truetype/tlwg/Loma.ttf",
		}
	case "ru", "russian", "cyrillic":
		fontCandidates = []string{
			"truetype/dejavu/DejaVuSans.ttf",
			"truetype/liberation/LiberationSans-Regular.ttf",
			"opentype/noto/NotoSans-Regular.ttf",
			"truetype/droid/DroidSans.ttf",
		}
	case "he", "hebrew":
		fontCandidates = []string{
			"opentype/noto/NotoSansHebrew-Regular.ttf",
			"truetype/dejavu/DejaVuSans.ttf", // 部分支持
			"truetype/liberation/LiberationSans-Regular.ttf",
		}
	case "vi", "vietnamese", "tr", "turkish", "pt", "portuguese", "es", "spanish", "fr", "french", "de", "german", "it", "italian":
		fontCandidates = []string{
			"truetype/dejavu/DejaVuSans.ttf",
			"truetype/liberation/LiberationSans-Regular.ttf",
			"opentype/noto/NotoSans-Regular.ttf",
			"truetype/droid/DroidSans.ttf",
			"truetype/ubuntu/Ubuntu-R.ttf",
		}
	default:
		fontCandidates = []string{
			"truetype/dejavu/DejaVuSans.ttf",
			"truetype/liberation/LiberationSans-Regular.ttf",
			"opentype/noto/NotoSans-Regular.ttf",
			"truetype/droid/DroidSans.ttf",
		}
	}

	// 在多个目录中查找
	for _, dir := range linuxFontsDirs {
		if fontPath := sfd.findFirstExistingFont(dir, fontCandidates); fontPath != "" {
			return fontPath
		}
	}

	return ""
}

// findFirstExistingFont 查找第一个存在的字体文件
func (sfd *SystemFontDetector) findFirstExistingFont(baseDir string, candidates []string) string {
	for _, candidate := range candidates {
		fullPath := filepath.Join(baseDir, candidate)
		if fileExists(fullPath) {
			log.Printf("找到系统字体: %s", fullPath)
			return fullPath
		}
	}
	return ""
}
