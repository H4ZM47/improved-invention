#!/usr/bin/env bash
set -euo pipefail

mkdir -p dist
go build -o dist/task ./cmd/task
