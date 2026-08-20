package guilds

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	discordcontrol "github.com/xsyetopz/go-mamacord/internal/runtime/discord/control"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
)

var ErrGuildNotAccessible = errors.New("guild is not accessible to this user")

type ServiceCore struct {
	ClientID    string
	OAuth       adminauth.OAuthClient
	ModuleAdmin moduleapi.Admin
}

type ServiceStores struct {
	PluginKV storage.PluginKVStore
	Warnings storage.WarningStore
	Audit    storage.AuditStore
}

type DiscordAccess struct {
	KnownGuildIDs func() []uint64
	BotHasGuild   func(context.Context, uint64) (bool, error)
}

type DiscordCatalog struct {
	ListGuildChannels  func(context.Context, uint64) ([]GuildChannelInfo, error)
	ListGuildRoles     func(context.Context, uint64) ([]GuildRoleInfo, error)
	SearchGuildMembers func(context.Context, uint64, string, int) ([]GuildMemberInfo, error)
	ListGuildEmojis    func(context.Context, uint64) ([]GuildEmojiInfo, error)
	ListGuildStickers  func(context.Context, uint64) ([]GuildStickerInfo, error)
}

type DiscordModeration struct {
	SetSlowmode   func(context.Context, uint64, int) error
	SetNickname   func(context.Context, uint64, uint64, *string) error
	TimeoutMember func(context.Context, uint64, uint64, int64) error
	PurgeMessages func(context.Context, discordcontrol.PurgeSpec) (int, error)
}

type DiscordRoles struct {
	CreateRole func(context.Context, discordcontrol.RoleCreateSpec) (discordcontrol.RoleResult, error)
	EditRole   func(context.Context, discordcontrol.RoleEditSpec) (discordcontrol.RoleResult, error)
	DeleteRole func(context.Context, uint64, uint64) error
	AddRole    func(context.Context, discordcontrol.RoleMemberSpec) error
	RemoveRole func(context.Context, discordcontrol.RoleMemberSpec) error
}

type DiscordMedia struct {
	CreateEmojiUpload   func(context.Context, uint64, string, string, []byte, int, int) (discordcontrol.EmojiResult, error)
	EditEmoji           func(context.Context, discordcontrol.EmojiEditSpec) (discordcontrol.EmojiResult, error)
	DeleteEmoji         func(context.Context, discordcontrol.EmojiDeleteSpec) error
	CreateStickerUpload func(context.Context, uint64, string, string, string, string, []byte, int, int) (discordcontrol.StickerResult, error)
	EditSticker         func(context.Context, discordcontrol.StickerEditSpec) (discordcontrol.StickerResult, error)
	DeleteSticker       func(context.Context, discordcontrol.StickerDeleteSpec) error
}

type Service struct {
	ServiceCore
	ServiceStores
	DiscordAccess
	DiscordCatalog
	DiscordModeration
	DiscordRoles
	DiscordMedia
	cache guildCache
}

type guildCache struct {
	mu       sync.Mutex
	entries  map[string]guildsCacheEntry
	inflight map[string]*guildsInflight
}

type guildsCacheEntry struct {
	fetchedAt    time.Time
	expiresAt    time.Time
	blockedUntil time.Time
	retryAfter   time.Duration
	guilds       []adminauth.OAuthGuild
}

type guildsInflight struct {
	done   chan struct{}
	guilds []adminauth.OAuthGuild
	err    error
}

func (s *Service) Init() {
	if s == nil {
		return
	}
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	if s.cache.entries == nil {
		s.cache.entries = map[string]guildsCacheEntry{}
	}
	if s.cache.inflight == nil {
		s.cache.inflight = map[string]*guildsInflight{}
	}
}

const (
	guildsCacheTTL              = 30 * time.Second
	guildsStaleWhileRateLimited = 10 * time.Minute
)

func tokenCacheKey(accessToken string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(accessToken)))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func cloneGuilds(in []adminauth.OAuthGuild) []adminauth.OAuthGuild {
	if len(in) == 0 {
		return nil
	}
	out := make([]adminauth.OAuthGuild, len(in))
	copy(out, in)
	return out
}

func (s *Service) fetchGuildsCached(ctx context.Context, accessToken string) ([]adminauth.OAuthGuild, error) {
	if s == nil || s.OAuth == nil {
		return nil, errors.New("oauth client is not configured")
	}

	key := tokenCacheKey(accessToken)
	now := time.Now()

	s.cache.mu.Lock()
	if s.cache.entries == nil {
		s.cache.entries = map[string]guildsCacheEntry{}
	}
	if s.cache.inflight == nil {
		s.cache.inflight = map[string]*guildsInflight{}
	}

	if entry, ok := s.cache.entries[key]; ok {
		if now.Before(entry.blockedUntil) {
			if len(entry.guilds) > 0 && now.Sub(entry.fetchedAt) <= guildsStaleWhileRateLimited {
				out := cloneGuilds(entry.guilds)
				s.cache.mu.Unlock()
				return out, nil
			}
			retry := time.Until(entry.blockedUntil)
			if retry < 0 {
				retry = 0
			}
			s.cache.mu.Unlock()
			return nil, &PublicError{
				Status:     http.StatusTooManyRequests,
				Message:    "Discord is rate limiting right now. Please try again in a moment.",
				RetryAfter: retry,
			}
		}

		if now.Before(entry.expiresAt) && len(entry.guilds) > 0 {
			out := cloneGuilds(entry.guilds)
			s.cache.mu.Unlock()
			return out, nil
		}
	}

	if inflight, ok := s.cache.inflight[key]; ok && inflight != nil {
		done := inflight.done
		s.cache.mu.Unlock()
		select {
		case <-done:
			return cloneGuilds(inflight.guilds), inflight.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	inflight := &guildsInflight{done: make(chan struct{})}
	s.cache.inflight[key] = inflight
	s.cache.mu.Unlock()

	guilds, err := s.OAuth.FetchGuilds(ctx, accessToken)

	s.cache.mu.Lock()
	delete(s.cache.inflight, key)

	entry := s.cache.entries[key]
	if err == nil {
		entry.guilds = cloneGuilds(guilds)
		entry.fetchedAt = now
		entry.expiresAt = now.Add(guildsCacheTTL)
		entry.blockedUntil = time.Time{}
		entry.retryAfter = 0
	} else if rateLimit, ok := adminauth.AsRateLimit(err); ok {
		entry.retryAfter = rateLimit.RetryAfter
		entry.blockedUntil = now.Add(rateLimit.RetryAfter)
		if len(entry.guilds) > 0 && entry.expiresAt.Before(now.Add(rateLimit.RetryAfter)) {
			entry.expiresAt = now.Add(rateLimit.RetryAfter)
		}
	}
	s.cache.entries[key] = entry

	inflight.guilds = cloneGuilds(guilds)
	inflight.err = err
	close(inflight.done)
	s.cache.mu.Unlock()

	if err != nil {
		if _, ok := adminauth.AsRateLimit(err); ok {
			s.cache.mu.Lock()
			cached := s.cache.entries[key]
			s.cache.mu.Unlock()
			if len(cached.guilds) > 0 && now.Sub(cached.fetchedAt) <= guildsStaleWhileRateLimited {
				return cloneGuilds(cached.guilds), nil
			}
			return nil, &PublicError{
				Status:     http.StatusTooManyRequests,
				Message:    "Discord is rate limiting right now. Please try again in a moment.",
				RetryAfter: cached.blockedUntil.Sub(now),
			}
		}
	}

	return cloneGuilds(guilds), err
}

type PublicError struct {
	Status     int
	Message    string
	RetryAfter time.Duration
}

func (e *PublicError) Error() string { return e.Message }

func (e *PublicError) PublicMessage() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *PublicError) RetryDelay() time.Duration {
	if e == nil {
		return 0
	}
	return e.RetryAfter
}

func (e *PublicError) StatusCode() int {
	if e == nil || e.Status == 0 {
		return http.StatusBadRequest
	}
	return e.Status
}

func DiscordRuntimeUnavailable(feature string) error {
	message := "discord runtime is unavailable"
	if feature = strings.TrimSpace(feature); feature != "" {
		message += " for " + feature
	}
	return &PublicError{Status: http.StatusServiceUnavailable, Message: message}
}
