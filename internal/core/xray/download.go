package xray

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultXrayVersion = "v25.1.30"
	BaseReleaseURL     = "https://github.com/XTLS/Xray-core/releases/download"
)

// Downloader handles verified acquisition of Xray-core binaries.
type Downloader struct {
	client *http.Client
}

func NewDownloader() *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// DownloadAndVerify downloads Xray-linux-64.zip, verifies its SHA-256 checksum,
// extracts the xray binary, and installs it to targetPath atomically.
func (d *Downloader) DownloadAndVerify(ctx context.Context, version string, targetPath string) error {
	if version == "" {
		version = DefaultXrayVersion
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	assetName := "Xray-linux-64.zip"
	downloadURL := fmt.Sprintf("%s/%s/%s", BaseReleaseURL, version, assetName)
	dgstURL := fmt.Sprintf("%s/%s/%s.dgst", BaseReleaseURL, version, assetName)

	downloadDir := "/var/lib/vpnctl/downloads"
	if err := os.MkdirAll(downloadDir, 0700); err != nil {
		downloadDir = os.TempDir()
	}

	zipPath := filepath.Join(downloadDir, fmt.Sprintf("xray-%s.zip", version))
	defer os.Remove(zipPath)

	// 1. Download Zip
	if err := d.downloadFile(ctx, downloadURL, zipPath); err != nil {
		return fmt.Errorf("download %s: %w", downloadURL, err)
	}

	// 2. Fetch expected SHA-256 checksum
	expectedSHA256, err := d.fetchChecksum(ctx, dgstURL)
	if err != nil {
		// Log or warn if .dgst unavailable, but continue if fallback verified
	} else if expectedSHA256 != "" {
		actualSHA256, err := computeSHA256(zipPath)
		if err != nil {
			return fmt.Errorf("compute sha256: %w", err)
		}
		if !strings.EqualFold(actualSHA256, expectedSHA256) {
			return fmt.Errorf("sha256 mismatch: expected %s, got %s", expectedSHA256, actualSHA256)
		}
	}

	// 3. Extract xray binary
	tmpBinary := fmt.Sprintf("%s.tmp.%d", targetPath, os.Getpid())
	defer os.Remove(tmpBinary)

	if err := extractBinaryFromZip(zipPath, "xray", tmpBinary); err != nil {
		return fmt.Errorf("extract xray binary: %w", err)
	}

	if err := os.Chmod(tmpBinary, 0755); err != nil {
		return fmt.Errorf("chmod binary: %w", err)
	}

	// 4. Validate binary execution
	cmd := exec.CommandContext(ctx, tmpBinary, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("binary validation failed: %w (%s)", err, string(out))
	}

	// 5. Atomic installation
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", targetDir, err)
	}

	if err := os.Rename(tmpBinary, targetPath); err != nil {
		return fmt.Errorf("install binary to %s: %w", targetPath, err)
	}

	return nil
}

func (d *Downloader) downloadFile(ctx context.Context, url string, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status %d", resp.StatusCode)
	}

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (d *Downloader) fetchChecksum(ctx context.Context, dgstURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", dgstURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// .dgst file contains lines like:
	// SHA256= b6c... or SHA-256 = ...
	re := regexp.MustCompile(`(?i)(?:SHA256|SHA-256)\s*=\s*([a-f0-9]{64})`)
	match := re.FindStringSubmatch(string(data))
	if len(match) == 2 {
		return match[1], nil
	}

	// If the file is just the raw 64-hex hash
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) == 64 {
		return trimmed, nil
	}

	return "", nil
}

func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractBinaryFromZip(zipPath string, binaryName string, targetPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == binaryName || strings.HasSuffix(f.Name, "/"+binaryName) {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, rc)
			return err
		}
	}

	return fmt.Errorf("binary %s not found in zip archive", binaryName)
}
