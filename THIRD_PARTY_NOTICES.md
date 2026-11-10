# 第三方来源与许可证

本项目使用 GPL-3.0-or-later 发布。保留上游许可证不意味着上游作者为本重写版背书。

| 项目 | 用途 | 许可证 / 来源 |
| --- | --- | --- |
| Cue | 功能参考、Whisper 模型目录及固定运行时校验信息 | GPL-3.0-or-later；[固定提交](https://github.com/Blueturboguy07/cue/tree/1971534255a5df55ad55f129ed1ae1c2503ac246) |
| Wails v3.0.0 | Go 桌面框架与前端运行时 | MIT；[源码](https://github.com/wailsapp/wails/tree/v3.0.0) |
| whisper.cpp 1.9.1 | 本地语音推理运行时 | MIT；[源码](https://github.com/ggml-org/whisper.cpp/tree/v1.9.1)；完整通知见 `licenses/whisper.cpp.LICENSE` |
| Sherpa-ONNX 2.0.1 | 本地中文流式语音识别 | Apache-2.0；[固定版本](https://github.com/k2-fsa/sherpa-onnx/tree/v2.0.1)；完整许可见 `licenses/sherpa-onnx.LICENSE` |
| ONNX Runtime 1.28.2 | Sherpa 模型计算运行时 | MIT；[固定版本](https://github.com/microsoft/onnxruntime/tree/v1.28.2)；许可及依赖通知见 `licenses/onnxruntime.LICENSE`、`licenses/onnxruntime.ThirdPartyNotices.txt` |
| OpenCC 1.1.9 | 转录结果的繁体到简体归一化字典 | Apache-2.0；[固定版本](https://github.com/BYVoid/OpenCC/tree/ver.1.1.9)；字典来源和校验见 `internal/textnorm/data/README.md`，许可见 `licenses/OpenCC.LICENSE` |
| purego 0.9.1 | 无 CGO 调用本地语音动态库 | Apache-2.0；[固定版本](https://github.com/ebitengine/purego/tree/v0.9.1)；完整许可见 `licenses/purego.LICENSE` |
| coder/websocket | 实时语音 WebSocket | ISC；[源码](https://github.com/coder/websocket) |
| go-winio | Windows named pipe | MIT；[源码](https://github.com/microsoft/go-winio) |
| DOMPurify | Markdown HTML 清洗 | Apache-2.0 OR MPL-2.0；[源码](https://github.com/cure53/DOMPurify) |
| marked | Markdown 渲染 | MIT；[源码](https://github.com/markedjs/marked) |
| TypeScript / Vite / Playwright | 开发、构建、界面测试 | Apache-2.0 / MIT / Apache-2.0，版本见前端锁文件 |

Whisper 模型下载自 `ggerganov/whisper.cpp` 与 `akashmjn/tinydiarize-whisper.cpp` 的 Hugging Face 仓库。模型权重按对应仓库许可使用，不随源码预装。目录记录的字节数与 SHA-256 用于检测下载损坏或内容被替换。

本地中文流式模型固定为 `sherpa-onnx-streaming-zipformer-zh-int8-2025-06-30`，下载自 [Sherpa-ONNX 官方模型发行](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models)。归档说明其权重源自 [yuekai/icefall-asr-multi-zh-hans-zipformer-large](https://huggingface.co/yuekai/icefall-asr-multi-zh-hans-zipformer-large)，训练代码来自 Icefall。模型归档没有独立 LICENSE；不能把 Sherpa-ONNX 代码的 Apache-2.0 许可推断成权重许可。当前作为本地试用资源随本机构建放入 `sherpa/model/`，对外分发前须核实权重作者的许可条件。

运行时归档与模型归档的 SHA-256 使用官方 GitHub release asset 的 digest；固定来源、大小与各模型文件校验值见 `internal/sherpa/artifacts.json`。`scripts/prepare-sherpa.mjs` 校验后仅复制识别所需动态库、模型及来源说明，不携带 WebSocket 服务器或测试音频。

本项目中的 Go 调用层、平台适配、存储与前端为重写实现；不包含原作者 publik 发布 token、商业账户开通或匿名遥测。

Go 传递依赖完整版本见 `go.mod` 与 `go.sum`，前端依赖见 `frontend/package-lock.json`。发行包保留本文件、项目许可证和本地推理运行时许可证。
