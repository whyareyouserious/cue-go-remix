#!/bin/bash
set -euo pipefail

task_root="$(cd "$(dirname "$0")/../.." && pwd)"
task_arch="${1:-$(uname -m)}"
task_output="${2:-$task_root/native/bin/cue-audio}"
case "$task_arch" in arm64|x86_64) ;; *) printf '不支持的架构：%s\n' "$task_arch" >&2; exit 1;; esac

mkdir -p "$(dirname "$task_output")"
swiftc -parse-as-library -O -target "$task_arch-apple-macosx13.0" \
  -module-cache-path "${TMPDIR:-/tmp}/cue-swift-cache" \
  "$task_root/native/macos/MicrophoneRecovery.swift" \
  "$task_root/native/macos/AudioCapture.swift" -o "$task_output" \
  -framework AVFoundation -framework ScreenCaptureKit -framework CoreMedia \
  -Xlinker -sectcreate -Xlinker __TEXT -Xlinker __info_plist \
  -Xlinker "$task_root/native/macos/Info.plist"
codesign --force --sign - "$task_output"
printf '已生成音频组件：%s\n' "$task_output"
