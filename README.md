# portal-down-gui

A desktop GUI for [portal-down](https://github.com/traditionalism/portal-down) — download your granted assets from the [Cfx.re portal](https://portal.cfx.re/) without touching a terminal.

Built with [Wails](https://wails.io/) (Go backend + React/TypeScript frontend).

## Download

Grab the latest release for your platform from the [Releases](../../releases) page:

| Platform | File |
|---|---|
| Windows (64-bit) | `portal-down-gui-windows-amd64.zip` |
| macOS (Apple Silicon) | `portal-down-gui-macos-arm64.zip` |
| macOS (Intel) | `portal-down-gui-macos-amd64.zip` |
| Linux (64-bit) | `portal-down-gui-linux-amd64.tar.gz` |

> **macOS:** After unzipping, right-click the `.app` → Open the first time (Gatekeeper blocks unsigned apps on double-click).

## Getting your forum token

1. Go to [forum.cfx.re](https://forum.cfx.re/) and log in.
2. Open Browser DevTools → **Application** → **Cookies** → find the cookie named `_t` and copy its value.

## Usage

1. Paste your forum token into the **Connection** panel and click **Connect**.  
   The app authenticates and loads your asset list automatically.
2. Use the search box to filter assets by name or ID.
3. Check the assets you want and click **Download Selected**, or click **Download all**.
4. Downloaded `.zip` files land in the folder shown under **Output** (default: `~/Downloads/portal-down`). Click the folder icon to pick a different location.
5. Optionally enable **Upload to Discord** and paste a webhook URL to mirror each download to a Discord channel. Files over 25 MB post a size notice instead.

Settings (token, folder, webhook) are saved automatically when you click **Save settings** or **Connect**.

## Building from source

### Prerequisites

- [Go 1.21+](https://golang.org/dl/)
- [Node.js 18+](https://nodejs.org/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Development

```bash
wails dev
```

### Production build (current platform)

```bash
wails build
```

### Cross-platform builds (via scripts)

```bash
# All platforms (requires the appropriate OS / cross-compile toolchain)
bash scripts/build-all.sh

# Individual platforms
bash scripts/build-windows.sh
bash scripts/build-macos-arm.sh
bash scripts/build-macos-intel.sh
bash scripts/build-linux.sh
```
