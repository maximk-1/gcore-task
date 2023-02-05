#!/usr/bin/env bash

GOOS=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case $arch in
 x86_64)
   GOARCH=amd64
  ;;
 arm64)
   GOARCH=arm64
  ;;
esac

docker run --rm \
  -v "$(pwd)":/app -w /app \
  -e "GOARCH=$GOARCH" -e "GOOS=$GOOS" \
  golang:1.19 go build -v -o httpserver httpserver.go
