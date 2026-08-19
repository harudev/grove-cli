package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const repoOwner = "harudev"
const repoName = "grove-cli"

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var checkUpdateCmd = &cobra.Command{
	Use:   "check-update",
	Short: "새 버전 확인",
	RunE:  runCheckUpdate,
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "최신 버전으로 업데이트",
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(checkUpdateCmd)
	rootCmd.AddCommand(updateCmd)
}

func runCheckUpdate(cmd *cobra.Command, args []string) error {
	latest, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("최신 버전 확인 실패: %w", err)
	}

	latestVer := strings.TrimPrefix(latest.TagName, "v")
	currentVer := strings.TrimPrefix(appVersion, "v")

	fmt.Fprintf(os.Stderr, "  현재 버전: %s\n", cyanStyle.Render(currentVer))
	fmt.Fprintf(os.Stderr, "  최신 버전: %s\n", cyanStyle.Render(latestVer))

	if currentVer == "dev" {
		fmt.Fprintf(os.Stderr, "\n  %s 개발 빌드입니다. grove update로 릴리스 버전을 설치하세요.\n", yellowStyle.Render("⚠"))
		return nil
	}

	if currentVer == latestVer {
		fmt.Fprintf(os.Stderr, "\n  %s 이미 최신 버전입니다.\n", "✅")
	} else {
		fmt.Fprintf(os.Stderr, "\n  %s 새 버전이 있습니다! grove update로 업데이트하세요.\n", yellowStyle.Render("⬆"))
	}
	return nil
}

func runUpdate(cmd *cobra.Command, args []string) error {
	latest, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("최신 버전 확인 실패: %w", err)
	}

	latestVer := strings.TrimPrefix(latest.TagName, "v")
	currentVer := strings.TrimPrefix(appVersion, "v")

	if currentVer != "dev" && currentVer == latestVer {
		fmt.Fprintf(os.Stderr, "%s 이미 최신 버전입니다 (%s)\n", "✅", latestVer)
		return nil
	}

	// Find matching asset
	assetName := findAssetName(latest)
	if assetName == "" {
		return fmt.Errorf("현재 플랫폼(%s/%s)에 맞는 릴리스를 찾을 수 없습니다", runtime.GOOS, runtime.GOARCH)
	}

	var downloadURL string
	for _, a := range latest.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("다운로드 URL을 찾을 수 없습니다: %s", assetName)
	}

	fmt.Fprintf(os.Stderr, "%s %s → %s 업데이트 중...\n", cyanStyle.Render("⬇"), currentVer, latestVer)

	// Find current binary path
	binPath, err := findBinaryPath()
	if err != nil {
		return err
	}

	// Download and extract
	tmpDir, err := os.MkdirTemp("", "grove-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadAsset(downloadURL, archivePath, latest.TagName, assetName); err != nil {
		return fmt.Errorf("다운로드 실패: %w", err)
	}

	newBinary := filepath.Join(tmpDir, "grove")
	if err := extractBinary(archivePath, newBinary); err != nil {
		return fmt.Errorf("압축 해제 실패: %w", err)
	}

	// Replace binary
	if err := os.Chmod(newBinary, 0o755); err != nil {
		return err
	}

	if err := replaceBinary(binPath, newBinary); err != nil {
		return fmt.Errorf("바이너리 교체 실패: %w\nsudo grove update를 시도하세요", err)
	}

	fmt.Fprintf(os.Stderr, "%s grove %s 업데이트 완료!\n", "✅", latestVer)
	return nil
}

func fetchLatestRelease() (*ghRelease, error) {
	// Try gh CLI first (handles auth)
	if _, err := exec.LookPath("gh"); err == nil {
		out, err := exec.Command("gh", "api",
			fmt.Sprintf("repos/%s/%s/releases/latest", repoOwner, repoName)).Output()
		if err == nil {
			var rel ghRelease
			if json.Unmarshal(out, &rel) == nil && rel.TagName != "" {
				return &rel, nil
			}
		}
	}

	// Fallback: direct HTTP
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("릴리스가 없습니다. 먼저 GitHub Release를 생성하세요")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func findAssetName(rel *ghRelease) string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Match goreleaser naming: grove_<version>_<os>_<arch>.tar.gz
	ver := strings.TrimPrefix(rel.TagName, "v")
	target := fmt.Sprintf("grove_%s_%s_%s.tar.gz", ver, os, arch)

	for _, a := range rel.Assets {
		if a.Name == target {
			return target
		}
	}
	return ""
}

func findBinaryPath() (string, error) {
	// Find where grove is installed
	path, err := exec.LookPath("grove")
	if err != nil {
		return "", fmt.Errorf("grove 바이너리를 찾을 수 없습니다: %w", err)
	}
	// Resolve symlinks
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, nil
	}
	return resolved, nil
}

func downloadAsset(url, dest, tagName, assetName string) error {
	// Use gh CLI if available (handles auth for private repos)
	if _, err := exec.LookPath("gh"); err == nil {
		dir := filepath.Dir(dest)
		err := exec.Command("gh", "release", "download", tagName,
			"--repo", fmt.Sprintf("%s/%s", repoOwner, repoName),
			"--pattern", assetName,
			"--dir", dir,
		).Run()
		if err == nil {
			return nil
		}
	}
	return downloadFile(url, dest)
}

func downloadFile(url, dest string) error {
	// Try with gh token for private repos
	var authHeader string
	if ghPath, err := exec.LookPath("gh"); err == nil {
		if out, err := exec.Command(ghPath, "auth", "token").Output(); err == nil {
			authHeader = "token " + strings.TrimSpace(string(out))
		}
	}

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func extractBinary(archivePath, destBinary string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Name == "grove" || strings.HasSuffix(hdr.Name, "/grove") {
			out, err := os.Create(destBinary)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, tr)
			return err
		}
	}
	return fmt.Errorf("grove 바이너리를 아카이브에서 찾을 수 없습니다")
}

func replaceBinary(oldPath, newPath string) error {
	// Try direct rename first
	if err := os.Rename(newPath, oldPath); err == nil {
		return nil
	}

	// Fallback: copy (for cross-device moves)
	src, err := os.Open(newPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(oldPath, os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// updateCheckCache stores the last version check result to avoid repeated API calls.
type updateCheckCache struct {
	CheckedAt     time.Time `json:"checkedAt"`
	LatestVersion string    `json:"latestVersion"`
}

func updateCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "grove", "update-check.json")
}

func loadUpdateCache() updateCheckCache {
	data, err := os.ReadFile(updateCachePath())
	if err != nil {
		return updateCheckCache{}
	}
	var cache updateCheckCache
	_ = json.Unmarshal(data, &cache)
	return cache
}

func saveUpdateCache(cache updateCheckCache) {
	p := updateCachePath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	data, _ := json.Marshal(cache)
	_ = os.WriteFile(p, data, 0o644)
}

// isNewerVersion returns true if latest is semantically greater than current.
func isNewerVersion(latest, current string) bool {
	lp := strings.Split(latest, ".")
	cp := strings.Split(current, ".")
	n := len(lp)
	if len(cp) > n {
		n = len(cp)
	}
	for i := 0; i < n; i++ {
		var l, c int
		if i < len(lp) {
			l, _ = strconv.Atoi(lp[i])
		}
		if i < len(cp) {
			c, _ = strconv.Atoi(cp[i])
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}

// showUpdateNoticeIfNeeded prints an update notice after a command if a newer version is available.
// Cache is refreshed at most once per 24 hours (waits up to 3s if stale).
func showUpdateNoticeIfNeeded() {
	if appVersion == "dev" {
		return
	}
	currentVer := strings.TrimPrefix(appVersion, "v")
	cache := loadUpdateCache()

	if cache.LatestVersion == "" || time.Since(cache.CheckedAt) > 24*time.Hour {
		done := make(chan updateCheckCache, 1)
		go func() {
			latest, err := fetchLatestRelease()
			if err != nil {
				done <- updateCheckCache{}
				return
			}
			c := updateCheckCache{
				CheckedAt:     time.Now(),
				LatestVersion: strings.TrimPrefix(latest.TagName, "v"),
			}
			saveUpdateCache(c)
			done <- c
		}()
		select {
		case result := <-done:
			if result.LatestVersion != "" {
				cache = result
			}
		case <-time.After(3 * time.Second):
		}
	}

	if cache.LatestVersion != "" && isNewerVersion(cache.LatestVersion, currentVer) {
		fmt.Fprintf(os.Stderr, "\n%s grove %s 업데이트가 있습니다 (현재: %s) → grove update\n",
			yellowStyle.Render("⬆"), cache.LatestVersion, currentVer)
	}
}
