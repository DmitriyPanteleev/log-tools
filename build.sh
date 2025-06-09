#!/bin/bash

set -e

VERSION=$(cat VERSION)
GIT_COMMIT=$(git rev-parse --short HEAD)
LDFLAGS="-X 'main.Version=${VERSION}' -X 'main.GitCommit=${GIT_COMMIT}'"

GOOS=linux  GOARCH=amd64 go build -ldflags="$LDFLAGS" -o log-tools-linux-amd64
GOOS=linux  GOARCH=arm64 go build -ldflags="$LDFLAGS" -o log-tools-linux-arm64
GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o log-tools-macos-arm64

echo "Сборка завершена! Версия: $VERSION, Коммит: $GIT_COMMIT"

