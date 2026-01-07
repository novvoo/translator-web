# 多阶段构建
FROM node:18-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Go 构建阶段
FROM golang:1.24.1-alpine AS backend-builder

WORKDIR /app
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /app/backend
RUN go mod download

COPY backend/ ./
COPY --from=frontend-builder /app/frontend/build ./frontend/build

RUN CGO_ENABLED=0 GOOS=linux go build -o /translator-web

# 最终运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY --from=backend-builder /translator-web .

# 创建必要的目录
RUN mkdir -p uploads outputs

EXPOSE 8080

CMD ["./translator-web"]
