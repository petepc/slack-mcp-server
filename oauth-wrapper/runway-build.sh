#!/bin/bash
# Build script for Runway deployment

set -e

echo "Building OAuth wrapper for Runway..."

# Ensure we're in the oauth-wrapper directory
cd "$(dirname "$0")"

# Initialize go module if needed
if [ ! -f "go.mod" ]; then
    go mod init github.com/korotovsky/slack-mcp-server/oauth-wrapper
fi

# Download dependencies
go mod tidy

# Build the binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o oauth-wrapper main.go

echo "OAuth wrapper build complete"