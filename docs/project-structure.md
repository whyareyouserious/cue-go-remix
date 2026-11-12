# 项目结构说明

本轮按照用户批准的方案，参考 `service-duan` 的控制层、业务层、持久化层、配置和数据模型分工整理 Cue。沿用 Go 包和 TypeScript 模块组织方式，没有增加只做转发的 `service/impl`、数据库或新的 UI 方案。

## 分层与依赖

| Java 项目的职责 | Cue 对应目录 | 负责内容 |
| --- | --- | --- |
| 启动配置与依赖装配 | `main.go`、`internal/desktop` | 创建业务运行时、绑定接口、创建 Wails 窗口 |
| Controller | `internal/controller` | 保持前端方法签名，协调业务调用与桌面文件对话框 |
| Service | `internal/service` | 问答、听写、会议、纪要和授权的任务生命周期 |
| Mapper / 数据访问 | `internal/repository` | 本地 JSON 读写、备份恢复、配置校验与密钥脱敏 |
| Entity / DTO | `internal/model` | 设置、会议、问答、事件、窗口布局等共享数据结构 |
| Config | `internal/config` | 数据目录、开发环境与正式版授权地址 |
| Logging | `internal/logging` | 日志文件管理与错误、服务商的固定分类 |

前端通过 `api/wails.ts` 调用 `controller.App`，业务由 `service.Runtime` 调度，数据由 `repository.Store` 管理。`service` 不依赖 Wails；`controller` 通过小型 `Desktop` 接口请求选文件和窗口操作，不导入 `desktop` 的具体实现。`desktop` 是组装点，持有业务运行时并把其事件转给前端。

`Runtime` 集中持有业务锁、上下文和取消函数；分文件不会创建多份共享状态。桌面鼠标跟踪由 `Host` 自己管理。`entitlement`、`provider`、`listening` 等已有独立模块继续处理各自的具体能力，避免把所有细节再塞回业务编排层。

## 当前目录

```text
cue-go-remix/
├── main.go                         # 应用启动
├── assets.go                       # 嵌入 frontend/dist
├── internal/
│   ├── desktop/
│   │   ├── application.go          # 依赖装配、系统事件与退出
│   │   ├── window.go               # 窗口创建、文件对话框、穿透
│   │   ├── overlay.go              # 窗口位置、尺寸与命中区域
│   │   ├── mouse.go                # 鼠标跟踪
│   │   ├── shortcuts.go            # 快捷键
│   │   └── protection_*.go         # 各平台录屏保护
│   ├── controller/
│   │   ├── app.go                  # App 与 Desktop 接口
│   │   ├── chat.go / listening.go
│   │   ├── meeting.go / models.go
│   │   ├── settings.go / license.go
│   │   └── window.go / applink.go / diagnostics.go
│   ├── service/
│   │   ├── runtime.go              # 共享状态、初始化、启动与关闭
│   │   ├── chat.go / listening.go  # 问答与语音会话
│   │   ├── meeting.go / notes.go   # 会议记录与纪要任务
│   │   ├── slides.go / models.go   # 页面跟踪、本地模型操作
│   │   ├── settings.go / license.go
│   │   └── events.go / applink.go / diagnostics.go
│   ├── repository/
│   │   ├── store.go                # Store 初始化、ID 与复制
│   │   ├── settings.go             # 配置读取、脱敏、校验与保存
│   │   ├── meeting.go              # 会议持久化
│   │   ├── json.go / recovery.go   # 原子写入、备份与恢复
│   │   └── replace_*.go            # 各平台文件替换
│   ├── model/
│   │   ├── settings.go / meeting.go / chat.go
│   │   └── status.go / event.go / model_info.go / overlay.go
│   ├── config/
│   │   ├── app.go                  # 数据目录
│   │   └── license*.go             # 构建注入及开发/正式环境区分
│   ├── logging/
│   │   └── logger.go / diagnostics.go
│   ├── entitlement/
│   │   ├── manager.go              # 授权状态持有者与构造
│   │   ├── activation.go           # 激活、试用、重试
│   │   ├── heartbeat.go / status.go
│   │   ├── credentials.go / lifecycle.go
│   │   ├── client.go               # 授权 HTTP 协议
│   │   └── platform/               # 设备标识与系统凭据库
│   ├── assistant/                  # 提示词、转录上下文与页面指纹
│   ├── provider/                   # 聊天及语音提供商协议
│   ├── listening/                  # 音频会话和队列
│   ├── platform/                   # 原生音频、截图、鼠标坐标
│   ├── sherpa/                     # 中文流式识别
│   ├── whisper/                    # 可选 Whisper 模型及推理进程
│   ├── textnorm/                   # 简体转换
│   └── applink/                    # 本地助手连接协议
├── frontend/
│   ├── src/
│   │   ├── main.ts                 # 仅连接样式、DOM、事件和启动流程
│   │   ├── types.ts / runtime.d.ts
│   │   ├── api/                    # wails.ts、preview.ts
│   │   ├── app/                    # 状态、DOM 外壳、启动、事件、布局
│   │   ├── components/             # 弹窗、确认框、提示、图标
│   │   ├── features/
│   │   │   ├── chat.ts / toolbar.ts / transcript.ts
│   │   │   ├── license.ts / meetings.ts / models.ts
│   │   │   ├── slides.ts / guide.ts / consent.ts
│   │   │   └── settings/
│   │   │       ├── index.ts        # 设置外壳、切页与保存
│   │   │       ├── form.ts         # 读取草稿表单
│   │   │       ├── endpoint.ts     # 聊天及语音服务配置
│   │   │       └── style.ts / appearance.ts / models.ts / access.ts
│   │   ├── styles/                 # index.css 与各区域样式
│   │   └── utils/                  # DOM、格式化、Markdown
│   └── tests/                      # 界面、菜单条、授权回归
├── native/macos/                   # Swift 音频组件
├── cmd/prepare-whisper/            # Whisper 运行时准备工具
├── scripts/                        # 构建和语音资源准备
├── build/                          # 平台资源与生成产物
└── docs/                           # 方案、结构、测试和验证说明
```

Go 测试放在对应包旁边；树中省略各个 `*_test.go`、前端静态资源及依赖配置文件。`build/bin`、`frontend/dist`、`node_modules` 和模型缓存为生成内容，不属于分层源码。

## 拆分前后对照

| 原文件 | 拆分后的主要位置 |
| --- | --- |
| 根 `app.go`（722 行） | `controller/*`、`service/*`、`desktop/*` |
| 根 `main.go`（71 行） | `main.go`、`assets.go`、`desktop/application.go`、`desktop/window.go` |
| 根 `overlay.go`、`protection_*.go` | `desktop/overlay.go`、`desktop/mouse.go`、`desktop/protection_*.go`、`model/overlay.go` |
| 根 `diagnostics.go` | `logging/*`、`service/diagnostics.go` |
| 根 `entitlement.go`、`entitlement_config*.go` | `service/license.go`、`controller/license.go`、`config/license*.go` |
| `internal/store/store.go`（352 行） | `repository/store.go`、`settings.go`、`meeting.go`、`json.go`、`recovery.go` |
| `internal/model/model.go` | `model` 下按业务命名的文件 |
| `internal/entitlement/manager.go`（523 行） | 64 行的 `manager.go`，以及激活、心跳、状态、凭据和生命周期文件 |
| `frontend/src/main.ts`（833 行） | 10 行启动入口，业务移至 `app`、`features`、`components`、`utils` |
| `frontend/src/style.css`（2101 行） | `styles/index.css` 按原顺序加载各界面区域样式 |
| `frontend/src/api.ts`、`icons.ts` | `api/wails.ts`、`api/preview.ts`、`components/icons.ts` |

前端共享状态统一由 `app/state.ts` 管理；业务模块读取同一状态，布局和公共弹窗不反向导入具体设置页，模块依赖检查无循环。设置子页用回调请求重绘，避免与设置外壳循环导入。

## 后续修改入口

| 要修改的功能 | 先看哪里 |
| --- | --- |
| 输入问题、按 Enter 回答、流式显示 | `features/chat.ts`、`service/chat.go`、`provider/chat.go` |
| 语音中间字幕与最终句 | `features/transcript.ts`、`service/listening.go`、`listening`、`sherpa` |
| 卡密激活界面与授权流程 | `features/license.ts`、`service/license.go`、`entitlement` |
| 设置页面和保存 | `features/settings`、`service/settings.go`、`repository/settings.go` |
| 窗口跳动、鼠标穿透、拖动区域 | `app/layout.ts`、`app/observe.ts`、`desktop/overlay.go`、`desktop/mouse.go` |
| 会议历史、导出和纪要 | `features/meetings.ts`、`controller/meeting.go`、`service/meeting.go`、`service/notes.go` |
| JSON 数据及恢复 | `repository`；数据结构在 `model` |
| 样式或公共弹窗 | 对应 `styles/*.css`、`components/modal.ts` |

新增前端可调用方法时，在 `controller.App` 明确声明。不要直接嵌入整个 `service.Runtime`，否则内部生命周期、原始设置等方法也会暴露成 Wails 接口。

## 构建与兼容

- 根目录保留 Go 入口和 `//go:embed all:frontend/dist`，避免移动至 `cmd/cue` 后无法跨目录嵌入资源；`go run .` 和现有双平台打包入口继续有效，具体环境参数见 [README 调试启动](../readme.md#调试启动)。
- Wails 调用名改为 `cue-go-remix/internal/controller.App.<方法名>`，已在 `api/wails.ts` 同步。方法参数、返回数据及事件名称保持兼容。
- 正式授权地址链接符号改为 `cue-go-remix/internal/config.LicenseBaseURL`，`scripts/build.mjs` 统一处理。自行编写构建命令时也需使用新符号。
- 本轮不迁移用户数据；`settings.json`、`meetings.json`、模型目录、系统凭据服务名和 JSON 字段保持原样。样式内容和覆盖顺序保持不变，核心交互仍是实时转录填入输入框后按 Enter 回答。
- 本轮验证与平台边界见 [验证记录](verification.md)，人工操作步骤见 [手工测试说明](manual-test.md)。
