package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Config struct {
	ForumToken        string `json:"forumToken"`
	DiscordWebhookURL string `json:"discordWebhookUrl"`
	DownloadDir       string `json:"downloadDir"`
	UseDiscord        bool   `json:"useDiscord"`
}

type DownloadResult struct {
	AssetID int    `json:"assetId"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Error   string `json:"error,omitempty"`
}

type App struct {
	ctx    context.Context
	mu     sync.Mutex
	client *http.Client
	authed bool
}

func NewApp() *App {
	return &App{client: newHTTPClient()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "portal-down-gui")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "config.json"), nil
}

func (a *App) defaultDownloadDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Downloads", "portal-down")
	}
	return "assets"
}

func (a *App) LoadConfig() (Config, error) {
	cfg := Config{DownloadDir: a.defaultDownloadDir()}
	path, err := a.configPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = a.defaultDownloadDir()
	}
	return cfg, nil
}

func (a *App) SaveConfig(cfg Config) error {
	path, err := a.configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (a *App) PickDirectory() (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Select Download Folder",
	})
}

func (a *App) TestWebhook(url string) error {
	if url == "" {
		return fmt.Errorf("webhook URL is empty")
	}
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// Authenticate runs the SSO flow against forum.cfx.re using the supplied token.
func (a *App) Authenticate(forumToken string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.client = newHTTPClient()
	a.authed = false

	emitLog(a.ctx, "authenticating with forum.cfx.re...")
	if err := portalAuthenticate(a.client, forumToken); err != nil {
		emitLog(a.ctx, fmt.Sprintf("auth failed: %v", err))
		return err
	}
	a.authed = true
	emitLog(a.ctx, "authentication successful")
	return nil
}

func (a *App) FetchAssets() ([]AssetGrant, error) {
	a.mu.Lock()
	authed := a.authed
	client := a.client
	a.mu.Unlock()

	if !authed {
		return nil, fmt.Errorf("not authenticated — save your forum token and connect first")
	}
	emitLog(a.ctx, "fetching asset grants...")
	grants, err := portalFetchAssets(client, func(s string) { emitLog(a.ctx, s) })
	if err != nil {
		emitLog(a.ctx, fmt.Sprintf("fetch failed: %v", err))
		return nil, err
	}
	emitLog(a.ctx, fmt.Sprintf("found %d asset(s)", len(grants)))
	return grants, nil
}

func (a *App) DownloadAssets(grantIDs []int, useDiscord bool, downloadDir, discordWebhook string) ([]DownloadResult, error) {
	a.mu.Lock()
	authed := a.authed
	client := a.client
	a.mu.Unlock()

	if !authed {
		return nil, fmt.Errorf("not authenticated")
	}
	if downloadDir == "" {
		downloadDir = a.defaultDownloadDir()
	}
	if useDiscord && discordWebhook == "" {
		return nil, fmt.Errorf("discord upload enabled but webhook URL is empty")
	}

	grants, err := portalFetchAssets(client, nil)
	if err != nil {
		return nil, err
	}

	wanted := make(map[int]bool, len(grantIDs))
	for _, id := range grantIDs {
		wanted[id] = true
	}

	var queue []AssetGrant
	for _, g := range grants {
		if len(wanted) == 0 || wanted[g.GrantID] {
			queue = append(queue, g)
		}
	}

	results := make([]DownloadResult, 0, len(queue))
	total := len(queue)
	for i, g := range queue {
		emitLog(a.ctx, fmt.Sprintf("[%d/%d] downloading %s (id %d)...", i+1, total, g.Asset.Name, g.Asset.ID))

		path, size, err := portalDownloadAsset(client, g, downloadDir, useDiscord, discordWebhook, func(s string) { emitLog(a.ctx, s) })
		res := DownloadResult{AssetID: g.Asset.ID, Name: g.Asset.Name, Path: path, Size: size}
		if err != nil {
			res.Error = err.Error()
			emitLog(a.ctx, fmt.Sprintf("err: %v", err))
		} else {
			emitLog(a.ctx, fmt.Sprintf("downloaded %s (%s)", g.Asset.Name, formatBytes(size)))
		}
		results = append(results, res)
		wruntime.EventsEmit(a.ctx, "download:progress", map[string]any{
			"index":  i + 1,
			"total":  total,
			"result": res,
		})
	}

	emitLog(a.ctx, fmt.Sprintf("done — %d asset(s) processed", total))
	return results, nil
}
