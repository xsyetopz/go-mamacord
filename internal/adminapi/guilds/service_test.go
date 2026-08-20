package guilds

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
)

type scriptedOAuth struct {
	mu        sync.Mutex
	guilds    []adminauth.OAuthGuild
	nextError error
	calls     int
}

func (oauth *scriptedOAuth) ExchangeCode(context.Context, string, string) (adminauth.OAuthToken, error) {
	return adminauth.OAuthToken{}, errors.New("unexpected ExchangeCode call")
}

func (oauth *scriptedOAuth) FetchUser(context.Context, string) (adminauth.OAuthUser, error) {
	return adminauth.OAuthUser{}, errors.New("unexpected FetchUser call")
}

func (oauth *scriptedOAuth) FetchGuilds(context.Context, string) ([]adminauth.OAuthGuild, error) {
	oauth.mu.Lock()
	defer oauth.mu.Unlock()
	oauth.calls++
	if oauth.nextError != nil {
		return nil, oauth.nextError
	}
	return cloneGuilds(oauth.guilds), nil
}

func (oauth *scriptedOAuth) setNextError(err error) {
	oauth.mu.Lock()
	defer oauth.mu.Unlock()
	oauth.nextError = err
}

func (oauth *scriptedOAuth) callCount() int {
	oauth.mu.Lock()
	defer oauth.mu.Unlock()
	return oauth.calls
}

func TestUserGuildsCachesAndServesStaleDataWhileRateLimited(t *testing.T) {
	t.Parallel()

	oauth := &scriptedOAuth{guilds: []adminauth.OAuthGuild{{
		ID: "123", Name: "Manageable", Owner: true, Permissions: adminauth.OAuthPermissions("8"),
	}}}
	service := &Service{ServiceCore: ServiceCore{OAuth: oauth}}

	first, err := service.UserGuilds(context.Background(), "access-token")
	if err != nil {
		t.Fatalf("UserGuilds first call: %v", err)
	}
	if len(first) != 1 || first[0].ID != Snowflake(123) {
		t.Fatalf("unexpected first guilds: %#v", first)
	}
	if got := oauth.callCount(); got != 1 {
		t.Fatalf("OAuth calls after first fetch = %d, want 1", got)
	}

	second, err := service.UserGuilds(context.Background(), "access-token")
	if err != nil {
		t.Fatalf("UserGuilds cached call: %v", err)
	}
	if len(second) != 1 || second[0].ID != Snowflake(123) {
		t.Fatalf("unexpected cached guilds: %#v", second)
	}
	if got := oauth.callCount(); got != 1 {
		t.Fatalf("OAuth calls after cache hit = %d, want 1", got)
	}

	key := tokenCacheKey("access-token")
	service.cache.mu.Lock()
	entry := service.cache.entries[key]
	entry.expiresAt = time.Now().Add(-time.Second)
	service.cache.entries[key] = entry
	service.cache.mu.Unlock()
	oauth.setNextError(&adminauth.OAuthRateLimitError{RetryAfter: time.Minute})

	stale, err := service.UserGuilds(context.Background(), "access-token")
	if err != nil {
		t.Fatalf("UserGuilds rate-limited call: %v", err)
	}
	if len(stale) != 1 || stale[0].ID != Snowflake(123) {
		t.Fatalf("unexpected stale guilds: %#v", stale)
	}
	if got := oauth.callCount(); got != 2 {
		t.Fatalf("OAuth calls after stale refresh = %d, want 2", got)
	}
}

func TestUserGuildsExposesRateLimitAsPublicError(t *testing.T) {
	t.Parallel()

	oauth := &scriptedOAuth{nextError: &adminauth.OAuthRateLimitError{RetryAfter: 1500 * time.Millisecond}}
	service := &Service{ServiceCore: ServiceCore{OAuth: oauth}}
	_, err := service.UserGuilds(context.Background(), "access-token")
	var publicError *PublicError
	if !errors.As(err, &publicError) {
		t.Fatalf("UserGuilds error = %T %v, want *PublicError", err, err)
	}
	if publicError.StatusCode() != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", publicError.StatusCode(), http.StatusTooManyRequests)
	}
	if publicError.RetryDelay() != 1500*time.Millisecond {
		t.Fatalf("retry delay = %s, want 1.5s", publicError.RetryDelay())
	}
}

func TestInstallURLsPreserveDiscordParameters(t *testing.T) {
	t.Parallel()

	service := &Service{ServiceCore: ServiceCore{ClientID: "client-id"}}
	selected, err := service.InstallURL(123, "https://dashboard.example")
	if err != nil {
		t.Fatalf("InstallURL: %v", err)
	}
	parsed, err := url.Parse(selected)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "discord.com" || parsed.Path != "/oauth2/authorize" {
		t.Fatalf("unexpected install endpoint: %s", selected)
	}
	values := parsed.Query()
	expected := map[string]string{
		"client_id": "client-id", "scope": "bot applications.commands", "permissions": "8",
		"guild_id": "123", "disable_guild_select": "true",
	}
	for key, want := range expected {
		if got := values.Get(key); got != want {
			t.Fatalf("selected install %s = %q, want %q", key, got, want)
		}
	}

	anyGuild, err := service.InstallURLAnyGuild("https://dashboard.example")
	if err != nil {
		t.Fatalf("InstallURLAnyGuild: %v", err)
	}
	anyValues, err := url.Parse(anyGuild)
	if err != nil {
		t.Fatalf("url.Parse any guild: %v", err)
	}
	if anyValues.Query().Has("guild_id") || anyValues.Query().Has("disable_guild_select") {
		t.Fatalf("any-guild install unexpectedly pins a guild: %s", anyGuild)
	}
}

func TestGuildResponsesPreserveFlatJSONAndSnowflakeStrings(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(GuildDashboardResponse{
		Guild:   UserGuildSummary{ID: Snowflake(123)},
		Manager: ManagerSection{PluginSection: PluginSection{ID: "manager"}},
		Wellness: WellnessSection{
			PluginSection:            PluginSection{ID: "wellness"},
			DefaultReminderChannelID: Snowflake(9007199254740993),
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	guild := decoded["guild"].(map[string]any)
	if got := guild["id"]; got != "123" {
		t.Fatalf("guild id = %#v, want string 123", got)
	}
	manager := decoded["manager"].(map[string]any)
	if got := manager["id"]; got != "manager" {
		t.Fatalf("flat manager id = %#v, want manager", got)
	}
	if _, nested := manager["PluginSection"]; nested {
		t.Fatalf("manager response contains nested PluginSection: %#v", manager)
	}
	wellness := decoded["wellness"].(map[string]any)
	if got := wellness["default_reminder_channel_id"]; got != "9007199254740993" {
		t.Fatalf("wellness snowflake = %#v, want exact string", got)
	}
}
