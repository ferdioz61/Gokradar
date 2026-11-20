#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

mkdir -p "$HOME/.local/bin" "$ROOT_DIR/dist"

echo "==> Linux (64-bit Intel/AMD) için derleniyor: ~/.local/bin/radar ve dist/radar-linux-64bit"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$HOME/.local/bin/radar" .
cp "$HOME/.local/bin/radar" "$ROOT_DIR/dist/radar-linux-64bit"

echo "==> Windows 10/11 (64-bit Intel/AMD) için derleniyor: dist/radar-windows-64bit.exe"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$ROOT_DIR/dist/radar-windows-64bit.exe" .

echo "==> Windows 10 (32-bit eski sistemler) için derleniyor: dist/radar-windows-32bit.exe"
CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags="-s -w" -o "$ROOT_DIR/dist/radar-windows-32bit.exe" .

ls -lh "$HOME/.local/bin/radar" "$ROOT_DIR/dist/radar-linux-64bit" "$ROOT_DIR/dist/radar-windows-64bit.exe" "$ROOT_DIR/dist/radar-windows-32bit.exe"
echo "Tamamlandı!"
