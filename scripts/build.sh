#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

mkdir -p "$HOME/.local/bin" "$ROOT_DIR/dist"

echo "==> Linux (CachyOS) için derleniyor: ~/.local/bin/radar"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$HOME/.local/bin/radar" .

echo "==> Windows 10 (64-bit) için derleniyor: dist/radar.exe"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$ROOT_DIR/dist/radar.exe" .

ls -lh "$HOME/.local/bin/radar" "$ROOT_DIR/dist/radar.exe"
echo "Tamamlandı!"
