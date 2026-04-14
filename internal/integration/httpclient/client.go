package httpclient

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func New(timeout time.Duration, proxyURL string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse proxy url: %w", err)
		}
		if strings.TrimSpace(u.Scheme) == "" || strings.TrimSpace(u.Host) == "" {
			return nil, fmt.Errorf("proxy url must include scheme and host")
		}
		transport.Proxy = http.ProxyURL(u)
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}
