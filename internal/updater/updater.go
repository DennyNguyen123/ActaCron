package updater

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Body        string        `json:"body"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

// ReleaseInfo represents the update check result.
type ReleaseInfo struct {
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	TagName        string    `json:"tag_name"`
	Notes          string    `json:"notes"`
	PublishedAt    time.Time `json:"published_at"`
	AssetURL       string    `json:"asset_url"`
	AssetName      string    `json:"asset_name"`
	HasUpdate      bool      `json:"has_update"`
}

// Client handles update queries and installation.
type Client struct {
	httpClient *http.Client
	baseURL    string // empty defaults to https://api.github.com
}

// NewClient returns a new updater Client.
func NewClient(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{httpClient: client}
}

// CheckForUpdate queries the GitHub repository for the latest release.
func (c *Client) CheckForUpdate(repo string, currentVer string) (*ReleaseInfo, error) {
	apiURL := c.baseURL
	if apiURL == "" {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create update request: %w", err)
	}

	uaVer := currentVer
	if uaVer == "" {
		uaVer = "1.0.0"
	}
	req.Header.Set("User-Agent", fmt.Sprintf("ActaCron-Updater/%s (%s; %s)", uaVer, runtime.GOOS, runtime.GOARCH))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update check network request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &ReleaseInfo{
			CurrentVersion: currentVer,
			HasUpdate:      false,
			Notes:          "No releases found",
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error (%d): %s", resp.StatusCode, string(body))
	}

	var ghRel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&ghRel); err != nil {
		return nil, fmt.Errorf("failed to decode release response: %w", err)
	}

	hasNewer := IsNewerVersion(currentVer, ghRel.TagName)
	assetURL, assetName := findMatchingAsset(ghRel.Assets)

	return &ReleaseInfo{
		CurrentVersion: currentVer,
		LatestVersion:  strings.TrimPrefix(ghRel.TagName, "v"),
		TagName:        ghRel.TagName,
		Notes:          ghRel.Body,
		PublishedAt:    ghRel.PublishedAt,
		AssetURL:       assetURL,
		AssetName:      assetName,
		HasUpdate:      hasNewer,
	}, nil
}

func findMatchingAsset(assets []githubAsset) (string, string) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// 1. Exact match for OS and Arch
	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		if strings.Contains(lower, goos) && strings.Contains(lower, goarch) {
			return a.BrowserDownloadURL, a.Name
		}
	}

	// 2. Fallback to OS alone if single arch
	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		if strings.Contains(lower, goos) {
			return a.BrowserDownloadURL, a.Name
		}
	}

	return "", ""
}

// ApplyUpdate downloads the asset and performs in-place binary swap.
func (c *Client) ApplyUpdate(downloadURL string, currentExePath string) error {
	if downloadURL == "" {
		return fmt.Errorf("empty download URL")
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "ActaCron-Updater")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with HTTP %d", resp.StatusCode)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read download content: %w", err)
	}

	// If zip, extract the binary
	var newBinary []byte
	if isZip(payload) {
		bin, err := extractBinaryFromZip(payload)
		if err != nil {
			return fmt.Errorf("failed to extract binary from zip: %w", err)
		}
		newBinary = bin
	} else {
		newBinary = payload
	}

	tempDir := filepath.Dir(currentExePath)
	newExeTemp := filepath.Join(tempDir, fmt.Sprintf("actacron-update-%d.tmp", time.Now().UnixNano()))

	if err := os.WriteFile(newExeTemp, newBinary, 0755); err != nil {
		return fmt.Errorf("failed to write temporary binary: %w", err)
	}

	oldExe := currentExePath + ".old"
	// Remove any previous .old file
	_ = os.Remove(oldExe)

	// Step 1: Rename currentExe to currentExe.old (Windows allows renaming running exe)
	if err := os.Rename(currentExePath, oldExe); err != nil {
		_ = os.Remove(newExeTemp)
		return fmt.Errorf("failed to backup current executable: %w", err)
	}

	// Step 2: Rename newExeTemp to currentExePath
	if err := os.Rename(newExeTemp, currentExePath); err != nil {
		// Rollback if possible
		_ = os.Rename(oldExe, currentExePath)
		_ = os.Remove(newExeTemp)
		return fmt.Errorf("failed to swap executable: %w", err)
	}

	return nil
}

func isZip(data []byte) bool {
	return len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04
}

func extractBinaryFromZip(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		// Look for actacron or actacron.exe
		if strings.HasSuffix(name, "actacron.exe") || strings.HasSuffix(name, "actacron") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}

	return nil, fmt.Errorf("no actacron binary found in zip archive")
}

// CleanupOldBinary removes the .old backup file if present.
func CleanupOldBinary(exePath string) {
	oldFile := exePath + ".old"
	if _, err := os.Stat(oldFile); err == nil {
		_ = os.Remove(oldFile)
	}
}
