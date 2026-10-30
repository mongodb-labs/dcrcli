#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "Testing portable remote-copy argument construction"
go test ./fscopy ./sshctl ./webui

echo "Cross-building Windows amd64 binary"
out="$(mktemp -d)/dcr-windows-amd64.exe"
trap 'rm -rf "$(dirname "$out")"' EXIT
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o "$out" .

echo "Windows compatibility check passed: $out"
