package mediautil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

const defaultDownloadLimit int64 = 20 << 20

func DownloadURL(ctx context.Context, client *http.Client, rawURL string) ([]byte, string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, "", errors.New("download url is empty")
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download failed: http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultDownloadLimit))
	if err != nil {
		return nil, "", err
	}
	if len(body) == 0 {
		return nil, "", errors.New("download returned empty body")
	}
	return body, NormalizeImageMIMEType(resp.Header.Get("Content-Type"), parsed.Path, body), nil
}

func NormalizeImageMIMEType(contentType, sourcePath string, body []byte) string {
	if mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType)); err == nil && strings.HasPrefix(mediaType, "image/") {
		return mediaType
	}

	switch strings.ToLower(filepath.Ext(strings.TrimSpace(sourcePath))) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".heic":
		return "image/heic"
	case ".heif":
		return "image/heif"
	}

	if detected := http.DetectContentType(body); strings.HasPrefix(detected, "image/") {
		return detected
	}
	return "image/jpeg"
}
