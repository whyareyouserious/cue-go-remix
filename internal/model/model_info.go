package model

type ModelInfo struct {
	ID          string `json:"id"`
	Bytes       int64  `json:"bytes"`
	SHA256      string `json:"sha256"`
	Installed   bool   `json:"installed"`
	Downloading bool   `json:"downloading"`
	Downloaded  int64  `json:"downloaded"`
}
