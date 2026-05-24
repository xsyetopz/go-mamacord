package adminapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/buildinfo"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/ops"
)

func optionalUint64(value uint64) *uint64 {
	return &value
}

type fakeOAuthClient struct{}

func (fakeOAuthClient) ExchangeCode(context.Context, string, string) (OAuthToken, error) {
	return OAuthToken{AccessToken: "token", TokenType: "Bearer", Scope: "identify guilds"}, nil
}

func (fakeOAuthClient) FetchUser(context.Context, string) (OAuthUser, error) {
	return OAuthUser{
		ID:         "42",
		Username:   "owner",
		GlobalName: "Owner",
		Avatar:     "abc",
	}, nil
}

func (fakeOAuthClient) FetchGuilds(context.Context, string) ([]OAuthGuild, error) {
	return []OAuthGuild{
		{
			ID:          "1",
			Name:        "Guild",
			Owner:       true,
			Permissions: OAuthPermissions("8"),
		},
	}, nil
}

func TestHandleMeRequiresSession(t *testing.T) {
	t.Parallel()

	server, err := New(Options{
		Addr:          "127.0.0.1:0",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Service:       Service{},
		SessionSecret: strings.Repeat("x", 32),
		ClientID:      "cid",
		ClientSecret:  "secret",
		OwnerStatus: func() OwnerStatus {
			return OwnerStatus{
				Configured:      true,
				Resolved:        true,
				Source:          "discord",
				EffectiveUserID: optionalUint64(42),
			}
		},
		OAuthClient: fakeOAuthClient{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	server.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, _ := payload["authenticated"].(bool); got {
		t.Fatalf("expected authenticated=false, got %#v", payload)
	}
}

func TestHandleModulesWithSession(t *testing.T) {
	t.Parallel()

	server, err := New(Options{
		Addr:   "127.0.0.1:0",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Service: Service{
			Config: config.Config{},
			BuildInfo: func() buildinfo.Info {
				return buildinfo.Info{Version: "test"}
			},
			Snapshot: func() ops.Snapshot {
				return ops.Snapshot{Ready: true}
			},
		},
		SessionSecret: strings.Repeat("x", 32),
		ClientID:      "cid",
		ClientSecret:  "secret",
		OwnerStatus: func() OwnerStatus {
			return OwnerStatus{
				Configured:      true,
				Resolved:        true,
				Source:          "discord",
				EffectiveUserID: optionalUint64(42),
			}
		},
		OAuthClient: fakeOAuthClient{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := server.putSession(context.Background(), session{
		ID:        "session-token",
		UserID:    42,
		Username:  "owner",
		Name:      "Owner",
		CSRFToken: "csrf",
		IsOwner:   true,
		ExpiresAt: 4102444800,
	}); err != nil {
		t.Fatalf("putSession: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/owner/status", nil)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: server.signCookieValue(sessionCookieName, "session-token"),
	})
	server.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := payload["snapshot"]; !ok {
		t.Fatalf("expected snapshot in response")
	}
	setupAny, ok := payload["setup"]
	if !ok {
		t.Fatalf("expected setup in response")
	}
	setup, ok := setupAny.(map[string]any)
	if !ok {
		t.Fatalf("expected setup to be object, got %T", setupAny)
	}
	hintsAny, ok := setup["hints"]
	if !ok {
		t.Fatalf("expected setup.hints in response")
	}
	if _, ok := hintsAny.([]any); !ok {
		t.Fatalf("expected setup.hints to be array, got %T", hintsAny)
	}
}

func TestHandleLoginReturns503WhenAuthNotConfigured(t *testing.T) {
	t.Parallel()

	server, err := New(Options{
		Addr:          "127.0.0.1:0",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Service:       Service{},
		SessionSecret: strings.Repeat("x", 32),
		ClientID:      "",
		ClientSecret:  "",
		OwnerStatus: func() OwnerStatus {
			return OwnerStatus{Resolved: false, Source: "unresolved"}
		},
		OAuthClient: fakeOAuthClient{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	server.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSetupWithoutSession(t *testing.T) {
	t.Parallel()

	server, err := New(Options{
		Addr:   "127.0.0.1:0",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Service: Service{
			Config: config.Config{
				AdminAddr:               "127.0.0.1:8081",
				DashboardClientID:       "client-id",
				DashboardClientSecret:   "client-secret",
				DashboardSessionSecret:  strings.Repeat("x", 32),
				DashboardSigningKeyID:   "official",
				DashboardSigningKeyFile: "/tmp/key",
				OwnerUserID:             optionalUint64(42),
			},
			OwnerStatus: func() OwnerStatus {
				return OwnerStatus{
					Configured:      true,
					Resolved:        true,
					Source:          "discord",
					EffectiveUserID: optionalUint64(42),
				}
			},
		},
		SessionSecret: strings.Repeat("x", 32),
		ClientID:      "cid",
		ClientSecret:  "secret",
		OwnerStatus: func() OwnerStatus {
			return OwnerStatus{
				Configured:      true,
				Resolved:        true,
				Source:          "discord",
				EffectiveUserID: optionalUint64(42),
			}
		},
		OAuthClient: fakeOAuthClient{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/setup", nil)
	server.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, _ := payload["auth_configured"].(bool); !got {
		t.Fatalf("expected auth_configured=true, got %#v", payload["auth_configured"])
	}
	if got, _ := payload["login_ready"].(bool); !got {
		t.Fatalf("expected login_ready=true, got %#v", payload["login_ready"])
	}
	if got, _ := payload["owner_configured"].(bool); !got {
		t.Fatalf("expected owner_configured=true, got %#v", payload["owner_configured"])
	}
	if got, _ := payload["owner_resolved"].(bool); !got {
		t.Fatalf("expected owner_resolved=true, got %#v", payload["owner_resolved"])
	}
	if got, _ := payload["owner_source"].(string); got != "discord" {
		t.Fatalf("expected owner_source=discord, got %#v", payload["owner_source"])
	}
}

func TestHandleGuildChannelsReturns503WhenDiscordRuntimeUnavailable(t *testing.T) {
	t.Parallel()

	server, err := New(Options{
		Addr:   "127.0.0.1:0",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Service: Service{
			OAuth: fakeOAuthClient{},
		},
		SessionSecret: strings.Repeat("x", 32),
		ClientID:      "cid",
		ClientSecret:  "secret",
		OwnerStatus: func() OwnerStatus {
			return OwnerStatus{
				Configured:      true,
				Resolved:        true,
				Source:          "discord",
				EffectiveUserID: optionalUint64(42),
			}
		},
		OAuthClient: fakeOAuthClient{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := server.putSession(context.Background(), session{
		ID:          "session-token",
		UserID:      42,
		Username:    "owner",
		Name:        "Owner",
		AccessToken: "token",
		CSRFToken:   "csrf",
		IsOwner:     true,
		ExpiresAt:   4102444800,
	}); err != nil {
		t.Fatalf("putSession: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/guilds/channels?guild_id=1", nil)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: server.signCookieValue(sessionCookieName, "session-token"),
	})
	server.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, _ := payload["error"].(string); !strings.Contains(got, "discord runtime") {
		t.Fatalf("expected discord runtime error, got %#v", payload)
	}
}

func TestAdminOwnerRoutesRequireSession(t *testing.T) {
	t.Parallel()

	server, err := New(Options{
		Addr:          "127.0.0.1:0",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Service:       Service{},
		SessionSecret: strings.Repeat("x", 32),
		ClientID:      "cid",
		ClientSecret:  "secret",
		OwnerStatus: func() OwnerStatus {
			return OwnerStatus{
				Configured:      true,
				Resolved:        true,
				Source:          "discord",
				EffectiveUserID: optionalUint64(42),
			}
		},
		OAuthClient: fakeOAuthClient{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, path := range []string{
		"/api/owner/plugins/search",
		"/api/owner/config/modules",
		"/api/owner/migrations/status",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		server.handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}
