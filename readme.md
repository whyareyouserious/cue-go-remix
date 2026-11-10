# Cue · Wails 中文重写版

一个悬浮在桌面上的实时语音助手：分别转写麦克风和系统声音，将对方的问题填入输入框，按 Enter 即可请求大模型并逐步显示回答。也支持接话建议、屏幕问答、代码解题和会议纪要。

参考 [Blueturboguy07/cue](https://github.com/Blueturboguy07/cue)，使用 **Go + Wails v3 + TypeScript** 重写。支持 **macOS 与 Windows**，可使用自己的 AI API Key、第三方中转站，或本地中文流式识别；也保留 Whisper 分段转写。无需 Electron，也没有本项目运营的云端服务。

> 当前版本为 `0.1.0`。macOS 已验证紧凑界面、第三方选择、密钥框聚焦和设置保存重开，Windows x64 已交叉编译。Windows 真机音频与录屏排除、真实云服务连通性尚未验证。详细记录见 [验证说明](docs/verification.md)。

人工测试步骤见 [手工测试说明](docs/manual-test.md)。应用在本地 `logs/cue.log` 记录操作类别、保存结果和退出事件，不记录 API Key、接口地址、问题或转录正文。

## 界面与使用流程

界面采用无外投影的磨砂玻璃风格，由 Gemini 设计前端视觉，Codex 检查功能与桌面集成。启动时默认显示悬浮工具条；点击“显示”展开回答卡片，点击“转录历史”查看发言记录。在面板边缘及紧邻的透明空白处即可拖动，无须对准拖动按钮。透明宿主窗口随内容调整。

1. 点击工具条齿轮进入“设置 → 密钥”，通过提供商按钮组选择服务，填写 Base URL、API Key、快速模型和深度思考模型，再保存设置。
2. 在“设置 → 音频”选择“本地实时（中文）”，无需语音 API Key；也可配置独立的云端语音服务或 Whisper 分段转写。需要转写对方声音时，开启“采集系统音频”。
3. 点击“开始会话”，再点击“显示”展开回答区。双方的暂定字幕会随说话持续更新，停顿后形成历史记录；对方的暂定文字即会填入输入框，最终文字替换同一句。手工输入时不会被新的转录覆盖。
4. 按 Enter 或发送按钮请求回答；回答按流式内容逐步显示。**转录不会每句自动触发大模型请求**，也可使用“我该怎么说？”或“智能辅助”及对应快捷键。

原项目作为功能与交互参考，前端视觉独立设计，不要求一比一还原。

## 功能

| 功能 | 行为 |
| --- | --- |
| 接话建议 | 从最近 16 轮双方转录中生成可直接说出口的回复 |
| 智能辅助 | 结合当前屏幕与最近 14 轮对话给出建议 |
| 自由提问 | 输入问题，结合屏幕与最近 12 轮对话回答 |
| 指定问题回答 | 自动将对方的新问题填入输入框，按 Enter 请求回答；手工编辑后不会继续被转录覆盖 |
| 屏幕解题 | 根据截图给出思路、代码与复杂度，不混入个人背景或会议摘要 |
| 回顾 | 回顾当前会话的转录，最多使用最近 200 轮；没有转录时不请求模型 |
| 多模型 | Fast / Smart 模型切换、流式 Markdown、代码复制、取消与超时处理 |
| 双通道转写 | 麦克风标为“我”，系统声音标为“对方”，独立处理音频 |
| 本地中文实时 | Sherpa-ONNX 中文流式识别，模型随包，双方独立字幕；中间与最终文本统一为简体 |
| 本地 Whisper（可选） | 分段识别；30 个模型、下载/续传/取消/导入/删除、大小与 SHA-256 校验 |
| 会议记忆 | 本地保存转录、30 分钟内恢复、纪要生成、历史查看、Markdown 导出与删除 |
| 页面记录 | 主动开启后识别屏幕变化并生成页面描述，最多保留 50 条内存记录 |
| 桌面悬浮 | 透明置顶、拖动与位置记忆、折叠、透明边缘穿透、手动全窗穿透、全局快捷键 |
| 本地助手连接 | Unix socket / Windows named pipe，按调用方授权读取状态、控制听写和读取页面描述 |

## 直接使用构建产物

本地构建输出：

- macOS Apple Silicon：`build/bin/Cue.app`，打包文件 `build/bin/Cue-macos-arm64.zip`。
- Windows x64：`build/bin/windows-amd64/Cue.exe`，打包文件 `build/bin/Cue-windows-amd64.zip`。

Windows 请解压整个压缩包，保留 `Cue.exe` 旁的 `sherpa/` 目录，以及使用可选 Whisper 时的 `whisper/` 目录。首次运行需要 [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)。中文流式运行时和模型随构建包提供；Whisper 模型不预装，需进入“设置 → 音频 → Whisper 本地模型管理”按需下载。

macOS 将 `Cue.app` 放到固定目录后打开。当前产物使用本机临时签名，尚未经过 Apple 公证；Windows 产物也未做商业代码签名。重新构建 macOS 应用可能需要重新授予权限。

## 开发环境

| 依赖 | 要求 |
| --- | --- |
| Go | 项目声明 `go 1.26.0`，固定工具链 `go1.26.10`；启用自动工具链下载时会按项目获取 |
| Node.js | 推荐 22.12+；前端依赖由 `package-lock.json` 固定 |
| macOS | 13.3+，Xcode Command Line Tools；构建 Whisper 另需 CMake |
| Windows | Windows 10 2004+ / Windows 11 x64，WebView2 Runtime |

无需额外安装 Wails CLI；项目直接调用 Wails v3 库与 Go 构建。

```bash
# 在项目根目录执行
npm ci --prefix frontend
go mod download

# 准备当前平台的中文流式运行时与模型
node scripts/prepare-sherpa.mjs

# 可选：需要 Whisper 分段转写时，另外准备其运行时
go run ./cmd/prepare-whisper

# 构建当前平台
node scripts/build.mjs

# 可选：生成便于分发的 zip
node scripts/package.mjs
```

macOS 的音频组件由 `native/macos/AudioCapture.swift` 编译，构建脚本自动放入应用的 `Contents/Helpers/`。macOS 构建同时启用 Wails 的 `private_mac_apis` 标签，使窗口空白区域透明；这是应用编译选项，不修改系统设置。Windows 采集使用 Go 调用 WASAPI / GDI，无需安装额外录音程序。

### 构建 Windows 版

可以在 Windows 本机或 macOS 上构建 Windows x64：

```bash
node scripts/prepare-sherpa.mjs windows amd64
# 可选 Whisper 运行时
go run ./cmd/prepare-whisper -platform windows -arch amd64
node scripts/build.mjs windows amd64
node scripts/package.mjs windows amd64
```

Windows 代码无需 CGO，构建脚本设置 `CGO_ENABLED=0`。交叉编译只能证明代码可生成 EXE，不能替代 Windows 真机设备和权限测试。

### 构建 macOS 版

```bash
# Apple Silicon
node scripts/prepare-sherpa.mjs darwin arm64
# 可选 Whisper 运行时
go run ./cmd/prepare-whisper -platform darwin -arch arm64
node scripts/build.mjs darwin arm64

# Intel：在 macOS 上构建，需要对应架构的系统工具链
node scripts/prepare-sherpa.mjs darwin amd64
# 可选 Whisper 运行时
go run ./cmd/prepare-whisper -platform darwin -arch amd64
node scripts/build.mjs darwin amd64
```

目前交付与实测的是 Apple Silicon 构建。Intel 路径已提供，但未做 Intel 真机验证。

### 修改代码与测试

```bash
npm run build --prefix frontend
# macOS 开发运行也需要窗口透明支持；Windows 可直接 go run .
go run -tags private_mac_apis .

go test -race ./internal/...
go test ./...
go vet ./...
npm run build --prefix frontend

# 浏览器界面测试；首次使用需安装测试浏览器
cd frontend
npx playwright install chromium
npm run test:ui
```

开发时原生权限身份可能与 `.app` 不同，测试音频推荐使用打包应用。单独运行前端开发服务器后加 `?preview=1` 可查看带明显标识的界面预览；预览不连接桌面后端、不采集音频，也不会假装完成 AI 请求。

## 配置 AI 与第三方中转站

打开“设置 → 密钥”，点击提供商按钮，填写服务地址、API Key、快速模型和深度思考模型。支持 OpenAI、Anthropic、Gemini、Azure、Cerebras、DeepSeek、Groq、MiniMax、Ollama 和自定义端点。

### OpenAI 兼容中转站

点击“第三方中转”，按服务商文档填写：

```text
Base URL：https://relay.example.com/v1
API Key：中转站发放的密钥
快速模型：该站支持的模型 ID
深度思考模型：该站支持的另一个模型 ID
流式回答：支持 SSE 时开启，否则关闭
```

实现会向 `/chat/completions` 发送请求。Base URL 已包含完整 `/chat/completions` 时不会重复拼接；自定义路径前缀和查询参数会保留。密钥通过 `Authorization: Bearer ...` 发送，只发送到配置的服务。

支持普通 JSON `choices[].message.content` 和 SSE `choices[].delta.content` / `[DONE]`。图片通过 OpenAI 兼容的 `image_url` 内容块发送。中转站和模型必须支持图片，屏幕问答才能正常工作。

模型 ID 由中转站决定，可能与官方名称不同。遇到 401、403、404、429、余额不足或接口返回非 JSON 时，界面会显示对应错误；不会偷偷切换到其他提供商。端点更换后不会自动把旧密钥发送到新域名，需重新填写。

**兼容边界：**支持 OpenAI Chat Completions 协议；Anthropic、Gemini 可使用对应协议和自定义地址。仅提供 OpenAI Responses API 或非标准私有协议的中转站，需要增加适配，不能直接当作兼容 Chat Completions 使用。Azure 使用兼容的 `/openai/v1` 路径，模型名填写部署名。

本地 Ollama 可使用 `http://127.0.0.1:11434/v1`；无鉴权的本地兼容端点可不填 Key。使用屏幕功能需选择支持视觉的模型。

### 聊天与语音分开配置

云端语音在“设置 → 音频”拥有独立的地址、API Key 和转写模型；本地中文实时模式无需这些凭据。聊天中转可用，并不表示它提供语音接口。

| 转写方式 | 协议与说明 |
| --- | --- |
| OpenAI / Groq / 自定义 | 分段 WAV 上传至 `/audio/transcriptions`；例如 `whisper-1`，以服务商实际支持为准 |
| Deepgram | 支持 WAV 分段请求与实时 WebSocket |
| Gemini | 支持音频内容分段请求与实时 WebSocket |
| OpenAI 实时 | 使用 Realtime 转写协议，需要支持该协议的服务与转写模型 |
| 本地实时（中文） | Sherpa-ONNX 连续处理音频，实时更新暂定字幕，停顿后提交整句；无需地址或密钥 |
| Whisper（分段） | whisper.cpp，等待分段后识别；语音不发送云端，不自动回退到云端 |

对于常见中转站，建议先使用**分段转写**；聊天的流式开关与实时语音不是同一个能力。云端实时模式对 OpenAI、Deepgram、Gemini 提供，网关必须支持对应 WebSocket 协议。本地中文实时模式直接使用随包模型，与中转站无关。

## 本地中文实时转写

1. 使用已带模型的构建包，或先执行 `node scripts/prepare-sherpa.mjs` 再构建应用。也可明确指定目标：`node scripts/prepare-sherpa.mjs darwin arm64`、`darwin amd64` 或 `windows amd64`。
2. 在“设置 → 音频”选择“本地实时（中文）”并保存；不需要填写 Base URL、API Key 或模型名。既有用户配置不会被默认设置自动覆盖。
3. 点击“开始会话”→“显示”。麦克风和系统声音分别显示“我 / 对方 · 正在说”的暂定字幕，允许随上下文修正，停顿后保存最终文字。采集系统声音需要主动打开对应开关并授予平台权限。
4. “对方”的暂定文字立即更新提问框，最终结果替换同一句；不会重复追加，也不会覆盖手工编辑。仍须按 Enter 才请求大模型，发送后同一句后续修正不会重新填回输入框。

使用 Sherpa-ONNX `2.0.1` 和中文 Zipformer int8 `2025-06-30` 模型，模型文件合计约 167 MB。准备脚本按固定大小和 SHA-256 校验官方归档及模型文件。macOS 放在应用 `Contents/Resources/sherpa/`，Windows 放在 `Cue.exe` 旁的 `sherpa/`；请保留其中的运行库和 `model/` 文件。

音频仅在本机内存处理，不上传给转写服务。中文暂定和最终结果统一转换为简体；简繁转换不等于纠正错词。当前模型面向中文，其他语言可选择云端服务或可选 Whisper。本机公开中文样本已验证连续出字与双路处理，但真实环境的首字延迟和准确率仍取决于设备、收音与噪声，不能承诺每个字都即时准确识别。

## 本地 Whisper（可选分段转写）

1. 先运行 `go run ./cmd/prepare-whisper`，或使用已经包含运行时的构建包。
2. 在“设置 → 音频”的语音转写服务中点击“Whisper（分段）”。
3. 点击“Whisper 本地模型管理”下载模型；中文可从多语言 `base` 或 `small` 开始，名称带 `.en` 的模型仅适用于英语。
4. 如使用自行编译的运行时，在设置中填写 `whisper-server` / `whisper-server.exe` 的绝对路径。

模型下载允许取消与续传，完成后校验固定大小与 SHA-256。导入也需要匹配目录中同名模型的哈希；损坏文件不会被作为可用模型加载。模型使用中不能删除或替换。

监听期间只启动一个本地推理进程，两路语音串行推理。服务仅监听 `127.0.0.1` 的临时端口，使用随机请求路径。停止会话后排空已有队列，最多等待 15 秒，再释放进程。捕获的音频仅在内存中处理，不写临时 WAV。

运行时下载固定为 whisper.cpp `1.9.1`。Windows 使用校验过的官方二进制，macOS 使用校验过的固定源码本机编译。若上游重新压缩源码导致校验不符，脚本会失败，需要人工核实新归档，不能直接跳过校验。

## 权限与平台差异

| 能力 | macOS | Windows |
| --- | --- | --- |
| 屏幕问答 | 需要屏幕录制权限 | 原生 GDI 截图 |
| 麦克风 | 需要麦克风权限，提示可能显示 Cue / Cue 音频采集 | 需允许桌面应用访问麦克风 |
| 系统音频 | ScreenCaptureKit；默认关闭，设置中主动开启；系统可能显示录屏指示 | WASAPI loopback；默认开启 |
| 录屏排除 | Wails 内容保护，尽力而为，现代捕获方式可能仍看到窗口 | Windows 10 2004+ 的本地会话；远程桌面等环境关闭保护 |
| 全局快捷键 | Cmd 组合；注册冲突会提示 | Ctrl 组合；注册冲突会提示 |

macOS：系统设置 → 隐私与安全性 → 麦克风 / 屏幕录制。授权后可能需要重启应用。Windows：设置 → 隐私和安全性 → 麦克风 → 允许桌面应用访问。

录屏排除无法保证所有软件、系统版本和外部摄像头都看不到窗口。Linux 的桌面采集和完整发行包本次未实现；相关调用会明确报不支持。

## 快捷键

Windows 使用 Ctrl，macOS 使用 Cmd：

| 快捷键 | 功能 |
| --- | --- |
| Cmd/Ctrl + Enter | 接话建议 |
| Cmd/Ctrl + Shift + Enter | 智能辅助 |
| Cmd/Ctrl + H | 屏幕解题 |
| Cmd/Ctrl + Shift + / | 折叠 / 展开 |
| Cmd/Ctrl + Shift + I | 退出鼠标穿透，恢复窗口交互 |
| Cmd/Ctrl + Shift + X | 退出应用 |
| Cmd/Ctrl + , | 在窗口内打开设置 |
| Enter / Shift + Enter | 发送 / 换行 |

恢复穿透的快捷键若注册失败，不能开启手动全窗穿透。默认对面板及其周边拖动区域之外的空白穿透，鼠标进入面板或拖动区域后恢复交互。

## 会议与数据

数据位于：

- macOS：`~/Library/Application Support/cue-go-remix/`
- Windows：`%APPDATA%\cue-go-remix\`
- 开发/测试可用 `CUE_DATA_DIR` 指定独立目录；程序内的设置页显示实际路径。

| 文件或目录 | 内容 |
| --- | --- |
| `settings.json` / `.bak` | 配置、密钥、背景资料、窗口位置 |
| `meetings.json` / `.bak` | 转录、会议纪要与恢复信息 |
| `models/` | 下载的 Whisper 模型及 `.part` 续传文件 |
| `applink/` | 本地连接发现文件与调用方授权 |

配置与会议文件使用临时文件替换、备份和受限文件权限。API Key 目前为本机文件明文存储，不是系统钥匙串；普通配置读取只向界面返回“已设置”状态。窗口关闭后密钥不会上传给本项目服务。

每条最终转录到达后尝试保存；写入失败会明确提示，内存保留已接收内容。最多保留 50 场会议。停止听写会生成或刷新纪要，但保留当前会议便于继续；“结束并清空当前转录”会结束当前会议，历史仍可查看。30 分钟无新发言也会结束会议。重新打开时恢复 30 分钟内未结束的会议。

至少 4 条转录才自动生成纪要。生成需要调用聊天模型，可能产生服务费用；失败会提示，下次启动补生成。最近 3 场历史纪要用于辅助后续问题，当前对话优先。删除会议后旧版本可能仍存在于 `.bak` 文件中；备份不是加密或不可恢复删除。

屏幕仅在屏幕类问答或主动开启页面跟踪时采集。macOS 截图使用短暂临时图片，读取后删除；页面描述仅在内存中保留，清空会话或退出后移除。云端转写把音频发送至所选转写服务；本地转写不上传音频，但会议纪要和回答仍会发送相关文本至聊天服务。

## 本地助手连接

应用身份为 `com.cue.remix`。`applink/run.json` 给出当前进程的 socket / named pipe 和随机会话令牌，协议使用逐行 JSON-RPC 2.0。

先 `connect` 验证令牌并请求权限，再调用 `get_state`、`capture_diagnostics` 等状态接口或 `invoke_action`。只读状态返回计数，不包含转录与密钥；`set_capturing` 控制听写，`get_slides` 读取页面描述，分别需要 action / slides 授权。授权可在设置中撤销，撤销会立即断开旧连接。

令牌验证的是当前操作系统用户边界，不是调用方软件签名。授权弹窗会明确提示名称由请求方自行声明。详细协议和测试可参考 `internal/applink/`。

## 与原版的差异

- 桌面容器从 Electron 改为 Wails；音频使用原生平台接口。
- 中文界面，回答语言可选，增加可直接配置的个人背景文本。
- 磨砂玻璃前端视觉，启动默认显示工具条，回答卡与转录历史按需展开，透明窗口随实际内容调整。
- 采用 Cue 真实名称；不伪装成系统更新程序。
- 不接入原作者的 publik 充值/余额流程，不发送匿名使用遥测。
- 自定义中转的聊天和语音独立配置；支持明确选择非流式响应。
- 原版当前界面未提供的简历 PDF/DOCX 导入、STAR 表单，本次没有新增。
- App Link 使用本项目独立身份与发现目录，不自动注册到原作者的商业目录。

## 目录

```text
main.go / app.go       Wails 桌面入口、绑定与应用编排
frontend/             中文 TypeScript 界面与浏览器交互测试
internal/provider/    AI 与语音提供商、中转兼容协议
internal/listening/   采集生命周期、分段与转写队列
internal/platform/    macOS / Windows 原生截图与音频
internal/whisper/     本地模型与推理进程
internal/store/       本地持久化、备份恢复
internal/assistant/   功能提示词与页面指纹
internal/applink/     本地授权连接
native/macos/         Swift 音频组件
cmd/prepare-whisper/  固定运行时准备工具
scripts/build.mjs     双平台构建和打包
```

## 常见问题

**能聊天但不能听写？** 聊天和转写是独立接口。检查“设置 → 音频”里的地址、Key、模型，以及是否支持 `/audio/transcriptions` 或所选实时协议。

**为什么启动只有一条工具栏？** 这是默认的紧凑模式。点击“显示”展开回答区，点击“转录历史”查看记录；无需常驻大窗口。

**为什么转录后没有自动回答？** 实时转录会把对方的问题填入输入框。按 Enter、点击发送，或使用接话建议/智能辅助快捷键后才调用模型，不会对每句话自动发起请求。

**中转站返回模型不存在？** 使用站点实际公布的模型名；Azure 填部署名。支持的模型可能变化，默认值只是初始配置。

**模型支持文字，但截屏报错？** 该模型或中转不支持图片。换用视觉模型，或使用只依赖转录的接话建议/回顾。

**macOS 听不到对方？** 在“设置 → 音频”开启“采集系统音频”，授予屏幕录制权限后重启。只有麦克风权限无法采集系统输出。

**本地模型已下载但加载失败？** 检查运行时路径、模型语言和可用内存；先尝试 tiny/base。模型损坏会明确报校验失败。

**窗口无法点击？** 按 Cmd/Ctrl+Shift+I。若窗口被折叠，按 Cmd/Ctrl+Shift+/ 展开；重复启动应用也会显示已有窗口。

**录屏仍看到 Cue？** 这是平台能力限制。macOS 特别是新捕获接口无法保证排除；Windows 远程桌面环境会主动关闭该功能。

## 许可证与来源

本项目采用 **GPL-3.0-or-later**，见 [LICENSE](LICENSE)。功能与模型目录参考 Cue 固定提交 `1971534255a5df55ad55f129ed1ae1c2503ac246`。Wails、whisper.cpp 与其他依赖保留各自许可证，见 [第三方说明](THIRD_PARTY_NOTICES.md)。
