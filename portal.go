package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type SSOResponseBody struct {
	URL string `json:"url"`
}

type Pack struct {
	ID int `json:"id"`
}

type Version struct {
	ID            int    `json:"id"`
	VersionString string `json:"version"`
	Packs         []Pack `json:"packs"`
}

type Asset struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Versions []Version `json:"versions"`
}

type AssetGrant struct {
	GrantID int   `json:"grant_id"`
	Asset   Asset `json:"asset"`
}

type grantsResponse struct {
	Grants []AssetGrant `json:"grants"`
}

func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}
}

func portalAuthenticate(client *http.Client, forumToken string) error {
	if forumToken == "" {
		return fmt.Errorf("forum token is empty")
	}

	forumURL, _ := url.Parse("https://forum.cfx.re")
	client.Jar.(*cookiejar.Jar).SetCookies(forumURL, []*http.Cookie{
		{Name: "_t", Value: forumToken, HttpOnly: true, Secure: true},
	})

	resp, err := client.Get("https://portal-api.cfx.re/v1/auth/discourse?return=")
	if err != nil {
		return fmt.Errorf("SSO request failed: %w", err)
	}
	defer resp.Body.Close()

	var ssoBody SSOResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&ssoBody); err != nil {
		return fmt.Errorf("failed to parse SSO response: %w", err)
	}
	if ssoBody.URL == "" {
		return fmt.Errorf("SSO returned empty redirect URL — is your forum token valid?")
	}

	noRedirectClient := &http.Client{
		Jar: client.Jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	forumReq, _ := http.NewRequest("GET", ssoBody.URL, nil)
	forumResp, err := noRedirectClient.Do(forumReq)
	if err != nil {
		return fmt.Errorf("forum SSO request failed: %w", err)
	}
	forumResp.Body.Close()

	redirectTo := forumResp.Header.Get("Location")
	if redirectTo == "" {
		return fmt.Errorf("forum SSO did not redirect, status: %d", forumResp.StatusCode)
	}

	redirectURL, err := url.Parse(redirectTo)
	if err != nil {
		return fmt.Errorf("invalid redirect URL from forum: %w", err)
	}
	q := redirectURL.Query()
	sso := q.Get("sso")
	sig := q.Get("sig")
	if sso == "" || sig == "" {
		return fmt.Errorf("redirect URL missing sso/sig params: %s", redirectTo)
	}

	payload, _ := json.Marshal(map[string]string{"sso": sso, "sig": sig})
	callbackResp, err := client.Post(
		"https://portal-api.cfx.re/v1/auth/discourse",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("auth POST request failed: %w", err)
	}
	body, _ := io.ReadAll(callbackResp.Body)
	callbackResp.Body.Close()

	if callbackResp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth POST returned %d: %s", callbackResp.StatusCode, string(body))
	}

	return nil
}

func portalFetchAssets(client *http.Client, log func(string)) ([]AssetGrant, error) {
	var all []AssetGrant
	page := 1
	for {
		reqURL := fmt.Sprintf(
			"https://portal-api.cfx.re/v1/me/asset-grants?search=&sort=asset.name&direction=asc&page=%d&per_page=50",
			page,
		)
		resp, err := client.Get(reqURL)
		if err != nil {
			return nil, fmt.Errorf("asset list request failed (page %d): %w", page, err)
		}
		var result grantsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode asset list (page %d): %w", page, err)
		}
		resp.Body.Close()

		if len(result.Grants) == 0 {
			break
		}
		all = append(all, result.Grants...)
		if log != nil {
			log(fmt.Sprintf("fetched page %d (%d assets so far)", page, len(all)))
		}
		page++
	}
	return all, nil
}

func portalDownloadAsset(
	client *http.Client,
	grant AssetGrant,
	downloadDir string,
	useDiscord bool,
	discordWebhook string,
	log func(string),
) (string, int64, error) {
	asset := grant.Asset
	if len(asset.Versions) == 0 {
		return "", 0, fmt.Errorf("asset %s has no versions", asset.Name)
	}
	version := asset.Versions[0]
	if len(version.Packs) == 0 {
		return "", 0, fmt.Errorf("asset %s version %d has no packs", asset.Name, version.ID)
	}
	pack := version.Packs[0]

	downloadURL := fmt.Sprintf(
		"https://portal-api.cfx.re/v1/asset-grants/%d/versions/%d/packs/%d/download",
		grant.GrantID, version.ID, pack.ID,
	)
	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", 0, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	preview := make([]byte, 512)
	n, _ := resp.Body.Read(preview)

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("download returned %d: %s", resp.StatusCode, string(preview[:n]))
	}

	var downloadBody io.Reader
	if n > 0 && preview[0] == '{' {
		var redirectResp struct {
			URL string `json:"url"`
		}
		combined := io.MultiReader(bytes.NewReader(preview[:n]), resp.Body)
		if err := json.NewDecoder(combined).Decode(&redirectResp); err == nil && redirectResp.URL != "" {
			fileResp, err := client.Get(redirectResp.URL)
			if err != nil {
				return "", 0, fmt.Errorf("failed to fetch from redirect URL: %w", err)
			}
			defer fileResp.Body.Close()
			if fileResp.StatusCode != http.StatusOK {
				return "", 0, fmt.Errorf("redirect URL returned %d", fileResp.StatusCode)
			}
			downloadBody = fileResp.Body
		} else {
			downloadBody = io.MultiReader(bytes.NewReader(preview[:n]), resp.Body)
		}
	} else {
		downloadBody = io.MultiReader(bytes.NewReader(preview[:n]), resp.Body)
	}

	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create download dir: %w", err)
	}
	outPath := filepath.Join(downloadDir, asset.Name+".zip")
	f, err := os.Create(outPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create file %s: %w", outPath, err)
	}
	written, err := io.Copy(f, downloadBody)
	f.Close()
	if err != nil {
		return "", 0, fmt.Errorf("failed to write %s: %w", outPath, err)
	}

	if useDiscord && discordWebhook != "" {
		versionStr := version.VersionString
		if versionStr == "" {
			versionStr = strconv.Itoa(version.ID)
		}
		if log != nil {
			log(fmt.Sprintf("uploading %s to Discord...", asset.Name))
		}
		if err := portalSendDiscord(discordWebhook, asset.Name, versionStr, outPath); err != nil {
			if log != nil {
				log(fmt.Sprintf("discord upload failed: %v", err))
			}
		} else if log != nil {
			log(fmt.Sprintf("discord upload successful for %s", asset.Name))
		}
	}

	return outPath, written, nil
}

func portalSendDiscord(webhook, assetName, versionStr, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if stat.Size() > 25*1024*1024 {
		payload := map[string]string{
			"content": fmt.Sprintf(
				"**Asset Downloaded**\n**Name:** %s\n**Version:** %s\n*(File too large to upload (%.2f MB))*",
				assetName, versionStr, float64(stat.Size())/(1024*1024),
			),
		}
		jsonPayload, _ := json.Marshal(payload)
		resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	payload := map[string]string{
		"content": fmt.Sprintf("**Asset Downloaded!**\nName: %s\nInternal Version: %s", assetName, versionStr),
	}
	payloadBytes, _ := json.Marshal(payload)
	if err := writer.WriteField("payload_json", string(payloadBytes)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", webhook, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook post failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func formatBytes(b int64) string {
	const mb = 1024 * 1024
	if b >= mb {
		return strconv.FormatFloat(float64(b)/float64(mb), 'f', 2, 64) + " MB"
	}
	return strconv.FormatInt(b/1024, 10) + " KB"
}

func emitLog(ctx context.Context, line string) {
	if ctx == nil {
		return
	}
	wruntime.EventsEmit(ctx, "log", line)
}
