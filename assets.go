package main

import "embed"

//go:embed all:frontend/dist
var frontend embed.FS
