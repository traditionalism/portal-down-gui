#!/bin/bash
# Build for Windows (AMD64)

echo "Building for Windows (amd64)..."
# -webview2 embed bundles the WebView2 bootstrapper so the app installs the
# runtime (or shows a real error) instead of silently closing on machines
# that don't already have it (common on Windows 10 / fresh installs).
wails build -platform windows/amd64 -clean -webview2 embed
echo "Build complete! Check build/bin/"
