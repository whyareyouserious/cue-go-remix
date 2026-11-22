# Cue 软件图标

原创矢量图标使用深紫色圆角底、浅色 C 与薄荷色声波，呼应语音助手和现有界面。设计说明：大轮廓保证小尺寸辨识度，四角透明，不在图内放置小字。

- `cue.svg`：可编辑源文件，1024 × 1024。
- `cue.png`：透明 PNG，嵌入桌面窗口与关于面板。
- `cue.icns`：macOS 应用图标，包含 16 至 1024 像素图像。
- `cue.ico`：Windows 图标文件，包含 16、24、32、48、64、128、256 像素图像；构建时随 EXE 提供，可供快捷方式选用。

macOS 构建会复制 `cue.icns` 到应用资源目录，并通过 `CFBundleIconFile` 声明。Windows 窗口使用嵌入的 PNG，当前构建未给 EXE 注入资源图标。

修改 SVG 后，在安装好前端依赖的 macOS 上运行 `node scripts/prepare-icon.mjs`。脚本默认使用已安装的 Google Chrome；可通过 `CUE_TEST_BROWSER` 指定其他 Chromium 浏览器路径。生成过程只做本机渲染和格式导出，不调用 AI 生图 API。正常构建直接使用已导出的文件。
