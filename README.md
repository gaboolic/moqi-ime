# MOQI IME

使用 Go 语言实现的输入法核心后端引擎。

## 项目结构

```
go-backend/
├── imecore/                 # PIME 核心库
│   ├── protocol.go     # 通信协议定义
│   ├── client.go       # 客户端和服务接口
│   ├── service.go      # 文本服务基础实现
│   └── tray.go         # 托盘按钮辅助方法
├── example/            # 示例实现
│   └── simple_ime/     # 简单输入法示例
│       ├── main.go     # 入口文件
│       └── ime.go      # 输入法实现
├── go.mod              # Go 模块定义
└── README.md           # 说明文档
```

## 快速开始

### 1. 编译

```bash
cd go-backend
build.bat
```

`build.bat` 会生成可直接安装的运行目录：

```text
build/
├── backends.go-backend.json
└── go-backend/
    ├── server.exe
    └── input_methods/
```

### 2. 配置 PIME

在 PIME 根目录的 `backends.json` 中添加 Go 后端配置。

注意：这个仓库里的 `backends.json` 顶层是数组，不是 `{ "backends": [...] }`。

```json
[
  {
    "name": "go-backend",
    "command": "go-backend\\server.exe",
    "workingDir": "go-backend",
    "params": ""
  }
]
```

### 3. 注册输入法

确保 `C:\Program Files (x86)\PIME\go-backend\input_methods\*\ime.json` 存在。比如：

```json
{
  "name": "GoSimpleIME",
  "icon": "icon.ico",
  "backend": "go-backend"
}
```

## 开发自定义输入法

### 1. 实现 TextService 接口

```go
type MyIME struct {
    *imecore.TextServiceBase
    // 自定义字段
}

func NewMyIME(client *imecore.Client) *MyIME {
    return &MyIME{
        TextServiceBase: imecore.NewTextServiceBase(client),
    }
}

func (ime *MyIME) HandleRequest(req *imecore.Request) *imecore.Response {
    resp := imecore.NewResponse(req.SeqNum, true)
    
    switch req.Method {
    case "filterKeyDown":
        // 处理按键
        return ime.handleKeyDown(req, resp)
    // ... 其他方法
    }
    
    return resp
}
```

### 2. 注册到主服务

```go
func main() {
    server := NewServer()
    
    // 注册输入法
    server.RegisterService("your-ime-guid", func(client *imecore.Client, guid string) imecore.TextService {
        return NewMyIME(client)
    })
    
    // 运行服务
    if err := server.Run(); err != nil {
        log.Fatal(err)
    }
}
```

## 协议说明

### 通信方式

- 使用 stdin/stdout 进行通信
- 每行一条消息
- JSON 格式

### 请求格式

```
<client_id>|<JSON>
```

### 响应格式

```
PIME_MSG|<client_id>|<JSON>
```

### 消息类型

#### 初始化
```json
{
  "method": "init",
  "id": "client_guid",
  "isWindows8Above": true,
  "isMetroApp": false,
  "isUiLess": false,
  "isConsole": false
}
```

#### 按键处理
```json
{
  "method": "filterKeyDown",
  "keyCode": 65,
  "charCode": 97,
  "scanCode": 30
}
```

#### 响应
```json
{
  "success": true,
  "returnValue": 1,
  "compositionString": "a",
  "candidateList": ["啊", "阿", "吖"],
  "showCandidates": true
}
```

## Rime AI 写好评

`go-backend/input_methods/rime` 现在支持一个基于当前 composition 的 AI 候选生成功能。

### 配置文件

优先读取下面任一位置的 `ai_config.json`：

- `%APPDATA%/PIME/Rime/ai_config.json`
- `input_methods/rime/ai_config.json`

首次启动时，如果用户目录下还没有 `ai_config.json`，程序会自动把 `input_methods/rime/ai_config.json` 复制到 `%APPDATA%/PIME/Rime/`，已存在时不会覆盖。

配置示例：

```json
{
  "api": {
    "base_url": "https://api.openai.com/v1",
    "api_key": "replace-with-your-token",
    "model": "gpt-4o-mini"
  },
  "actions": [
    {
      "name": "写好评",
      "hotkey": "Ctrl+Shift+G",
      "prompt": "请围绕“{{composition}}”生成最多 3 条适合直接发布的中文好评，每条 20 字左右。"
    }
  ]
}
```

- `api.base_url`: OpenAI 兼容接口基础地址
- `api.api_key`: 接口密钥
- `api.model`: 模型名
- `actions[].hotkey`: 触发热键，目前支持字母和数字主键，例如 `Ctrl+Shift+G`、`Ctrl+Alt+1`
- `actions[].prompt`: 提示词模板，支持 `{{previous_commit}}`、`{{composition}}`、`{{candidate_1}}`、`{{candidate_2}}`、`{{candidate_3}}`、`{{candidates_top3}}` 等占位符
- 例如输入了 `kafeiji`，当前前三个候选是 `咖啡机 / 咖啡壶 / 咖啡杯`，那么 `{{composition}}` 会替换成 `kafeiji`，`{{candidate_1}}` 会替换成 `咖啡机`

### 使用方式

1. 先在 Rime 里输入一个主题词，例如 `咖啡机`
2. 按配置文件中对应的热键，例如 `Ctrl+Shift+G`
3. 等待 1 到 3 秒
4. 在候选窗里用数字键、上下方向键或回车选择 AI 生成的好评
5. 按 `Esc` 可取消 AI 候选并回到原来的输入状态

### 环境变量

如果没有找到 `ai_config.json`，会回退到下面 3 个环境变量：

- `MOQI_AI_BASE_URL`: OpenAI 兼容接口的基础地址，例如 `https://api.openai.com/v1`
- `MOQI_AI_API_KEY`: 接口密钥
- `MOQI_AI_MODEL`: 模型名，例如 `gpt-4o-mini`

### 行为说明

- AI 生成基于当前 `compositionString`，不会读取宿主应用里的选中文本
- 第一版是同步调用，接口超时默认 20 秒
- AI 返回结果会被整理成最多 3 条单行候选
- 如果接口失败、超时或未返回可用结果，输入法会保留原来的 Rime 组合串，不会误上屏

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

LGPL-2.1 License
