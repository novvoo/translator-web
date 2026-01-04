package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	if err := build(); err != nil {
		fmt.Fprintf(os.Stderr, "构建失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\n✓ 构建完成！运行 ./apikeyprobe 启动服务器")
}

func build() error {
	// 1. 构建前端
	fmt.Println("📦 构建前端...")
	if err := buildFrontend(); err != nil {
		return fmt.Errorf("前端构建失败: %w", err)
	}

	// 2. 复制前端文件到 backend
	fmt.Println("\n📋 复制前端构建文件...")
	if err := copyFrontendToBacked(); err != nil {
		return fmt.Errorf("复制前端文件失败: %w", err)
	}

	// 3. 构建 Go 后端
	fmt.Println("\n🔨 构建 Go 后端...")
	if err := buildBackend(); err != nil {
		return fmt.Errorf("后端构建失败: %w", err)
	}

	return nil
}

func buildFrontend() error {
	// 安装依赖
	fmt.Println("  安装 npm 依赖...")
	npmInstall := exec.Command(getNpmCmd(), "install")
	npmInstall.Dir = "frontend"
	npmInstall.Stdout = os.Stdout
	npmInstall.Stderr = os.Stderr
	if err := npmInstall.Run(); err != nil {
		return err
	}

	// 构建
	fmt.Println("  运行 npm build...")
	npmBuild := exec.Command(getNpmCmd(), "run", "build")
	npmBuild.Dir = "frontend"
	npmBuild.Stdout = os.Stdout
	npmBuild.Stderr = os.Stderr
	return npmBuild.Run()
}

func copyFrontendToBackend() error {
	src := "frontend/build"
	dst := "backend/frontend/build"

	// 删除旧的构建文件
	if err := os.RemoveAll(dst); err != nil && !os.IsNotExist(err) {
		return err
	}

	// 创建目标目录
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// 递归复制
	return copyDir(src, dst)
}

func buildBackend() error {
	outputName := "epub-translator-web"
	if runtime.GOOS == "windows" {
		outputName += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", "../"+outputName)
	cmd.Dir = "backend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// 辅助函数：递归复制目录
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算目标路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// 辅助函数：复制单个文件
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// 辅助函数：获取 npm 命令（Windows 使用 npm.cmd）
func getNpmCmd() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}
