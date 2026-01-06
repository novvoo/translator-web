# Document Translator Web

基于 Go + React 的文档翻译工具，使用 AI 生成双语对照版本。支持 EPUB 电子书和 PDF 文档。

## 🎉 最新更新：PDF 支持

**版本 2.1.0** 现已支持 PDF 文档翻译！

### 新功能亮点
- ✅ **PDF 文档支持** - 支持上传和翻译 PDF 文件
- ✅ **统一处理接口** - EPUB 和 PDF 使用相同的翻译流程
- ✅ **智能文本提取** - 自动提取 PDF 中的文本内容
- ✅ **双语文本输出** - PDF 翻译结果保存为双语对照文本文件
- ✅ **自动会话管理** - 无需注册登录，自动为每个访问者创建独立会话
- ✅ **完全数据隔离** - 每个用户独立的任务列表、文件存储和翻译缓存
- ✅ **安全权限控制** - 无法访问其他用户的任务和文件
- ✅ **适合公共部署** - 可以安全地部署为公共服务

📖 **详细文档**:
- [会话隔离功能说明](./README_SESSION_ISOLATION.md)
- [快速启动指南](./QUICKSTART_SESSION.md)
- [技术文档](./SESSION_ISOLATION.md)
- [测试指南](./test_session_isolation.md)

## 功能特性

- 📚 上传 EPUB 文件进行翻译
- 📄 上传 PDF 文件进行翻译（新功能！）
- 🔒 **多用户会话隔离** - 每个用户的数据完全独立
- 🤖 **支持多种 AI 提供商**：
  - OpenAI (GPT-4, GPT-3.5)
  - Claude (Anthropic)
  - Google Gemini
  - DeepSeek
  - Ollama (本地模型)
  - 自定义 API（任何 OpenAI 兼容接口）
- 🌍 支持多种目标语言
- 📊 实时显示翻译进度
- 💾 自动缓存翻译结果
- 📥 下载双语对照的 EPUB 文件或文本文件
- ⚙️ 灵活的模型配置和参数调优

## 支持的文件格式

- **EPUB**: 电子书格式，输出双语对照的 EPUB 文件
- **PDF**: 便携式文档格式，输出双语对照的文本文件

## 技术栈

- **后端**: Go + Gin（内嵌前端）
- **前端**: React + Material-UI
- **架构**: 单一可执行文件，无需额外依赖

## 快速启动

### 首次使用

如果遇到 `pattern all:frontend/build: no matching files found` 错误，说明前端还未构建。有两种解决方案：

**方案 1：使用开发模式（推荐）**
```bash
go run dev.go
```
开发模式会自动启动前端开发服务器，无需预先构建。

**方案 2：先构建前端**
```bash
cd frontend
npm install
npm run build
cd ..
```

### 方式一：使用启动脚本（推荐）

**Windows:**
```bash
start.bat
```

**Linux/Mac:**
```bash
chmod +x start.sh
./start.sh
```

### 方式二：使用 Go 命令

**开发模式（支持热重载）:**
```bash
go run dev.go
```

**生产模式（构建单一可执行文件）:**
```bash
go run build.go
./translator-web      # Linux/Mac
translator-web.exe    # Windows
```

### 方式三：使用 Make

```bash
make dev          # 开发模式
make build        # 构建生产版本
make docker-build # Docker 构建
make docker-run   # Docker 运行
```

### 方式四：使用 Docker

```bash
docker-compose up -d
```

访问 http://localhost:8080

构建后会生成单一可执行文件，包含前端和后端，无需额外依赖。

## 使用说明

1. **上传文档文件**：点击"选择文档文件"按钮，支持 .epub 和 .pdf 格式
2. **配置翻译参数**：
   - **选择 AI 提供商**：OpenAI、Claude、Gemini、DeepSeek、Ollama 或自定义
   - 选择目标语言
   - 输入 API Key（Ollama 本地模型无需）
   - 选择模型（系统会根据提供商自动填充推荐模型）
   - 调整 Temperature（0-2，控制翻译的创造性）
   - （可选）添加自定义提示词
3. **开始翻译**：点击"开始翻译"按钮
4. **查看进度**：在任务列表中实时查看翻译进度
5. **下载结果**：翻译完成后点击"下载翻译文件"

## AI 提供商配置

详细的配置指南请查看 [AI_PROVIDERS.md](./AI_PROVIDERS.md)

### 快速配置示例

### 快速配置示例

#### OpenAI

```
Provider: openai
API URL: https://api.openai.com/v1/chat/completions
Model: gpt-4
API Key: sk-...
```

#### Claude (Anthropic)

```
Provider: claude
API URL: https://api.anthropic.com/v1/messages
Model: claude-3-5-sonnet-20241022
API Key: sk-ant-...
```

#### Google Gemini

```
Provider: gemini
API URL: https://generativelanguage.googleapis.com/v1/models/gemini-pro:generateContent
Model: gemini-pro
API Key: your-google-api-key
```

#### DeepSeek

```
Provider: deepseek
API URL: https://api.deepseek.com/v1/chat/completions
Model: deepseek-chat
API Key: your-deepseek-key
```

#### Ollama (本地模型)

```bash
# 首先安装并启动 Ollama
ollama pull llama2
ollama serve
```

```
Provider: ollama
API URL: http://localhost:11434/api/generate
Model: llama2
API Key: (留空)
```

#### Azure OpenAI

```
Provider: custom
API URL: https://your-resource.openai.azure.com/openai/deployments/your-deployment/chat/completions?api-version=2023-05-15
Model: gpt-4
API Key: your-azure-key
```

更多配置示例和详细说明，请查看：
- [AI 提供商配置指南](./AI_PROVIDERS.md)
- [使用示例](./USAGE_EXAMPLES.md)

## 项目结构

```
translator-web/
├── backend/
│   ├── main.go              # 主程序入口
│   ├── handlers/            # API 处理器
│   │   └── translate.go     # 翻译相关 API
│   ├── models/              # 数据模型
│   │   └── task.go          # 任务模型
│   ├── translator/          # 翻译核心
│   │   ├── epub.go          # EPUB 文件处理
│   │   ├── llm.go           # 旧版 LLM 客户端（兼容）
│   │   ├── provider.go      # AI 提供商实现
│   │   ├── client.go        # 新版翻译客户端
│   │   ├── cache.go         # 翻译缓存
│   │   ├── toc.go           # 目录翻译
│   │   └── metadata.go      # 元数据翻译
│   └── go.mod               # Go 依赖
├── frontend/
│   ├── src/
│   │   ├── App.js           # React 主组件
│   │   └── index.js         # 入口文件
│   ├── public/
│   │   └── index.html       # HTML 模板
│   └── package.json         # npm 依赖
├── dev.go                   # 开发模式启动脚本
├── build.go                 # 生产构建脚本
└── README.md
```

## API 接口

### POST /api/translate
上传 EPUB 文件并开始翻译

**参数**:
- `file`: EPUB 文件
- `targetLanguage`: 目标语言
- `llmConfig`: LLM 配置（JSON 字符串）
  - `provider`: 提供商类型（openai/claude/gemini/deepseek/ollama/custom）
  - `apiKey`: API Key（Ollama 可选）
  - `apiUrl`: API URL
  - `model`: 模型名称
  - `temperature`: 温度参数（0-2）
  - `maxTokens`: 最大 token 数
  - `extra`: 额外参数（可选，用于自定义提供商）
- `userPrompt`: 自定义提示词（可选）

**请求示例**:
```bash
curl -X POST http://localhost:8080/api/translate \
  -F "file=@book.epub" \
  -F "targetLanguage=Chinese" \
  -F 'llmConfig={
    "provider": "openai",
    "apiKey": "sk-xxx",
    "apiUrl": "https://api.openai.com/v1/chat/completions",
    "model": "gpt-4",
    "temperature": 0.3,
    "maxTokens": 4000
  }'
```

**返回**:
```json
{
  "taskId": "uuid",
  "message": "翻译任务已创建"
}
```

### GET /api/status/:taskId
获取任务状态

**返回**:
```json
{
  "id": "uuid",
  "sourceFile": "book.epub",
  "targetLanguage": "Chinese",
  "status": "processing",
  "progress": 0.5,
  "createdAt": "2024-01-01T00:00:00Z"
}
```

### GET /api/download/:taskId
下载翻译后的文件

### GET /api/tasks
获取所有任务列表

## 注意事项

- EPUB 文件大小限制：100MB
- 翻译时间取决于文件大小和 API 响应速度
- 建议使用 GPT-4 或更高级的模型以获得更好的翻译质量
- API Key 仅在内存中使用，不会被存储

## 部署

### Fly.io 部署

```bash
# 安装 flyctl
curl -L https://fly.io/install.sh | sh

# 登录
flyctl auth login

# 创建应用
flyctl launch

# 部署
flyctl deploy
```

### Docker 部署

```bash
# 构建镜像
docker build -t translator-web .

# 运行容器
docker run -d -p 8080:8080 \
  -v $(pwd)/uploads:/root/uploads \
  -v $(pwd)/outputs:/root/outputs \
  translator-web
```

### 传统服务器部署

```bash
# 构建
go run build.go

# 复制可执行文件到服务器
scp translator-web user@server:/path/to/app/

# 在服务器上运行
./translator-web
```

## 许可证

MIT License
