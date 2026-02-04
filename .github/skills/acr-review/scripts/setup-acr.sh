#!/bin/bash
set -e

ACR_PATH="$HOME/.acr"
ACR_BIN="$ACR_PATH/acr"

# Skip if already built
if [ -f "$ACR_BIN" ]; then
    echo "ACR already installed at $ACR_BIN"
    exit 0
fi

# Check if Go is available
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. ACR cannot be built."
    echo ""
    echo "To install Go:"
    echo "1. Download from https://go.dev/dl/"
    echo "2. Install and ensure 'go' is in your PATH"
    echo "3. Run this script again"
    exit 1
fi

# Clone and build
echo "Cloning ACR repository..."
git clone https://github.com/richhaase/agentic-code-reviewer.git "$ACR_PATH"

echo "Building ACR..."
cd "$ACR_PATH"
go build -o acr ./cmd/acr

if [ -f "$ACR_BIN" ]; then
    echo "ACR built successfully at $ACR_BIN"
else
    echo "Error: Build failed - acr binary not created"
    exit 1
fi
