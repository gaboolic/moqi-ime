# MOQI IME

基于 Go 的输入法后端进程：通过标准输入/标准输出与 Windows 端宿主（`moqi-im-windows` / Moqi IME）以行协议通信，为多种输入法实现提供统一运行时。

## 仓库根目录结构

```
moqi-ime/
├── server.go              # 程序入口：日志、加载输入法、stdin 主循环
├── go.mod                 # 模块名：github.com/gaboolic/moqi-ime
├── imecore/               # 协议与 TextService 抽象
│   ├── protocol.go        # 请求/响应结构、MOQI_MSG 常量
│   ├── client.go          # 客户端会话信息
│   └── service.go         # TextServiceBase 默认按键/生命周期实现
└── input_methods/         # 与可执行文件同目录部署的子目录（见下文）
    ├── fcitx5/
    ├── meow/
    ├── rime/
    └── simple_pinyin/
```

每个输入法目录下通常包含 `ime.json`（至少含 `guid`、`name` 等），以及各实现所需的资源（如 Rime 的 `data/`、`icons/`、`ai_config.json` 等）。

## 启动流程（`server.go` → `main`）

1. **日志**：调用 `openLogFile()`，按顺序尝试创建/追加日志文件；成功则 `log` 写入文件，失败则退回到标准错误输出。
2. **创建服务**：`NewServer()` 初始化客户端表、工厂表与 stdin 的 `bufio.Reader`。
3. **加载输入法**：`loadInputMethods(server)`（见下一节）。
4. **主循环**：`Run()` 逐行读取 stdin，空行跳过；每行交给 `handleMessage`，出错时仍尽量对解析出的 `client_id` 回复 `success: false`，避免宿主阻塞。

运行时数据目录使用 **`Moqi`** 作为应用文件夹名（例如 `%LOCALAPPDATA%\Moqi\Logs`、`%APPDATA%\Moqi\Rime`），与仓库目录名 `moqi-ime` 相互独立。

## 输入法如何被加载（`loadInputMethods`）

运行时以 **`os.Executable()` 所在目录** 为根，读取 `input_methods/` 下的**子目录**：

1. 对每个子目录，尝试读取 `<子目录>/ime.json`；读不到或 JSON 无效则跳过。
2. 从 `ime.json` 取出 `guid`（转小写）、`name` 等；**没有 `guid` 则跳过该目录**。
3. 调用 `RegisterService(guid, factory)`，把「语言配置 GUID」映射到具体的 `TextService` 工厂。

**重要**：**选用哪段 Go 实现不是由 `ime.json` 里的 `moduleName` 等字段动态决定的**，而是由 **`loadInputMethods` 里对子目录名的 `switch entry.Name()`** 决定：

| 子目录名 `entry.Name()` | 注册的实现 |
|-------------------------|------------|
| `meow` | `input_methods/meow` |
| `rime` | `input_methods/rime` |
| `simple_pinyin` | `input_methods/simple_pinyin` |
| `fcitx5` | `input_methods/fcitx5` |
| **其他任意目录名** | **默认使用 `simple_pinyin` 的实现**（仍使用该机读到的 `guid`） |

因此：新增输入法时，若需要独立实现，必须在 `server.go` 中 **import 新包** 并在上述 `switch` 中 **增加对应目录名分支**；仅放 `ime.json` 而不会匹配到已有 `case` 时，会落到默认拼音实现。

## 通信协议格式

- **传输**：一行一条消息，文本编码与宿主一致；内容为 **JSON** 负载，外层带前缀字段。
- **请求行（宿主 → 本进程）**：

  ```text
  <client_id>|<JSON>
  ```

  其中 `<client_id>` 为宿主分配的连接标识；`<JSON>` 反序列化为 `imecore.Request`（见 `protocol.go`）。

- **响应行（本进程 → 宿主）**：前缀固定为代码中的常量 **`MOQI_MSG`**（`imecore.MsgMOQI`），格式为：

  ```text
  MOQI_MSG|<client_id>|<JSON>
  ```

  末尾换行由 `fmt.Printf` 输出；**所有正常响应均带 `MOQI_MSG` 前缀**，与旧式仅 `<client_id>|...` 的写法不同，请以当前代码为准。

## 请求与响应语义概要

### `method` 路由（`handleRequest`）

- **`init`**：根据请求中的 **输入法 GUID** 创建会话。GUID 优先取顶层 **`id`** 字符串（与宿主约定一致）；若为空则从 **`data.guid`** 读取。在 `factories` 中查找工厂，调用 `TextService.Init`；失败则从 `clients` 中移除。
- **`close`**：调用 `Service.Close()` 并删除客户端。
- **`onActivate`、`onDeactivate`、`filterKeyDown`、`onKeyDown`、`filterKeyUp`、`onKeyUp`、`onCommand`、`onMenu`、`onCompositionTerminated`、`onPreservedKey`、`onLangProfileActivated`**：转发到已初始化客户端的 `HandleRequest`，再把 `*imecore.Response` 转为 map 发送。
- **其他 `method`**：返回 `success: false` 与错误说明。

### 按键相关返回值

各输入法在 `HandleRequest` 中设置 `Response` 的 `ReturnValue`（或 `ReturnData`）。`convertResponse` 会写入 JSON 的 **`return`** 字段（整数或宿主约定的类型）：**`0` 表示未吞掉按键，`非 0`（通常为 `1`）表示已处理**。组合串、候选、上屏等通过 `compositionString`、`candidateList`、`commitString`、`showCandidates`、`setSelKeys` 等字段返回，与 `imecore.Response` 一致。

### 请求中的常用字段

除 `method`、`seqNum` 外，按键类请求常带 `keyCode`、`charCode`、`scanCode`、`repeatCount`、`keyStates` 等；`keyStates` 支持布尔数组或整数数组两种 JSON 形式（见 `protocol.go` 的 `UnmarshalJSON`）。`compositionString`、`candidateList` 等由宿主在需要时传入。

## 如何新增一种输入法（检查清单）

1. 在 `input_methods/<目录名>/` 下添加 **`ime.json`**，至少包含 **`guid`**（与 Windows 侧语言配置文件一致，会转小写匹配）。
2. 实现 `imecore.TextService`：`Init`、`HandleRequest`、`Close`（可嵌入 `TextServiceBase` 再覆盖分支）。
3. 在 **`server.go` 顶部 import** 新包。
4. 在 **`loadInputMethods` 的 `switch entry.Name()`** 中为 **`<目录名>`** 增加 `case`，调用 `server.RegisterService(guid, factory)`。
5. 将 **`input_methods/<目录名>/`** 与编译出的 **`server.exe`** 放在同一部署根目录下（见下节）。

若只加目录和 `ime.json` 而不改 `switch`，GUID 会注册为 **默认拼音** 实现。

## 编译与运行假设

- **Go 版本**：见 `go.mod`（当前为 Go 1.21+）。
- **编译示例**（在仓库根目录 `moqi-ime/`）：

  ```bash
  go build -o server.exe .
  ```

- **目录布局**：`server.exe` 与 **`input_methods/`** 须位于**同一目录**（因为 `loadInputMethods` 与 Rime 等均用 `filepath.Dir(os.Executable())` 拼接路径）。
- **宿主配置**：Windows 侧 `backends.json` 为**顶层 JSON 数组**（不是 `{ "backends": [...] }`）。与 `moqi-im-windows` 配套时示例为：

  ```json
  [
    {
      "name": "moqi-ime",
      "command": "moqi-ime\\server.exe",
      "workingDir": "moqi-ime",
      "params": ""
    }
  ]
  ```

  其中 `command` / `workingDir` 相对于宿主安装根目录，需与实际文件夹名一致。

## 日志路径（`openLogFile`）

按顺序尝试，**第一个成功打开的文件** 作为日志输出：

1. `%LOCALAPPDATA%\Moqi\Logs\moqi-ime.log`
2. `%TEMP%\Moqi\moqi-ime.log`
3. 当前工作目录下的 `moqi-ime.log`

目录不存在时会尝试 `MkdirAll`。若全部失败，日志打到 **stderr**。

## 配置与 Rime 数据路径

- **Rime 共享数据**：`<exeDir>\input_methods\rime\data`
- **Rime 用户目录**：`%APPDATA%\Moqi\Rime`（Rime 实现中 `APP == "Moqi"`）
- **打开日志目录菜单项**：`%LOCALAPPDATA%\Moqi\Logs`

## Rime：AI 候选（写好评等）

`input_methods/rime` 在 **`New(client)`** 时加载 AI 配置；若配置合法且能构造 HTTP 客户端，则根据热键在 **`filterKeyDown` / `filterKeyUp` / `onKeyDown` / `onKeyUp`** 中与 Rime 按键逻辑协同处理（见 `rime.go` 中 `handleAI*`、`triggerAIReview` 等）。

### 配置文件位置与复制行为

1. 启动时会尝试把**内置**配置复制到用户目录（仅当用户文件尚不存在）：  
   源：`<exeDir>\input_methods\rime\ai_config.json`  
   目标：`%APPDATA%\Moqi\Rime\ai_config.json`
2. 加载顺序：**先用户路径，再内置路径**；若都读不到有效配置，再尝试**纯环境变量**拼出一套 API 配置（见下）。

### `ai_config.json` 示例

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

- **`api`**：`base_url`、`api_key`、`model` 必填（可从环境变量补全，见 `fillAIAPIConfigFromEnv`）。
- **`actions`**：可配置多个动作；热键解析支持 `Ctrl` / `Alt` / `Shift` 与主键 **A–Z、0–9**（如 `Ctrl+Shift+G`、`Ctrl+Alt+1`）。若缺省 `actions`，会使用内置默认动作（默认热键 **Ctrl+Shift+G**，名称「写好评」）。
- **`prompt` 占位符**（`applyAIPromptPlaceholders`）：`{{previous_commit}}`、`{{composition}}`、`{{raw_input}}`、`{{candidate_1}}`～`{{candidate_3}}`、`{{first_candidate}}`～`{{third_candidate}}`、`{{candidates_top3}}` 等；若模板中**未出现任何占位符**，会在提示词后附加一段固定上下文（上一句 / 原始输入 / 前三个候选）。

### 环境变量（无配置文件或字段留空时）

- `MOQI_AI_BASE_URL`：OpenAI 兼容 API 根 URL（尾部 `/` 会被规整）
- `MOQI_AI_API_KEY`
- `MOQI_AI_MODEL`  

三者齐全时，即使无 `ai_config.json` 也可启用 AI；此时动作列表为默认的一条。

### HTTP 与结果处理（`ai_client.go`）

- 使用 **OpenAI 兼容的 Chat Completions** JSON API；单次请求超时 **`aiRequestTimeout` = 20 秒**（同步调用，请求在触发键处理路径内完成）。
- 模型回复会解析为**最多 3 条**候选（去重、去编号前缀、单行过长截断等，见 `normalizeAICandidates`）。
- **输入来源**：当前 Rime **组合串**与**当前候选列表前若干项**（及可选的「上一句」提交记忆），**不读取宿主应用内选中文本**。
- **失败或空结果**：记录日志并 `resetAIState`，**不强行上屏**，保留原有 Rime 状态。
- **交互**：AI 激活后可用数字键、`↑`/`↓`、回车、空格选择与提交；`Esc` 退出 AI 候选模式（具体按键处理见 `isAIHandledKey` 与 `handleAIKeyDown` 等）。
