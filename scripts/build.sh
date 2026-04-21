#!/usr/bin/env bash
set -euo pipefail

mkdir -p dist
go build -o dist/grind ./cmd/grind
