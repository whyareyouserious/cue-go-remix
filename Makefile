.PHONY: setup dev test build windows whisper

setup:
	cd frontend && npm ci
	go mod download

dev:
	cd frontend && npm run build
	go run .

test:
	go test -race ./internal/...
	cd frontend && npm run build

build:
	node scripts/build.mjs

windows:
	node scripts/build.mjs windows amd64

whisper:
	go run ./cmd/prepare-whisper
