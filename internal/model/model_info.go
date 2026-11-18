package model

// ModelInfo 描述 Whisper 模型的校验信息与本机安装、下载状态。
type ModelInfo struct {
	// ID 模型目录标识。
	ID string `json:"id"`
	// Bytes 预期模型文件大小，单位字节。
	Bytes int64 `json:"bytes"`
	// SHA256 预期模型文件的 SHA-256 十六进制摘要。
	SHA256 string `json:"sha256"`
	// Installed 是否已有可用的本机模型文件。
	Installed bool `json:"installed"`
	// Downloading 是否存在正在执行的下载任务。
	Downloading bool `json:"downloading"`
	// Downloaded 当前已下载字节数。
	Downloaded int64 `json:"downloaded"`
}
