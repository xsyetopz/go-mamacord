package cdn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DiscordCDNFetcher struct {
	c *http.Client
}

const defaultDiscordCDNTimeout = 5 * time.Second

func NewDiscordCDNFetcher() *DiscordCDNFetcher {
	return NewDiscordCDNFetcherWithClient(nil)
}

func NewDiscordCDNFetcherWithClient(client *http.Client) *DiscordCDNFetcher {
	if client == nil {
		client = &http.Client{
			Timeout: defaultDiscordCDNTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > maxRedirects {
					return errors.New("too many redirects")
				}
				if !isAllowedDiscordCDNURL(req.URL) {
					return errors.New("redirect target not allowed")
				}
				return nil
			},
		}
	}
	return &DiscordCDNFetcher{c: client}
}

const maxRedirects = 2

func (f *DiscordCDNFetcher) Fetch(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("url is required")
	}
	if maxBytes <= 0 {
		return nil, errors.New("maxBytes must be positive")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if !isAllowedDiscordCDNURL(u) {
		return nil, errors.New("url host not allowed")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cdn status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxBytes+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxBytes {
		return nil, errors.New("file too large")
	}
	return b, nil
}

func isAllowedDiscordCDNURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	if strings.ToLower(strings.TrimSpace(u.Scheme)) != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" && u.Port() != "443" {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	switch host {
	case "cdn.discordapp.com", "media.discordapp.net":
		return true
	default:
		return false
	}
}
