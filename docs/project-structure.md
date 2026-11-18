# 无卡密版项目结构

维护分支：`codex/no-license`。无卡密业务基线为 `6bd618a`，分层参考为 `main` 的 `a416506`。本次按相同职责拆分 Go 包和 TypeScript 模块，保留无卡密版的问答、听写、纪要与桌面交互流程。

## 分层与依赖

| 目录 | 职责 |
| --- | --- |
| `main.go`、`assets.go` | 启动应用、嵌入前端资源 |
| `internal/desktop` | 装配依赖，管理 Wails 窗口、快捷键、鼠标跟踪、文件对话框与退出 |
| `internal/controller` | 显式提供前端绑定方法，协调业务调用与桌面能力 |
| `internal/service` | 管理问答、听写、页面记录、会议纪要及后台任务生命周期 |
| `internal/repository` | 读写本机 JSON，处理原子替换、备份恢复、配置校验与密钥脱敏 |
| `internal/model` | 定义设置、会议、问答、事件、状态和窗口布局的数据结构 |
| `internal/config` | 解析 `CUE_DATA_DIR` 或默认用户配置目录 |
| `internal/logging` | 管理诊断文件，归一化可安全记录的服务商和错误类别 |

前端经 `api/wails.ts` 调用 `controller.App`，业务由 `service.Runtime` 编排，存储由 `repository.Store` 管理。控制层通过 `Desktop` 接口请求窗口或文件选择，具体实现由 `desktop.Host` 装配。业务层不依赖 Wails。

`Runtime` 统一持有业务锁、上下文、取消函数和后台任务；拆分文件不会创建多份运行状态。窗口鼠标跟踪由 `Host` 管理。提供商协议、音频会话、模型推理和本地助手连接沿用已有独立模块。

## 目录

```text
main.go / assets.go
internal/
  desktop/       application.go、window.go、overlay.go、mouse.go、shortcuts.go、protection_*.go
  controller/    app.go、chat.go、listening.go、meeting.go、models.go、settings.go、window.go、applink.go、diagnostics.go
  service/       runtime.go、chat.go、listening.go、meeting.go、notes.go、slides.go、models.go、settings.go、events.go、applink.go、diagnostics.go
  repository/    store.go、settings.go、meeting.go、json.go、recovery.go、replace_*.go
  model/         settings.go、meeting.go、chat.go、status.go、event.go、model_info.go、overlay.go
  config/        app.go
  logging/       logger.go、diagnostics.go
  assistant/     提示词、转录上下文与页面指纹
  provider/      聊天与语音提供商协议
  listening/     音频采集会话和转写队列
  platform/      原生音频、截图和鼠标坐标
  sherpa/        中文流式识别
  whisper/       可选 Whisper 模型和推理进程
  textnorm/      简体转换
  applink/       本地助手连接协议
frontend/src/
  main.ts        连接样式、DOM、事件与启动流程
  types.ts       前后端数据类型
  api/           wails.ts、preview.ts
  app/           状态、外壳、启动、事件、布局与界面诊断
  components/    弹窗、确认框、提示和图标
  features/      问答、工具条、转录、会议、模型、页面、指南、本地助手权限与设置
  styles/        index.css 与按区域拆分的样式
  utils/         DOM、格式化与 Markdown
frontend/tests/  界面与工具条回归
docs/            结构、验证与手工测试说明
```

测试紧邻对应的 Go 包；`build/bin`、`frontend/dist`、依赖和语音模型属于生成内容，不随源码提交。

## 无卡密版维护约定

- 本分支没有卡密激活、试用、授权心跳、设备凭据存储或功能门禁；启动后直接显示工具条，配置 AI 服务后即可使用。
- AI API Key 是调用模型所需的凭据；本地助手权限控制其他本机程序访问 Cue。这两项功能继续保留。
- 从 `main` 同步修复时，检查是否依赖卡密模块；只迁移适用于无卡密版的改动，避免直接合并整个分支带回激活流程。
- 新增前端功能放入 `features`，通用界面放入 `components`；新增业务进入 `service`，本地读写进入 `repository`，不把逻辑重新堆入入口文件。

## 兼容与调用路径

本机数据目录、`settings.json`、`meetings.json` 及其 JSON 字段保持原有格式，无需迁移。启动时仍补齐已结束会议的缺失纪要；任务取消沿用应用上下文及各任务取消函数。

Wails 绑定类型从根包 `App` 移到 `controller.App`，前端完整调用前缀改为 `cue-go-remix/internal/controller.App.`。前后端必须配套构建，不能只更新旧安装包中的一侧。界面预览继续使用 `?preview=1`。

诊断仍写入数据目录下的 `logs/cue.log`，保留操作类别、结果和耗时，不记录 API Key、服务地址或对话正文。验证结果见 [验证记录](verification.md)，桌面与音频验收见 [手工测试](manual-test.md)。
