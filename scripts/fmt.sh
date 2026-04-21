#!/usr/bin/env bash
set -euo pipefail

gofmt -w $(find . -type f -name '*.go' -not -path './vendor/*')
