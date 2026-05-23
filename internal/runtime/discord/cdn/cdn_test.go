package cdn_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/cdn"
)

func TestDiscordCDNFetcher_RejectsHost(t *testing.T) {
	t.Parallel()

	f := cdn.NewDiscordCDNFetcher()
	_, err := f.Fetch(context.Background(), "https://example.com/x.png", 10)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestDiscordCDNFetcher_AllowsMediaAndCdn(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Hostname() != "media.discordapp.net" && r.URL.Hostname() != "cdn.discordapp.com" {
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Body:       io.NopCloser(strings.NewReader("forbidden")),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		}),
	}
	f := cdn.NewDiscordCDNFetcherWithClient(client)

	if _, err := f.Fetch(context.Background(), "https://cdn.discordapp.com/x", 10); err != nil {
		t.Fatalf("cdn fetch: %v", err)
	}
	if _, err := f.Fetch(context.Background(), "https://media.discordapp.net/x", 10); err != nil {
		t.Fatalf("media fetch: %v", err)
	}
}

func TestDiscordCDNFetcher_EnforcesSizeLimit(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("0123456789")),
			}, nil
		}),
	}
	f := cdn.NewDiscordCDNFetcherWithClient(client)

	if _, err := f.Fetch(context.Background(), "https://cdn.discordapp.com/x", 5); err == nil {
		t.Fatalf("expected error")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
