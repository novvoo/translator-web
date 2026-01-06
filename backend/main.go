package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"translator-web/handlers"
	"translator-web/middleware"

	"github.com/gin-gonic/gin"
)

//go:embed all:frontend/build
var frontendFS embed.FS

func main() {
	r := gin.Default()

	// 设置最大上传文件大小 (100MB)
	r.MaxMultipartMemory = 100 << 20

	// 应用会话中间件到所有路由
	r.Use(middleware.SessionMiddleware())

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

		// 尝试读取嵌入的文件系统
		entries, err := fs.ReadDir(frontendFS, ".")
		if err != nil || len(entries) == 0 {
			log.Println("⚠️  警告：前端文件未找到")
			r.NoRoute(func(c *gin.Context) {
				c.String(http.StatusNotFound, "Frontend not built. Please run 'go run build.go' first or set DEV_MODE=true")
			})
		} else {
			buildFS, err := fs.Sub(frontendFS, "frontend/build")
			if err != nil {
				log.Printf("⚠️  错误：无法访问前端文件: %v\n", err)
				r.NoRoute(func(c *gin.Context) {
					c.String(http.StatusNotFound, "Frontend files error: "+err.Error())
				})
			} else {
				r.NoRoute(gin.WrapH(http.FileServer(http.FS(buildFS))))
			}
		}
	}

	log.Println("🚀 文档翻译器服务器启动在 http://localhost:8080")
	log.Println("✅ 会话隔离已启用 - 每个用户的任务和文件完全独立")
	r.Run(":8080")
}
