package main

import "embed"

//go:embed all:frontend/dist
var frontend embed.FS

// appIcon 供桌面窗口与关于面板使用，与发行包中的图标保持一致。
//
//go:embed build/icons/cue.png
var appIcon []byte
