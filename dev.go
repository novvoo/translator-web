package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func main() {
	fmt.Println("🚀 启动 EPUB Translator 开发环境...\n")

	// 创建信号通道用于优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 安装前端依赖
	fmt.Println("📦 安装前端依赖...")
	npmInstall := exec.Command(getNpmCmd(), "install")
	npmInstall.Dir = "frontend"
	npmInstall.Stdout = os.Stdout
	npmInstall.Stderr = os.Stderr
	if err := npmInstall.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "安装前端依赖失败: %v\n", err)
		os.Exit(1)
	}

	// 启动前端开发服务器
	fmt.Println("📦 启动前端开发服务器 (端口 3000)...")
	frontendCmd := exec.Command(getNpxCmd(), "react-scripts", "start")
	frontendCmd.Dir = "frontend"
	frontendCmd.Env = append(os.Environ(), "BROWSER=none", "NODE_OPTIONS=--no-deprecation")
	frontendCmd.Stdout = os.Stdout
	frontendCmd.Stderr = os.Stderr
	if err := frontendCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动前端失败: %v\n", err)
		os.Exit(1)
	}

	// 等待前端服务器启动
	fmt.Println("⏳ 等待前端服务器启动...")
	if !waitForFrontend("http://localhost:3000", 60) {
		fmt.Fprintf(os.Stderr, "前端服务器启动超时\n")
		frontendCmd.Process.Kill()
		os.Exit(1)
	}
	fmt.Println("✓ 前端服务器已就绪")

	// 启动后端服务器（开发模式）
	fmt.Println("🔧 启动后端服务器（开发模式）...")
	backendCmd := exec.Command("go", "run", "main.go")
	backendCmd.Dir = "backend"
	backendCmd.Env = append(os.Environ(), "DEV_MODE=true")
	backendCmd.Stdout = os.Stdout
	backendCmd.Stderr = os.Stderr
	if err := backendCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动后端失败: %v\n", err)
		frontendCmd.Process.Kill()
		os.Exit(1)
	}

	fmt.Println("\n✓ 开发环境已启动！")
	fmt.Println("  访问: http://localhost:8080")
	fmt.Println("  (后端会自动代理前端请求)")
	fmt.Println("\n按 Ctrl+C 停止所有服务\n")

	// 等待中断信号
	<-sigChan

	fmt.Println("\n\n🛑 正在停止服务...")

	// 停止进程
	if frontendCmd.Process != nil {
		frontendCmd.Process.Kill()
	}
	if backendCmd.Process != nil {
		backendCmd.Process.Kill()
	}

	fmt.Println("✓ 已停止所有服务")
}

func getNpmCmd() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

func getNpxCmd() string {
	if runtime.GOOS == "windows" {
		return "npx.cmd"
	}
	return "npx"
}

// waitForFrontend 等待前端服务器启动（最多等待 timeout 秒）
func waitForFrontend(url string, timeout int) bool {
	for i := 0; i < timeout; i++ {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(1 * time.Second)
		if i%5 == 0 && i > 0 {
			fmt.Printf("  仍在等待... (%d秒)\n", i)
		}
	}
	return false
}
