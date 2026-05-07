#!/bin/bash
# Build script for all platforms

echo "Building for all platforms..."
echo "================================"

echo "Building for Windows (amd64)..."
wails build -platform windows/amd64 -clean -s

echo "Building for Linux (amd64)..."
wails build -platform linux/amd64 -clean -s

echo "================================"
echo "Build complete! Check build/bin/ directory"
