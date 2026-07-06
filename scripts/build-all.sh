#!/bin/bash
# Build script for all platforms

echo "Building for all platforms..."
echo "================================"

echo "Building for Windows (amd64)..."
wails build -platform windows/amd64 -clean -webview2 embed

echo "Building for Linux (amd64)..."
wails build -platform linux/amd64 -clean

echo "================================"
echo "Build complete! Check build/bin/ directory"
