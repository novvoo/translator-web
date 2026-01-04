package main

import (
	"embed"
	"epub-translator-web/handlers"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
)

var frontendFS embed.FS

func main() {
	r := gin.Default()

	// 设置最大上传文件大小 (100MB)
	r.MaxMultipartMemory = 100 << 20

	// API 路由
	api := r.Group("/api")
	{
		api.POST("/translate", handlers.TranslateHandler)
		api.GET("/status/:taskId", handlers.GetStatusHandler)
		api.GET("/download/:taskId", handlers.DownloadHandler)
		api.GET("/tasks", handlers.GetTasksHandler)
	}

	// 根据环境变量决定前端服务方式
	devMode := os.Getenv("DEV_MODE") == "true"

	if devMode {
		// 开发模式：代理到前端开发服务器
		log.Println("🔧 开发模式：代理前端请求到 http://localhost:3000")
		target, _ := url.Parse("http://localhost:3000")
		proxy := httputil.NewSingleHostReverseProxy(target)

		r.NoRoute(func(c *gin.Context) {
			proxy.ServeHTTP(c.Writer, c.Request)
		})
	} else {
		// 生产模式：使用内嵌的前端文件
		log.Println("📦 生产模式：使用内嵌前端文件")
		buildFS, err := fs.Sub(frontendFS, "frontend/build")
		if err != nil {
			log.Println("⚠️  警告：前端文件未找到，请先运行 'go run build.go' 构建前端")
			log.Println("    或使用开发模式：DEV_MODE=true go run main.go")
			panic(err)
		}
		r.NoRoute(gin.WrapH(http.FileServer(http.FS(buildFS))))
	}

	log.Println("🚀 EPUB Translator 服务器启动在 http://localhost:8080")
	r.Run(":8080")
}
