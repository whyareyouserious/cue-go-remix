package main

import (
	"io/fs"
	"log"

	"cue-go-remix/internal/desktop"
)

func main() {
	assets, err := fs.Sub(frontend, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	if err := desktop.Run(assets); err != nil {
		log.Fatal(err)
	}
}
