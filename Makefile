.PHONY: dev build clean docker-build docker-run

# 开发模式
dev:
	go run dev.go

# 构建生产版本
build:
	go run build.go

# 清理构建文件
clean:
	rm -rf backend/frontend/build
	rm -rf frontend/build
	rm -rf frontend/node_modules
	rm -f epub-translator-web epub-translator-web.exe
	rm -rf uploads outputs

# Docker 构建
docker-build:
	docker build -t epub-translator-web .

# Docker 运行
docker-run:
	docker-compose up -d

# Docker 停止
docker-stop:
	docker-compose down

# 安装依赖
install:
	cd backend && go mod download
	cd frontend && npm install

# 运行测试
test:
	cd backend && go test ./...
