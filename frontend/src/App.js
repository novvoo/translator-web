import React, { useState, useEffect } from 'react';
import {
  Container,
  Paper,
  Typography,
  Box,
  Button,
  TextField,
  LinearProgress,
  Alert,
  Card,
  CardContent,
  Grid,
  Chip,
  IconButton,
  Divider,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  FormControlLabel,
  Checkbox,
  Tooltip,
} from '@mui/material';
import {
  CloudUpload,
  Download,
  Refresh,
  Settings,
  DeleteOutline,
} from '@mui/icons-material';
import axios from 'axios';

function App() {
  // 从 localStorage 加载保存的配置
  const loadConfig = (key, defaultValue) => {
    try {
      const saved = localStorage.getItem(key);
      return saved !== null ? JSON.parse(saved) : defaultValue;
    } catch {
      return defaultValue;
    }
  };

  // 加载自定义API配置
  const loadCustomConfig = () => {
    try {
      const saved = localStorage.getItem('customApiConfig');
      return saved ? JSON.parse(saved) : { apiUrl: '', model: '', apiKey: '' };
    } catch {
      return { apiUrl: '', model: '', apiKey: '' };
    }
  };

  const [file, setFile] = useState(null);
  const [targetLanguage, setTargetLanguage] = useState(() => loadConfig('targetLanguage', 'Uni'));
  const [sourceLanguage, setSourceLanguage] = useState(() => loadConfig('sourceLanguage', 'English'));
  const [provider, setProvider] = useState(() => loadConfig('provider', 'openai'));
  const [apiKey, setApiKey] = useState(() => loadConfig('apiKey', ''));
  const [apiUrl, setApiUrl] = useState(() => loadConfig('apiUrl', 'https://api.openai.com/v1/chat/completions'));
  const [model, setModel] = useState(() => loadConfig('model', 'gpt-4'));
  const [temperature, setTemperature] = useState(() => loadConfig('temperature', 0.3));
  const [userPrompt, setUserPrompt] = useState(() => loadConfig('userPrompt', ''));
  const [forceRetranslate, setForceRetranslate] = useState(false);
  const [generateMode, setGenerateMode] = useState(() => loadConfig('generateMode', 'bilingual')); // 新增：生成模式
  const [tasks, setTasks] = useState([]);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState('');
  
  // 自定义API配置状态
  const [customApiConfig, setCustomApiConfig] = useState(() => loadCustomConfig());

  const languages = [
    'Uni', 'English', 'Japanese', 'Korean', 'French',
    'German', 'Spanish', 'Russian', 'Arabic', 'Portuguese'
  ];

  const providers = [
    { value: 'openai', label: 'OpenAI', defaultUrl: 'https://api.openai.com/v1/chat/completions', defaultModel: 'gpt-4' },
    { value: 'claude', label: 'Claude (Anthropic)', defaultUrl: 'https://api.anthropic.com/v1/messages', defaultModel: 'claude-3-5-sonnet-20241022' },
    { value: 'gemini', label: 'Google Gemini', defaultUrl: 'https://generativelanguage.googleapis.com/v1/models/gemini-pro:generateContent', defaultModel: 'gemini-pro' },
    { value: 'deepseek', label: 'DeepSeek', defaultUrl: 'https://api.deepseek.com/v1/chat/completions', defaultModel: 'deepseek-chat' },
    { value: 'ollama', label: 'Ollama (本地)', defaultUrl: 'http://localhost:11434/api/generate', defaultModel: 'llama2', noApiKey: true },
    { value: 'nltranslator', label: 'NLTranslator (Apple 翻译)', defaultUrl: 'http://localhost:8765/translate', defaultModel: '', noApiKey: true, modelOptional: true },
    { value: 'libretranslate', label: 'LibreTranslate', defaultUrl: 'https://libretranslate.com/translate', defaultModel: '', modelOptional: true, apiKeyOptional: true },
    { value: 'custom', label: '自定义 API', defaultUrl: '', defaultModel: '', modelOptional: true },
  ];

  // 保存配置到 localStorage
  useEffect(() => {
    localStorage.setItem('targetLanguage', JSON.stringify(targetLanguage));
  }, [targetLanguage]);

  useEffect(() => {
    localStorage.setItem('sourceLanguage', JSON.stringify(sourceLanguage));
  }, [sourceLanguage]);

  useEffect(() => {
    localStorage.setItem('provider', JSON.stringify(provider));
  }, [provider]);

  useEffect(() => {
    localStorage.setItem('apiKey', JSON.stringify(apiKey));
  }, [apiKey]);

  useEffect(() => {
    localStorage.setItem('apiUrl', JSON.stringify(apiUrl));
  }, [apiUrl]);

  useEffect(() => {
    localStorage.setItem('model', JSON.stringify(model));
  }, [model]);

  useEffect(() => {
    localStorage.setItem('temperature', JSON.stringify(temperature));
  }, [temperature]);

  useEffect(() => {
    localStorage.setItem('userPrompt', JSON.stringify(userPrompt));
  }, [userPrompt]);

  useEffect(() => {
    localStorage.setItem('generateMode', JSON.stringify(generateMode));
  }, [generateMode]);

  // 保存自定义API配置
  useEffect(() => {
    localStorage.setItem('customApiConfig', JSON.stringify(customApiConfig));
  }, [customApiConfig]);

  // 加载任务列表
  const loadTasks = async () => {
    try {
      const response = await axios.get('/api/tasks');
      const taskList = response.data.tasks || [];
      setTasks(taskList);

      // 返回是否有活跃任务
      return taskList.some(task =>
        task.status === 'processing' || task.status === 'pending'
      );
    } catch (err) {
      console.error('加载任务失败:', err);
      return false;
    }
  };

  useEffect(() => {
    // 初始加载
    loadTasks();

    // 动态刷新：有活跃任务时 3 秒刷新一次，否则 15 秒刷新一次
    let isActive = true;
    let timeoutId;

    const scheduleNextRefresh = async () => {
      if (!isActive) return;
      
      const hasActiveTasks = await loadTasks();
      const delay = hasActiveTasks ? 3000 : 15000; // 活跃任务 3 秒，否则 15 秒

      if (isActive) {
        timeoutId = setTimeout(scheduleNextRefresh, delay);
      }
    };

    // 启动第一次刷新
    timeoutId = setTimeout(scheduleNextRefresh, 3000);

    return () => {
      isActive = false;
      if (timeoutId) {
        clearTimeout(timeoutId);
      }
    };
  }, []); // 空依赖数组，只在组件挂载时设置一次

  const handleFileChange = (event) => {
    const selectedFile = event.target.files[0];
    if (selectedFile && (selectedFile.name.endsWith('.epub') || selectedFile.name.endsWith('.pdf'))) {
      setFile(selectedFile);
      setError('');
    } else {
      setError('请选择 .epub 或 .pdf 文件');
      setFile(null);
    }
  };

  const handleProviderChange = (newProvider) => {
    // 如果当前是自定义API，保存配置
    if (provider === 'custom') {
      setCustomApiConfig({
        apiUrl: apiUrl,
        model: model,
        apiKey: apiKey
      });
    }
    
    setProvider(newProvider);
    const providerConfig = providers.find(p => p.value === newProvider);
    
    if (newProvider === 'custom') {
      // 切换到自定义API时，恢复保存的配置
      setApiUrl(customApiConfig.apiUrl || '');
      setModel(customApiConfig.model || '');
      setApiKey(customApiConfig.apiKey || '');
    } else if (providerConfig) {
      // 切换到其他提供商时，使用默认配置
      setApiUrl(providerConfig.defaultUrl);
      setModel(providerConfig.defaultModel);
    }
  };

  const handleClearConfig = () => {
    if (window.confirm('确定要清除所有保存的配置吗？')) {
      localStorage.removeItem('targetLanguage');
      localStorage.removeItem('sourceLanguage');
      localStorage.removeItem('provider');
      localStorage.removeItem('apiKey');
      localStorage.removeItem('apiUrl');
      localStorage.removeItem('model');
      localStorage.removeItem('temperature');
      localStorage.removeItem('userPrompt');
      localStorage.removeItem('customApiConfig'); // 清除自定义API配置
      localStorage.removeItem('generateMode'); // 清除生成模式配置

      // 重置为默认值
      setTargetLanguage('Uni');
      setSourceLanguage('English');
      setProvider('openai');
      setApiKey('');
      setApiUrl('https://api.openai.com/v1/chat/completions');
      setModel('gpt-4');
      setTemperature(0.3);
      setUserPrompt('');
      setGenerateMode('bilingual'); // 重置生成模式
      setCustomApiConfig({ apiUrl: '', model: '', apiKey: '' });
    }
  };

  const handleUpload = async () => {
    if (!file) {
      setError('请选择文件');
      return;
    }
    const currentProvider = providers.find(p => p.value === provider);
    if (!currentProvider?.noApiKey && !currentProvider?.apiKeyOptional && !apiKey) {
      setError('请输入 API Key');
      return;
    }

    setUploading(true);
    setError('');

    const formData = new FormData();
    formData.append('file', file);
    formData.append('targetLanguage', targetLanguage);

    // LLM 配置
    const llmConfig = {
      provider: provider,
      apiKey: apiKey,
      apiUrl: apiUrl,
      model: model,
      temperature: temperature,
      maxTokens: 4000,
      extra: (provider === 'nltranslator' || provider === 'libretranslate') ? { sourceLanguage: sourceLanguage } : {},
    };

    formData.append('llmConfig', JSON.stringify(llmConfig));
    if (userPrompt) {
      formData.append('userPrompt', userPrompt);
    }
    formData.append('forceRetranslate', forceRetranslate.toString());
    formData.append('generateMode', generateMode); // 新增：生成模式

    try {
      const response = await axios.post('/api/translate', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      });

      setFile(null);
      setForceRetranslate(false); // 重置选项
      loadTasks();
    } catch (err) {
      setError(err.response?.data?.error || '上传失败');
    } finally {
      setUploading(false);
    }
  };

  const handleDownload = async (taskId, filename) => {
    try {
      const response = await axios.get(`/api/download/${taskId}`, {
        responseType: 'blob',
      });

      const url = window.URL.createObjectURL(new Blob([response.data]));
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', `translated_${filename}`);
      document.body.appendChild(link);
      link.click();
      link.remove();
    } catch (err) {
      alert('下载失败: ' + (err.response?.data?.error || err.message));
    }
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'completed': return 'success';
      case 'processing': return 'primary';
      case 'failed': return 'error';
      default: return 'default';
    }
  };

  const getStatusText = (status) => {
    switch (status) {
      case 'pending': return '等待中';
      case 'processing': return '翻译中';
      case 'completed': return '已完成';
      case 'failed': return '失败';
      default: return status;
    }
  };

  return (
    <Container maxWidth="lg" sx={{ py: 4 }}>
      <Box sx={{ mb: 4, textAlign: 'center' }}>
        <Typography variant="h3" component="h1" gutterBottom>
          📚 文档翻译器
        </Typography>
        <Typography variant="subtitle1" color="text.secondary">
          使用 AI 翻译 EPUB 电子书和 PDF 文档，生成双语对照版本
        </Typography>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 3 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <Paper sx={{ p: 3, mb: 4 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
          <Typography variant="h6">
            <Settings sx={{ verticalAlign: 'middle', mr: 1 }} />
            上传和配置
          </Typography>
          <Button
            size="small"
            startIcon={<DeleteOutline />}
            onClick={handleClearConfig}
            color="error"
            variant="outlined"
          >
            清除配置
          </Button>
        </Box>
        <Divider sx={{ mb: 3 }} />

        <Grid container spacing={3}>
          <Grid item xs={12}>
            <Button
              variant="outlined"
              component="label"
              startIcon={<CloudUpload />}
              fullWidth
              sx={{ py: 2 }}
            >
              {file ? file.name : '选择文档文件 (EPUB/PDF)'}
              <input
                type="file"
                hidden
                accept=".epub,.pdf"
                onChange={handleFileChange}
              />
            </Button>
            {file && file.name.toLowerCase().endsWith('.pdf') && (
              <Alert severity="info" sx={{ mt: 1 }}>
                📄 PDF 翻译说明：
                <br />
                • 使用 Go 原生 PDF 解析器，智能提取文本内容
                <br />
                • 支持数学公式识别和保留
                <br />
                • 输出双语对照的 PDF 文件
                <br />
                • 如遇到格式不兼容的 PDF，系统会自动尝试简化处理
              </Alert>
            )}
          </Grid>

          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <InputLabel>AI 提供商</InputLabel>
              <Select
                value={provider}
                label="AI 提供商"
                onChange={(e) => handleProviderChange(e.target.value)}
              >
                {providers.map((p) => (
                  <MenuItem key={p.value} value={p.value}>
                    {p.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>

          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <InputLabel>目标语言</InputLabel>
              <Select
                value={targetLanguage}
                label="目标语言"
                onChange={(e) => setTargetLanguage(e.target.value)}
              >
                {languages.map((lang) => (
                  <MenuItem key={lang} value={lang}>
                    {lang}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>

          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <InputLabel>生成模式</InputLabel>
              <Select
                value={generateMode}
                label="生成模式"
                onChange={(e) => setGenerateMode(e.target.value)}
              >
                <MenuItem value="bilingual">双语对照（推荐）</MenuItem>
                <MenuItem value="monolingual">仅译文</MenuItem>
              </Select>
            </FormControl>
            <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 1 }}>
              {generateMode === 'bilingual' 
                ? '📖 生成包含原文和译文的对照版本，便于学习和对比'
                : '📝 仅生成翻译后的内容，适合直接阅读'}
            </Typography>
          </Grid>

          {(provider === 'nltranslator' || provider === 'libretranslate') && (
            <Grid item xs={12} md={6}>
              <FormControl fullWidth>
                <InputLabel>原始语言</InputLabel>
                <Select
                  value={sourceLanguage}
                  label="原始语言"
                  onChange={(e) => setSourceLanguage(e.target.value)}
                >
                  {languages.map((lang) => (
                    <MenuItem key={lang} value={lang}>
                      {lang}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          )}

          {(provider !== 'nltranslator' && provider !== 'libretranslate') && (
            <>
              <Grid item xs={12} md={6}>
                <TextField
                  fullWidth
                  label="模型"
                  value={model}
                  onChange={(e) => {
                    setModel(e.target.value);
                    // 如果是自定义API，实时保存配置
                    if (provider === 'custom') {
                      setCustomApiConfig(prev => ({
                        ...prev,
                        model: e.target.value
                      }));
                    }
                  }}
                  placeholder={providers.find(p => p.value === provider)?.defaultModel || ''}
                  helperText="例如: gpt-4, claude-3-5-sonnet, gemini-pro"
                />
              </Grid>

              <Grid item xs={12} md={6}>
                <TextField
                  fullWidth
                  label="Temperature"
                  type="number"
                  value={temperature}
                  onChange={(e) => setTemperature(parseFloat(e.target.value))}
                  inputProps={{ min: 0, max: 2, step: 0.1 }}
                  helperText="控制翻译的创造性 (0-2)"
                />
              </Grid>
            </>
          )}

          {!providers.find(p => p.value === provider)?.noApiKey && (
            <Grid item xs={12}>
              <TextField
                fullWidth
                label={providers.find(p => p.value === provider)?.apiKeyOptional ? "API Key (可选)" : "API Key"}
                type="password"
                value={apiKey}
                onChange={(e) => {
                  setApiKey(e.target.value);
                  // 如果是自定义API，实时保存配置
                  if (provider === 'custom') {
                    setCustomApiConfig(prev => ({
                      ...prev,
                      apiKey: e.target.value
                    }));
                  }
                }}
                placeholder="sk-..."
                required={!providers.find(p => p.value === provider)?.apiKeyOptional}
                helperText={provider === 'libretranslate' ? 'LibreTranslate 公共实例通常不需要 API Key，私有部署可能需要' : '⚠️ API Key 将保存在浏览器本地存储中，请注意安全'}
              />
            </Grid>
          )}

          <Grid item xs={12}>
            <TextField
              fullWidth
              label="API URL"
              value={apiUrl}
              onChange={(e) => {
                setApiUrl(e.target.value);
                // 如果是自定义API，实时保存配置
                if (provider === 'custom') {
                  setCustomApiConfig(prev => ({
                    ...prev,
                    apiUrl: e.target.value
                  }));
                }
              }}
              placeholder="https://api.openai.com/v1/chat/completions"
              helperText={
                provider === 'nltranslator' 
                  ? 'NLTranslator Proxy 服务地址（需要先启动 NLTranslatorProxy）' 
                  : provider === 'libretranslate'
                  ? 'LibreTranslate 服务地址，例如：https://libretranslate.com/translate 或本地部署地址'
                  : ''
              }
            />
          </Grid>

          {(provider !== 'nltranslator' && provider !== 'libretranslate') && (
            <Grid item xs={12}>
              <TextField
                fullWidth
                label="自定义提示词（可选）"
                value={userPrompt}
                onChange={(e) => setUserPrompt(e.target.value)}
                placeholder="例如：使用正式语言，保留技术术语"
                multiline
                rows={2}
              />
            </Grid>
          )}

          <Grid item xs={12}>
            <FormControlLabel
              control={
                <Checkbox
                  checked={forceRetranslate}
                  onChange={(e) => setForceRetranslate(e.target.checked)}
                  color="warning"
                />
              }
              label={
                <Tooltip title="勾选后将忽略已有的翻译缓存，重新翻译所有内容。不勾选则会继续使用缓存，只翻译未完成的部分。">
                  <span>
                    强制重新翻译（忽略缓存）
                  </span>
                </Tooltip>
              }
            />
            <Typography variant="caption" color="text.secondary" display="block" sx={{ ml: 4 }}>
              {forceRetranslate
                ? '⚠️ 将重新翻译所有内容，不使用已有缓存'
                : '✓ 继续翻译模式：将使用已有缓存，只翻译未完成的部分'}
            </Typography>
          </Grid>

          <Grid item xs={12}>
            <Button
              variant="contained"
              size="large"
              fullWidth
              onClick={handleUpload}
              disabled={!file || uploading || (!providers.find(p => p.value === provider)?.noApiKey && !providers.find(p => p.value === provider)?.apiKeyOptional && !apiKey)}
              startIcon={<CloudUpload />}
            >
              {uploading ? '上传中...' : (forceRetranslate ? '开始重新翻译' : '开始翻译')}
            </Button>
          </Grid>
        </Grid>
      </Paper>

      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Typography variant="h6">
          翻译任务
        </Typography>
        <IconButton onClick={loadTasks} size="small">
          <Refresh />
        </IconButton>
      </Box>

      {tasks.length === 0 ? (
        <Paper sx={{ p: 4, textAlign: 'center' }}>
          <Typography color="text.secondary">
            暂无翻译任务
          </Typography>
        </Paper>
      ) : (
        <Grid container spacing={2}>
          {tasks.map((task) => (
            <Grid item xs={12} key={task.id}>
              <Card>
                <CardContent>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
                    <Box>
                      <Typography variant="h6" gutterBottom>
                        {task.sourceFile}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        目标语言: {task.targetLanguage}
                      </Typography>
                    </Box>
                    <Chip
                      label={getStatusText(task.status)}
                      color={getStatusColor(task.status)}
                      size="small"
                    />
                  </Box>

                  {task.status === 'processing' && (
                    <Box sx={{ mb: 2 }}>
                      <LinearProgress
                        variant="determinate"
                        value={task.progress * 100}
                      />
                      <Typography variant="caption" color="text.secondary" sx={{ mt: 0.5 }}>
                        进度: {Math.round(task.progress * 100)}%
                      </Typography>
                    </Box>
                  )}

                  {task.status === 'failed' && task.error && (
                    <Alert severity="error" sx={{ mb: 2 }}>
                      {task.error}
                    </Alert>
                  )}

                  {task.status === 'completed' && (
                    <Button
                      variant="contained"
                      startIcon={<Download />}
                      onClick={() => handleDownload(task.id, task.sourceFile)}
                    >
                      下载翻译文件 {task.sourceFile.toLowerCase().endsWith('.pdf') }
                    </Button>
                  )}

                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 2 }}>
                    创建时间: {new Date(task.createdAt).toLocaleString('zh-CN')}
                  </Typography>
                </CardContent>
              </Card>
            </Grid>
          ))}
        </Grid>
      )}
    </Container>
  );
}

export default App;
