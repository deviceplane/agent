#!/bin/bash

set -e

if [ -z "$AGENT_VERSION" ]; then
    echo "AGENT_VERSION not set"
    exit 1
fi

mkdir -p ./dist/agent

OS_PLATFORM_ARG=(linux)

declare -a OS_ARCH_ARG
OS_ARCH_ARG[linux]="amd64 arm arm64 mipsle"

for OS in ${OS_PLATFORM_ARG[@]}; do
    for ARCH in ${OS_ARCH_ARG[${OS}]}; do
        OUTPUT_BIN="dist/agent/$AGENT_VERSION/$OS/$ARCH/deviceplane-agent"
        if test "$OS" = "windows"; then
            OUTPUT_BIN="${OUTPUT_BIN}.exe"
        fi
        echo "Building binary for $OS/$ARCH..."
        GOOS=$OS GOARCH=$ARCH CGO_ENABLED=0 go build \
              -mod vendor \
              -ldflags="-s -w -X main.version=$AGENT_VERSION" \
              -o ${OUTPUT_BIN} ./cmd/agent
    done
done
