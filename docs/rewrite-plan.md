# Cue 的 Wails 重写方案（已确认）

调研及确认日期：2026-11-09。用户已确认开始实施，进度见 `tasks.md`；本文不代表功能已经完成。

## 一、已确认的事实

- 当前项目目录为空，没有现有源码、配置、Git 仓库或需要兼容的本地数据。
- 功能基准为 [Blueturboguy07/cue](https://github.com/Blueturboguy07/cue/tree/1971534255a5df55ad55f129ed1ae1c2503ac246)，固定提交 `1971534255a5df55ad55f129ed1ae1c2503ac246`，包版本 `0.2.14`。
- 上游使用 Electron、原生 HTML/CSS/JavaScript；主要功能分散在 `main.js`、`renderer/` 和 `src/`。
- 上游许可证为 GPL-3.0-or-later。重写中沿用的提示词、模型目录及其他内容保留来源与许可证，随项目提供许可文件和第三方说明。
- 本机为 macOS arm64，已安装 Go 1.25.5、Node.js 23.11.0、npm 10.9.2、Xcode Command Line Tools，尚未安装 Wails CLI。

## 二、推荐方案

采用 **Wails v3.0.0 + Go + TypeScript + Vite**，固定依赖版本。保留原版玻璃质感悬浮面板、工具条与转录侧栏，将界面和使用文档中文化。

Wails 官方已将 v3.0.0 标为稳定版；已核实该版本直接提供透明置顶窗口、鼠标穿透、录屏排除、全局快捷键和 macOS 辅助面板等能力，可减少原生窗口适配代码。该版本要求 Go 1.26.0，并声明 `go1.26.10` 工具链。实施时使用项目指定的 Go 工具链，不替换系统现有 Go 安装。

前端负责界面、输入和展示；Go 负责模型请求、流式响应、会话生命周期、持久化、模型文件管理和平台能力。麦克风、系统音频、截图、全局快捷键及窗口保护按操作系统适配，不依赖 Electron，也不把 WKWebView 当作 Chromium 使用。

macOS 与 Windows 都作为完整桌面版本交付，覆盖主要功能，并提供各自的构建入口；Windows 目标为 Windows 10 2004+ / Windows 11 x64，包含可运行的桌面构建产物。当前环境先验证 macOS；Windows 编译验证与真机验证分别记录。Linux 保留独立适配边界，其系统音频、全局快捷键与窗口行为受桌面环境影响；未验证能力在 README 中明确标注。

## 三、功能覆盖范围

| 模块 | 本次拟实现行为 | 上游依据 |
| --- | --- | --- |
| 悬浮窗 | 透明、无边框、置顶、拖动、位置记忆、折叠/展开、20%–100% 透明度、空白区域鼠标穿透、跨桌面显示 | `main.js`、`renderer/renderer.js` |
| 智能辅助 | 截图与近期对话共同提供建议 | `src/prompts.js` 的 `assist` |
| 接话建议 | 根据双方对话生成下一句建议 | `src/prompts.js` 的 `say` |
| 会话回顾 | 根据当前转录生成回顾，无转录时提示用户 | `src/prompts.js` 的 `recap` |
| 自由提问 | 输入问题，结合屏幕与对话回答 | `src/prompts.js` 的 `ask` |
| 指定问题回答 | 对方转录自动填入输入框，发送后针对该问题回答；保护用户手动编辑 | `answerThis`、`renderer/renderer.js` |
| 屏幕解题 | 仅使用截图，输出思路、代码与时间/空间复杂度 | `src/prompts.js` 的 `leetcode` |
| 回答展示 | 流式 Markdown、代码块、思考状态、滚动控制、错误反馈、超时与请求取消 | `src/llm.js`、`main.js`、`renderer/renderer.js` |
| AI 提供商 | OpenAI、Anthropic、Gemini、Azure、Cerebras、DeepSeek、Groq、MiniMax、Ollama、自定义 OpenAI 兼容端点 | `src/llm.js` |
| 第三方中转站 | 自定义 Base URL、API Key、Fast/Smart 模型名；支持 OpenAI 兼容 Chat Completions 请求，以及普通 JSON 和 SSE 流式响应 | 用户补充要求 |
| 模型与风格 | Fast/Smart 切换、模型 ID 自定义、回答风格规则；编码模式独立提示词 | `src/store.js`、`src/profile-context.js` |
| 双通道采集 | 麦克风代表“我”，系统音频代表“对方”；分别采集、转写和展示 | `main.js`、`src/stt*.js` |
| 云端转写 | 独立选择转写服务；迁移 OpenAI、Deepgram、Gemini、Groq 与自定义兼容端点的现有实现路径 | `src/stt.js`、`src/stt-streaming.js` |
| 本地转写 | whisper.cpp 常驻进程、双通道串行推理、语音活动检测；停止时排空尾句；失败时不自动上传云端 | `src/local-whisper-transcriber.js`、`src/whisper-server-session.js` |
| 本地模型管理 | 上游 30 个模型的目录、大小/哈希校验、下载、进度、取消、续传、导入、删除 | `src/whisper-model-catalog.js`、`src/whisper-model-manager.js` |
| 会议记忆 | 保存转录、30 分钟内恢复、会后摘要/要点/决策/行动项/跟进、最近 3 场摘要作为背景、最多保留 50 场 | `src/meeting-memory.js`、`src/meetings.js` |
| 幻灯片跟踪 | 用户主动开启后监听屏幕变化、生成页面描述；仅保存内存记录，不持久化截图 | `src/slides.js`、`main.js` |
| 本地助手连接 | App Link 的状态/诊断、开始/停止听写、幻灯片描述访问；调用方授权与撤销、幻灯片单独授权 | `src/applink.js`、`vendor/app-link/` |
| 配置与引导 | 提供商、音频、风格、外观设置；首次引导、权限状态与设置入口；配置和会议文件可靠写入 | `renderer/index.html`、`src/settings-store-core.js` |

保留源码实际注册的五个全局快捷键：

| macOS | Windows | 功能 |
| --- | --- | --- |
| Cmd+Enter | Ctrl+Enter | 接话建议 |
| Cmd+Shift+Enter | Ctrl+Shift+Enter | 智能辅助 |
| Cmd+H | Ctrl+H | 屏幕解题 |
| Cmd+Shift+/ | Ctrl+Shift+/ | 折叠/展开 |
| Cmd+Shift+X | Ctrl+Shift+X | 退出 |

窗口内保留发送、换行、设置、关闭、恢复上一条转录问题等交互。全局注册失败必须显示原因，不能只实现窗口内键盘监听后宣称全局快捷键可用。

### 第三方中转站的兼容范围

- 通过设置配置中转站地址、API Key 和模型名，不将请求固定发送到官方地址。
- 首先支持常见 OpenAI 兼容 `/chat/completions` 协议，正确处理 Base URL 路径拼接，支持文本和截图请求；截图能力仍取决于中转站及所选模型是否支持图片。
- 提供流式开关，分别处理 SSE 增量响应与普通 JSON 响应；处理结束标记、空响应、取消和超时。
- 清楚显示鉴权失败、余额不足、限流、模型不存在、接口不兼容等返回错误；日志不输出 API Key。
- 聊天和语音转写分别配置地址、Key 与模型。转写通过中转站提供的 OpenAI 兼容 `/audio/transcriptions` 接口调用，不能因为聊天可用就假设它支持转写。
- 用本地模拟中转服务验证请求路径、鉴权头、请求体、流式/非流式响应及错误处理。真实中转站连通性在具备用户配置后验证。
- 非 OpenAI 兼容的私有协议需要按该服务文档增加适配，不能承诺任意中转站天然兼容。

## 四、平台实现与验收边界

| 能力 | macOS 方案 | Windows 方案 |
| --- | --- | --- |
| 悬浮窗口 | Wails 窗口选项与 NSPanel、跨 Spaces 配置 | Wails/WebView2 窗口选项 |
| 屏幕截图 | 原生屏幕采集；检查录屏权限，缩放与编码后交给模型 | 原生桌面采集与截图编码 |
| 麦克风 | 系统音频采集接口，正确处理权限与设备生命周期 | 系统音频采集接口，检查隐私设置与设备状态 |
| 系统音频 | ScreenCaptureKit；默认关闭，显式开启后采集，保留系统权限提示 | WASAPI loopback，独立于麦克风通道 |
| 全局快捷键 | Wails `GlobalShortcutManager`，处理注册冲突 | Wails `GlobalShortcutManager`，处理注册冲突 |
| 录屏排除 | Wails `ContentProtectionEnabled`；属于尽力而为，不能保证现代录屏工具全部排除 | Wails 内容保护接口；补充系统版本及远程会话检测，失败可见 |

macOS 会议音频的支持版本将依据实际采用的原生 API 与实测确定；上游 Electron 路径要求 macOS 14.4+，不能直接把这个限制当作 Wails 的已验证结论。录屏排除也不能用“绝对隐身”表述。

本机可验证 macOS 构建与运行。Windows/Linux 配置、编译或 CI 检查与真机音频/权限验证分别记录；未进行的真机验证不标成通过。需要真实服务凭据的 AI 请求与麦克风/录屏权限，按实际可用条件验收。

## 五、拟与原版保持的明确差异

1. 应用名称保持 Cue，不沿用上游将名称伪装成 Microsoft Edge Update 的实现。
2. 默认使用用户自己的 API Key 或本地模型，不接入原作者的 publik 账户开通、余额与充值流程。源码中的发布 token 本来为空；若需要商业接入，应使用本项目自己的服务配置。
3. 移除向原作者服务发送的匿名使用遥测。
4. 将现有可操作界面和 README 中文化；回答语言提供明确配置，不照搬原版强制英语提示词。
5. 简历、岗位、STAR 故事、PDF/DOCX 导入在上游当前版本只有后端残留，没有可操作 UI；本次不把“新增简历管理页”当作现有功能迁移要求。
6. 保留真实可用的幻灯片跟踪与 App Link，不把它们误归为未使用代码。App Link 使用本项目独立应用标识，授权语义保持一致。

## 六、实现结构

保持少量清晰的模块，不引入通用插件框架或服务器部署：

```text
桌面入口与 Wails 绑定
├── 助手：模式、提示词、上下文、提供商与流式请求
├── 听写：双路采集、转写、会话状态与尾句处理
├── 本地模型：运行时、下载校验、推理进程管理
├── 存储：设置、会议、恢复与摘要
├── 平台适配：窗口、屏幕、音频、快捷键、权限
├── 本地联动：App Link 与授权
└── 前端：悬浮面板、输入、转录、设置、引导
```

配置和会议数据存放在本应用独立数据目录，使用受限权限与原子替换。模型密钥和请求编排留在 Go 侧；前端通过绑定方法发起操作，通过事件接收字幕、回答和下载进度。

## 七、完成标准与文档

- 有可运行的 Wails 桌面应用、完整源代码、固定依赖与构建入口；核心能力不能用静态演示或空实现代替。
- 核心协议与生命周期测试覆盖：流式中断/错误、提供商请求格式、会话恢复、双通道边界、尾句处理、模型校验/取消/续传。
- 前端通过类型检查与生产构建，桌面应用通过本机启动和主要交互验证。
- 中文 `readme.md` 包含项目简介、功能与原版差异、环境依赖、开发运行、各平台构建、提供商配置、本地 Whisper、数据位置、快捷键、权限、常见问题、验证记录和许可证。
- README 纠正上游“macOS 无会议音频”“转录仅在内存”“Custom 不能转写”等过时或矛盾说明，只描述最终实际实现。
- 提供许可证与来源说明。不会在未配置真实账户时宣称所有在线提供商已做真实连通测试。

## 八、审核点

已根据用户补充要求，将 Windows 完整桌面版本和第三方中转站 API Key 调用纳入方案。

已确认并实施：按上述合并范围，以 Wails v3 实现 macOS/Windows 版本与第三方中转站兼容，保留原版真实可用的本地能力，采用第五节列出的明确差异。

方案审核后已完成任务拆分与编码，实际验证及平台限制见 `verification.md`。最终 macOS 发行要求为 13.3+（包含所用 Whisper Accelerate 依赖）；Linux 本次未交付完整采集实现。前端提供可选个人背景文本，未增加 PDF/DOCX 导入。

## 参考资料

- [Cue 固定版本源码](https://github.com/Blueturboguy07/cue/tree/1971534255a5df55ad55f129ed1ae1c2503ac246)
- [Cue 许可证](https://github.com/Blueturboguy07/cue/blob/1971534255a5df55ad55f129ed1ae1c2503ac246/LICENSE)
- [Wails v3.0.0 稳定版说明](https://github.com/wailsapp/wails/blob/v3.0.0/README.md)
- [Wails v3.0.0 Go 工具链要求](https://github.com/wailsapp/wails/blob/v3.0.0/v3/go.mod)
- [Wails v3.0.0 窗口能力](https://github.com/wailsapp/wails/blob/v3.0.0/v3/pkg/application/webview_window_options.go)
- [Wails v3.0.0 全局快捷键](https://github.com/wailsapp/wails/blob/v3.0.0/v3/pkg/application/global_shortcut_manager.go)
- [Wails v3 官方文档](https://v3.wails.io/)
