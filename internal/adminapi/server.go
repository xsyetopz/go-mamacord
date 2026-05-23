package adminapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

const (
	sessionCookieName = "mamacord_admin_session"
	stateCookieName   = "mamacord_admin_state"
	sessionTTL        = 12 * time.Hour
)

type OAuthUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Avatar     string `json:"avatar"`
}

type OAuthClient interface {
	ExchangeCode(ctx context.Context, code string, redirectURL string) (OAuthToken, error)
	FetchUser(ctx context.Context, accessToken string) (OAuthUser, error)
	FetchGuilds(ctx context.Context, accessToken string) ([]OAuthGuild, error)
}

type Options struct {
	Addr          string
	Logger        *slog.Logger
	Service       Service
	SessionSecret string
	ClientID      string
	ClientSecret  string
	OwnerStatus   func() OwnerStatus
	OAuthClient   OAuthClient
	SessionStore  store.AdminSessionStore
}

type Server struct {
	logger *slog.Logger
	addr   string
	svc    *Service

	clientID     string
	clientSecret string
	ownerStatus  func() OwnerStatus
	oauth        OAuthClient
	secret       []byte

	sessions store.AdminSessionStore

	stateMu    sync.Mutex
	stateStore map[string]oauthState

	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
}

type oauthState struct {
	RedirectURL string
	ReturnBase  string
}

type session struct {
	ID          string
	UserID      uint64
	Username    string
	Name        string
	AvatarURL   string
	CSRFToken   string
	AccessToken string
	IsOwner     bool
	ExpiresAt   int64
}

func New(opts Options) (*Server, error) {
	if strings.TrimSpace(opts.Addr) == "" {
		return nil, nil
	}
	if opts.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if opts.OAuthClient == nil {
		opts.OAuthClient = NewDiscordOAuthClient(opts.ClientID, opts.ClientSecret)
	}
	sessionStore := opts.SessionStore
	if sessionStore == nil {
		sessionStore = newMemorySessionStore()
	}
	svc := opts.Service
	svc.init()
	return &Server{
		logger:       opts.Logger.With(slog.String("component", "admin_api")),
		addr:         strings.TrimSpace(opts.Addr),
		svc:          &svc,
		clientID:     strings.TrimSpace(opts.ClientID),
		clientSecret: strings.TrimSpace(opts.ClientSecret),
		ownerStatus:  opts.OwnerStatus,
		oauth:        opts.OAuthClient,
		secret:       []byte(opts.SessionSecret),
		sessions:     sessionStore,
		stateStore:   map[string]oauthState{},
	}, nil
}

func (s *Server) Start() error {
	if s == nil || s.addr == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		return nil
	}

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.listener = listener
	s.server = httpServer
	go func() {
		err := httpServer.Serve(listener)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return
		}
		s.logger.Error("admin server stopped unexpectedly", slog.String("err", err.Error()))
	}()
	s.logger.Info("admin server listening", slog.String("addr", listener.Addr().String()))
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	server := s.server
	s.server = nil
	s.listener = nil
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

func (s *Server) Close(ctx context.Context) error {
	return s.Shutdown(ctx)
}

func (s *Server) handler() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("/api/setup", s.handleSetup)
	api.HandleFunc("/api/auth/login", s.handleLogin)
	api.HandleFunc("/api/auth/callback", s.handleCallback)
	api.HandleFunc("/api/auth/me", s.handleMe)
	api.HandleFunc("/api/auth/logout", s.handleLogout)
	api.HandleFunc("/api/guilds", s.withAuth(s.handleGuilds))
	api.HandleFunc("/api/guilds/dashboard", s.withAuth(s.handleGuildDashboard))
	api.HandleFunc("/api/guilds/config", s.withAuth(s.withCSRF(s.handleGuildConfig)))
	api.HandleFunc("/api/guilds/channels", s.withAuth(s.handleGuildChannels))
	api.HandleFunc("/api/guilds/roles", s.withAuth(s.handleGuildRoles))
	api.HandleFunc("/api/guilds/members", s.withAuth(s.handleGuildMembers))
	api.HandleFunc("/api/guilds/emojis", s.withAuth(s.handleGuildEmojis))
	api.HandleFunc("/api/guilds/stickers", s.withAuth(s.handleGuildStickers))
	api.HandleFunc("/api/guilds/moderation/warnings", s.withAuth(s.handleGuildWarnings))
	api.HandleFunc("/api/guilds/moderation/warn", s.withAuth(s.withCSRF(s.handleGuildWarn)))
	api.HandleFunc("/api/guilds/moderation/unwarn", s.withAuth(s.withCSRF(s.handleGuildUnwarn)))
	api.HandleFunc("/api/guilds/manager/slowmode", s.withAuth(s.withCSRF(s.handleGuildSlowmode)))
	api.HandleFunc("/api/guilds/manager/nick", s.withAuth(s.withCSRF(s.handleGuildNickname)))
	api.HandleFunc("/api/guilds/manager/roles/create", s.withAuth(s.withCSRF(s.handleGuildRoleCreate)))
	api.HandleFunc("/api/guilds/manager/roles/edit", s.withAuth(s.withCSRF(s.handleGuildRoleEdit)))
	api.HandleFunc("/api/guilds/manager/roles/delete", s.withAuth(s.withCSRF(s.handleGuildRoleDelete)))
	api.HandleFunc("/api/guilds/manager/roles/member", s.withAuth(s.withCSRF(s.handleGuildRoleMember)))
	api.HandleFunc("/api/guilds/manager/purge", s.withAuth(s.withCSRF(s.handleGuildPurge)))
	api.HandleFunc("/api/guilds/manager/emojis/create", s.withAuth(s.withCSRF(s.handleGuildEmojiCreate)))
	api.HandleFunc("/api/guilds/manager/emojis/edit", s.withAuth(s.withCSRF(s.handleGuildEmojiEdit)))
	api.HandleFunc("/api/guilds/manager/emojis/delete", s.withAuth(s.withCSRF(s.handleGuildEmojiDelete)))
	api.HandleFunc("/api/guilds/manager/stickers/create", s.withAuth(s.withCSRF(s.handleGuildStickerCreate)))
	api.HandleFunc("/api/guilds/manager/stickers/edit", s.withAuth(s.withCSRF(s.handleGuildStickerEdit)))
	api.HandleFunc("/api/guilds/manager/stickers/delete", s.withAuth(s.withCSRF(s.handleGuildStickerDelete)))
	api.HandleFunc("/api/install/start", s.withAuth(s.handleInstallStart))
	api.HandleFunc("/api/install/callback", s.withAuth(s.handleInstallCallback))

	api.HandleFunc("/api/owner/status", s.withOwner(s.handleStatus))
	api.HandleFunc("/api/owner/modules", s.withOwner(s.handleModules))
	api.HandleFunc("/api/owner/modules/set", s.withOwner(s.withCSRF(s.handleSetModule)))
	api.HandleFunc("/api/owner/modules/reset", s.withOwner(s.withCSRF(s.handleResetModule)))
	api.HandleFunc("/api/owner/modules/reload", s.withOwner(s.withCSRF(s.handleReloadModules)))

	api.HandleFunc("/api/owner/plugins", s.withOwner(s.handlePlugins))
	api.HandleFunc("/api/owner/plugins/reload", s.withOwner(s.withCSRF(s.handleReloadPlugins)))
	api.HandleFunc("/api/owner/plugins/scaffold", s.withOwner(s.withCSRF(s.handleScaffoldPlugin)))
	api.HandleFunc("/api/owner/plugins/sign", s.withOwner(s.withCSRF(s.handleSignPlugin)))
	api.HandleFunc("/api/owner/plugins/sources", s.withOwner(s.withCSRF(s.handleMarketplaceSources)))
	api.HandleFunc("/api/owner/plugins/sources/sync", s.withOwner(s.withCSRF(s.handleMarketplaceSourceSync)))
	api.HandleFunc("/api/owner/plugins/search", s.withOwner(s.handleMarketplaceSearch))
	api.HandleFunc("/api/owner/plugins/install", s.withOwner(s.withCSRF(s.handleMarketplaceInstall)))
	api.HandleFunc("/api/owner/plugins/update", s.withOwner(s.withCSRF(s.handleMarketplaceUpdate)))
	api.HandleFunc("/api/owner/plugins/uninstall", s.withOwner(s.withCSRF(s.handleMarketplaceUninstall)))
	api.HandleFunc("/api/owner/plugins/trust/signer", s.withOwner(s.withCSRF(s.handleMarketplaceTrustSigner)))
	api.HandleFunc("/api/owner/plugins/trust/vendor", s.withOwner(s.withCSRF(s.handleMarketplaceTrustVendor)))

	api.HandleFunc("/api/owner/config/modules", s.withOwner(s.handleModulesConfig))
	api.HandleFunc("/api/owner/config/permissions", s.withOwner(s.handlePermissionsConfig))
	api.HandleFunc("/api/owner/config/trusted-keys", s.withOwner(s.handleTrustedKeys))

	api.HandleFunc("/api/owner/migrations/status", s.withOwner(s.handleMigrationStatus))
	api.HandleFunc("/api/owner/migrations/backup", s.withOwner(s.withCSRF(s.handleMigrationBackup)))
	api.HandleFunc("/api/owner/migrations/up", s.withOwner(s.withCSRF(s.handleMigrationUp)))

	root := http.NewServeMux()
	root.Handle("/api/", api)
	root.Handle("/", s.dashboardHandler())
	return s.withCORS(root)
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.Setup(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dashboardBase := s.dashboardBaseURL(r)
	apiBase := s.apiBaseURL(r)
	resp.AppOrigin = strings.TrimRight(dashboardBase, "/")
	resp.RedirectURL = strings.TrimRight(apiBase, "/") + "/api/auth/callback"
	resp.InstallRedirectURL = strings.TrimRight(apiBase, "/") + "/api/install/callback"
	writeJSON(w, http.StatusOK, resp)
}

func normalizeOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")
	return raw
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	// Primary dev path is same-origin (admin API serves/proxies the dashboard).
	// This is a safety net for cases where the dashboard runs on a different local
	// origin (e.g. opening Vite directly on :5173 without a reverse proxy).
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if next == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		origin := strings.TrimSpace(r.Header.Get("Origin"))
		allowOrigin := ""
		if origin != "" && s.allowCORSOrigin(r, origin) {
			allowOrigin = origin
		}

		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			// Only answer preflight for routes we intentionally expose to browsers.
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) allowCORSOrigin(r *http.Request, origin string) bool {
	if s == nil {
		return false
	}

	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return false
	}

	norm := normalizeOrigin(origin)
	for _, allowed := range s.svc.Config.DashboardAllowedOrigins {
		if strings.EqualFold(norm, normalizeOrigin(allowed)) {
			return true
		}
	}

	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	// Localhost is always safe to allow. This avoids confusing CORS breakage for
	// local testing (including accidental "prod mode" toggles).
	if isLocalHostname(host) {
		return true
	}

	return false
}

func (s *Server) withAuth(next func(http.ResponseWriter, *http.Request, session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, err := s.readSession(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r, sess)
	}
}

func (s *Server) withOwner(next func(http.ResponseWriter, *http.Request, session)) http.HandlerFunc {
	return s.withAuth(func(w http.ResponseWriter, r *http.Request, sess session) {
		if !s.isOwnerUser(sess.UserID) {
			writeError(w, http.StatusForbidden, "owner access required")
			return
		}
		next(w, r, sess)
	})
}

func (s *Server) withCSRF(next func(http.ResponseWriter, *http.Request, session)) func(http.ResponseWriter, *http.Request, session) {
	return func(w http.ResponseWriter, r *http.Request, sess session) {
		if r.Method == http.MethodGet {
			next(w, r, sess)
			return
		}
		if subtleTokenCompare(r.Header.Get("X-CSRF-Token"), sess.CSRFToken) == false {
			writeError(w, http.StatusForbidden, "csrf validation failed")
			return
		}
		next(w, r, sess)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.authConfigured() {
		writeError(w, http.StatusServiceUnavailable, "dashboard auth is not configured")
		return
	}
	state, err := randomToken(24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start login")
		return
	}
	returnBase := s.dashboardBaseURL(r)
	apiBase := s.apiBaseURL(r)
	redirectURL := strings.TrimRight(apiBase, "/") + "/api/auth/callback"

	s.stateMu.Lock()
	s.stateStore[state] = oauthState{
		RedirectURL: redirectURL,
		ReturnBase:  returnBase,
	}
	s.stateMu.Unlock()

	http.SetCookie(w, s.cookie(r, stateCookieName, state, 10*time.Minute, true))

	values := url.Values{}
	values.Set("client_id", s.clientID)
	values.Set("response_type", "code")
	values.Set("scope", "identify guilds")
	values.Set("redirect_uri", redirectURL)
	values.Set("state", state)
	http.Redirect(w, r, "https://discord.com/oauth2/authorize?"+values.Encode(), http.StatusFound)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	queryState := strings.TrimSpace(r.URL.Query().Get("state"))
	if !s.authConfigured() {
		writeError(w, http.StatusServiceUnavailable, "dashboard auth is not configured")
		return
	}
	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	state, ok := s.unsignCookieValue(stateCookieName, cookie.Value)
	if !ok || !subtleTokenCompare(state, queryState) {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}

	s.stateMu.Lock()
	stateData, ok := s.stateStore[state]
	delete(s.stateStore, state)
	s.stateMu.Unlock()
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing oauth code")
		return
	}
	token, err := s.oauth.ExchangeCode(r.Context(), code, stateData.RedirectURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oauth token exchange failed")
		return
	}
	user, err := s.oauth.FetchUser(r.Context(), token.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oauth user lookup failed")
		return
	}
	userID, err := strconv.ParseUint(strings.TrimSpace(user.ID), 10, 64)
	if err != nil {
		writeError(w, http.StatusForbidden, "invalid oauth user")
		return
	}
	csrfToken, err := randomToken(24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	displayName := strings.TrimSpace(user.GlobalName)
	if displayName == "" {
		displayName = strings.TrimSpace(user.Username)
	}
	sessionID, err := randomToken(24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	isOwner := s.isOwnerUser(userID)
	sess := session{
		ID:          sessionID,
		UserID:      userID,
		Username:    strings.TrimSpace(user.Username),
		Name:        displayName,
		AvatarURL:   avatarURL(user),
		CSRFToken:   csrfToken,
		AccessToken: token.AccessToken,
		IsOwner:     isOwner,
		ExpiresAt:   time.Now().Add(sessionTTL).Unix(),
	}
	if err := s.putSession(r.Context(), sess); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, s.cookie(r, sessionCookieName, sessionID, sessionTTL, true))
	http.SetCookie(w, s.cookie(r, stateCookieName, "", -time.Hour, true))

	redirectTarget := strings.TrimRight(stateData.ReturnBase, "/") + "/#/servers"
	if isOwner {
		redirectTarget = strings.TrimRight(stateData.ReturnBase, "/") + "/#/owner"
	}
	http.Redirect(w, r, redirectTarget, http.StatusFound)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, err := s.readSession(r)
	if err != nil {
		// Beginner-friendly: /api/auth/me is diagnostic, not a hard 401.
		writeJSON(w, http.StatusOK, SessionResponse{
			Authenticated: false,
		})
		return
	}
	resp := SessionResponse{
		Authenticated: true,
		IsOwner:       s.isOwnerUser(sess.UserID),
		CSRFToken:     sess.CSRFToken,
	}
	resp.User.ID = Snowflake(sess.UserID)
	resp.User.Username = sess.Username
	resp.User.Name = sess.Name
	resp.User.AvatarURL = sess.AvatarURL
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if sessionID, ok := s.unsignCookieValue(sessionCookieName, cookie.Value); ok {
			_ = s.deleteSession(r.Context(), sessionID)
		}
	}
	http.SetCookie(w, s.cookie(r, sessionCookieName, "", -time.Hour, true))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGuilds(w http.ResponseWriter, r *http.Request, sess session) {
	guilds, err := s.svc.UserGuilds(r.Context(), sess.AccessToken)
	if err != nil {
		writeServiceError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Guilds []UserGuildSummary `json:"guilds"`
	}{Guilds: guilds})
}

func (s *Server) handleGuildDashboard(w http.ResponseWriter, r *http.Request, sess session) {
	guildIDRaw := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	guildID, err := strconv.ParseUint(guildIDRaw, 10, 64)
	if err != nil || guildID == 0 {
		writeError(w, http.StatusBadRequest, "invalid guild_id")
		return
	}
	dashboard, err := s.svc.GuildDashboard(r.Context(), sess.AccessToken, guildID)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) handleInstallStart(w http.ResponseWriter, r *http.Request, sess session) {
	guildIDRaw := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	var (
		url string
		err error
	)
	base := requestBaseURL(r)
	if guildIDRaw == "" {
		url, err = s.svc.InstallURLAnyGuild(base)
	} else {
		guildID, parseErr := strconv.ParseUint(guildIDRaw, 10, 64)
		if parseErr != nil || guildID == 0 {
			writeError(w, http.StatusBadRequest, "invalid guild_id")
			return
		}
		url, err = s.svc.InstallURL(guildID, base)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	attrs := []any{slog.Uint64("actor_id", sess.UserID)}
	if guildIDRaw != "" {
		if guildID, parseErr := strconv.ParseUint(guildIDRaw, 10, 64); parseErr == nil {
			attrs = append(attrs, slog.Uint64("guild_id", guildID))
		}
	}
	s.logger.Info("bot install started", attrs...)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Server) handleInstallCallback(w http.ResponseWriter, r *http.Request, sess session) {
	guildIDRaw := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	guildID, err := strconv.ParseUint(guildIDRaw, 10, 64)
	if err != nil || guildID == 0 {
		// When using the plain "bot install" authorize URL (no redirect_uri),
		// Discord will never hit this endpoint. If someone *does* land here (or
		// a portal redirect was misconfigured), keep it friendly.
		base := s.dashboardBaseURL(r)
		http.Redirect(w, r, strings.TrimRight(base, "/")+"/#/servers", http.StatusFound)
		return
	}
	if _, err := s.svc.GuildDashboard(r.Context(), sess.AccessToken, guildID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	base := s.dashboardBaseURL(r)
	http.Redirect(w, r, strings.TrimRight(base, "/")+"/#/servers/"+guildIDRaw, http.StatusFound)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, _ session) {
	resp, err := s.svc.Status(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleModules(w http.ResponseWriter, _ *http.Request, _ session) {
	writeJSON(w, http.StatusOK, map[string]any{"modules": s.svc.Modules()})
}

func (s *Server) handleSetModule(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		ModuleID string `json:"module_id"`
		Enabled  bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.svc.SetModuleEnabled(r.Context(), req.ModuleID, req.Enabled, sess.UserID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("admin module state updated", slog.Uint64("actor_id", sess.UserID), slog.String("module_id", req.ModuleID), slog.Bool("enabled", req.Enabled))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleResetModule(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		ModuleID string `json:"module_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.svc.ResetModule(r.Context(), req.ModuleID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("admin module reset", slog.Uint64("actor_id", sess.UserID), slog.String("module_id", req.ModuleID))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleReloadModules(w http.ResponseWriter, r *http.Request, sess session) {
	if err := s.svc.ReloadModules(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("admin modules reloaded", slog.Uint64("actor_id", sess.UserID))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request, _ session) {
	plugins, err := s.svc.Plugins()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plugins": plugins})
}

func (s *Server) handleReloadPlugins(w http.ResponseWriter, r *http.Request, sess session) {
	if err := s.svc.ReloadPlugins(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("admin plugins reloaded", slog.Uint64("actor_id", sess.UserID))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleScaffoldPlugin(w http.ResponseWriter, r *http.Request, sess session) {
	var req PluginScaffoldRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.svc.ScaffoldPlugin(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("admin plugin scaffolded", slog.Uint64("actor_id", sess.UserID), slog.String("plugin_id", resp.ID))
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSignPlugin(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		PluginID string `json:"plugin_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	path, err := s.svc.SignPlugin(req.PluginID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("admin plugin signed", slog.Uint64("actor_id", sess.UserID), slog.String("plugin_id", req.PluginID))
	writeJSON(w, http.StatusOK, map[string]any{"signature": path})
}

func (s *Server) handleMarketplaceSources(w http.ResponseWriter, r *http.Request, _ session) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.svc.MarketplaceSources(r.Context())
		if err != nil {
			writeServiceError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, MarketplaceSourcesResponse{Sources: items})
	case http.MethodPost:
		var req marketplace.SourceUpsert
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err := s.svc.UpsertMarketplaceSource(r.Context(), req)
		if err != nil {
			writeServiceError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		sourceID := strings.TrimSpace(r.URL.Query().Get("source_id"))
		if sourceID == "" {
			writeError(w, http.StatusBadRequest, "source_id is required")
			return
		}
		if err := s.svc.DeleteMarketplaceSource(r.Context(), sourceID); err != nil {
			writeServiceError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMarketplaceSourceSync(w http.ResponseWriter, r *http.Request, _ session) {
	var req struct {
		SourceID string `json:"source_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.svc.SyncMarketplaceSource(r.Context(), req.SourceID)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMarketplaceSearch(w http.ResponseWriter, r *http.Request, _ session) {
	query := marketplace.SearchQuery{
		SourceID: strings.TrimSpace(r.URL.Query().Get("source_id")),
		Term:     strings.TrimSpace(r.URL.Query().Get("term")),
		Refresh:  strings.TrimSpace(r.URL.Query().Get("refresh")) == "1",
	}
	results, err := s.svc.SearchMarketplace(r.Context(), query)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleMarketplaceInstall(w http.ResponseWriter, r *http.Request, sess session) {
	var req MarketplaceInstallRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.svc.InstallMarketplacePlugin(r.Context(), sess.UserID, req)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMarketplaceUpdate(w http.ResponseWriter, r *http.Request, sess session) {
	var req MarketplaceUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.svc.UpdateMarketplacePlugin(r.Context(), sess.UserID, req)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMarketplaceUninstall(w http.ResponseWriter, r *http.Request, _ session) {
	var req MarketplaceUninstallRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.svc.UninstallMarketplacePlugin(r.Context(), req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMarketplaceTrustSigner(w http.ResponseWriter, r *http.Request, _ session) {
	var req MarketplaceTrustSignerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.svc.TrustMarketplaceSigner(r.Context(), req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMarketplaceTrustVendor(w http.ResponseWriter, r *http.Request, _ session) {
	var req MarketplaceTrustVendorRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.svc.TrustMarketplaceVendor(r.Context(), req)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleModulesConfig(w http.ResponseWriter, r *http.Request, sess session) {
	switch r.Method {
	case http.MethodGet:
		file, err := s.svc.LoadModulesConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, file)
	case http.MethodPut:
		var file config.ModulesFile
		if err := decodeJSON(r, &file); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.svc.SaveModulesConfig(file); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.logger.Info("admin modules config updated", slog.Uint64("actor_id", sess.UserID))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePermissionsConfig(w http.ResponseWriter, r *http.Request, sess session) {
	switch r.Method {
	case http.MethodGet:
		file, err := s.svc.LoadPermissionsConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, file)
	case http.MethodPut:
		var file permissions.Policy
		if err := decodeJSON(r, &file); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.svc.SavePermissionsConfig(file); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.logger.Info("admin permissions config updated", slog.Uint64("actor_id", sess.UserID))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTrustedKeys(w http.ResponseWriter, r *http.Request, _ session) {
	resp, err := s.svc.TrustedKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMigrationStatus(w http.ResponseWriter, r *http.Request, _ session) {
	status, err := s.svc.MigrationStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleMigrationBackup(w http.ResponseWriter, r *http.Request, sess session) {
	path, err := s.svc.BackupMigrations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("admin migrations backup created", slog.Uint64("actor_id", sess.UserID), slog.String("path", path))
	writeJSON(w, http.StatusOK, map[string]any{"path": path})
}

func (s *Server) handleMigrationUp(w http.ResponseWriter, r *http.Request, sess session) {
	status, err := s.svc.MigrateUp(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("admin migrations applied", slog.Uint64("actor_id", sess.UserID), slog.Int("version", status.CurrentVersion))
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) oauthRedirectURL() string {
	return ""
}

func (s *Server) putSession(ctx context.Context, sess session) error {
	if s.sessions == nil {
		return errors.New("session store is not configured")
	}
	return s.sessions.PutAdminSession(ctx, store.AdminSession{
		ID:          sess.ID,
		UserID:      sess.UserID,
		Username:    sess.Username,
		Name:        sess.Name,
		AvatarURL:   sess.AvatarURL,
		CSRFToken:   sess.CSRFToken,
		AccessToken: sess.AccessToken,
		IsOwner:     sess.IsOwner,
		ExpiresAt:   sess.ExpiresAt,
	})
}

func (s *Server) readSession(r *http.Request) (session, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return session{}, err
	}
	sessionID, ok := s.unsignCookieValue(sessionCookieName, cookie.Value)
	if !ok {
		return session{}, errors.New("invalid session")
	}
	if s.sessions == nil {
		return session{}, errors.New("invalid session")
	}
	stored, ok, err := s.sessions.GetAdminSession(r.Context(), sessionID)
	if err != nil {
		return session{}, err
	}
	if !ok {
		return session{}, errors.New("invalid session")
	}
	if time.Now().Unix() >= stored.ExpiresAt {
		_, _ = s.sessions.DeleteExpiredAdminSessions(r.Context(), time.Now().Unix())
		return session{}, errors.New("session expired")
	}
	return session{
		ID:          stored.ID,
		UserID:      stored.UserID,
		Username:    stored.Username,
		Name:        stored.Name,
		AvatarURL:   stored.AvatarURL,
		CSRFToken:   stored.CSRFToken,
		AccessToken: stored.AccessToken,
		IsOwner:     stored.IsOwner,
		ExpiresAt:   stored.ExpiresAt,
	}, nil
}

func (s *Server) deleteSession(ctx context.Context, id string) error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.DeleteAdminSession(ctx, id)
}

func (s *Server) cookie(r *http.Request, name, value string, ttl time.Duration, httpOnly bool) *http.Cookie {
	if ttl > 0 && value != "" && (name == sessionCookieName || name == stateCookieName) {
		value = s.signCookieValue(name, value)
	}
	secure, sameSite := cookiePolicyFromRequest(r)
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: httpOnly,
		SameSite: sameSite,
		Secure:   secure,
		MaxAge:   int(ttl.Seconds()),
	}
}

func (s *Server) currentOwnerStatus() OwnerStatus {
	if s == nil || s.ownerStatus == nil {
		return OwnerStatus{Source: "unresolved"}
	}
	return s.ownerStatus()
}

func (s *Server) isOwnerUser(userID uint64) bool {
	status := s.currentOwnerStatus()
	if !status.Resolved || status.EffectiveUserID == nil {
		return false
	}
	return *status.EffectiveUserID == userID
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func subtleTokenCompare(a, b string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	return hmac.Equal([]byte(a), []byte(b))
}

func (s *Server) authConfigured() bool {
	// Keep this strict: in production we want missing config to be obvious.
	if strings.TrimSpace(s.clientID) == "" || strings.TrimSpace(s.clientSecret) == "" {
		return false
	}
	if len(s.secret) < 32 {
		return false
	}
	return true
}

func (s *Server) signCookieValue(name, value string) string {
	if len(s.secret) == 0 {
		return value
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(name))
	_, _ = mac.Write([]byte{':'})
	_, _ = mac.Write([]byte(value))
	sum := mac.Sum(nil)
	// Keep cookie value compact: 16 bytes is plenty for tamper detection here.
	sig := base64.RawURLEncoding.EncodeToString(sum[:16])
	return value + "." + sig
}

func (s *Server) unsignCookieValue(name, signed string) (string, bool) {
	if len(s.secret) == 0 {
		return "", false
	}
	parts := strings.Split(signed, ".")
	if len(parts) != 2 {
		return "", false
	}
	value := parts[0]
	want := s.signCookieValue(name, value)
	return value, subtleTokenCompare(want, signed)
}

func avatarURL(user OAuthUser) string {
	if strings.TrimSpace(user.Avatar) == "" || strings.TrimSpace(user.ID) == "" {
		return ""
	}
	return "https://cdn.discordapp.com/avatars/" + strings.TrimSpace(user.ID) + "/" + strings.TrimSpace(user.Avatar) + ".png"
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": strings.TrimSpace(message)})
}

func writeServiceError(w http.ResponseWriter, fallbackStatus int, err error) {
	if err == nil {
		return
	}
	if pe, ok := asPublicError(err); ok {
		status := pe.statusCode()
		payload := map[string]any{"error": strings.TrimSpace(pe.Message)}
		if pe.RetryAfter > 0 {
			retrySeconds := int64(pe.RetryAfter.Round(time.Second).Seconds())
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", strconv.FormatInt(retrySeconds, 10))
			payload["retry_after_ms"] = int64(pe.RetryAfter.Round(time.Millisecond) / time.Millisecond)
		}
		writeJSON(w, status, payload)
		return
	}
	writeError(w, fallbackStatus, err.Error())
}

func cookiePolicyFromRequest(r *http.Request) (bool, http.SameSite) {
	base := requestBaseURL(r)
	u, err := url.Parse(base)
	if err != nil {
		return false, http.SameSiteLaxMode
	}
	secure := strings.EqualFold(strings.TrimSpace(u.Scheme), "https")
	if secure {
		return true, http.SameSiteLaxMode
	}
	return false, http.SameSiteLaxMode
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

type memorySessionStore struct {
	mu       sync.Mutex
	sessions map[string]store.AdminSession
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{sessions: map[string]store.AdminSession{}}
}

func (s *memorySessionStore) GetAdminSession(_ context.Context, id string) (store.AdminSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	return sess, ok, nil
}

func (s *memorySessionStore) PutAdminSession(_ context.Context, sess store.AdminSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
	return nil
}

func (s *memorySessionStore) DeleteAdminSession(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

func (s *memorySessionStore) DeleteExpiredAdminSessions(_ context.Context, nowUnix int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for id, sess := range s.sessions {
		if sess.ExpiresAt <= nowUnix {
			delete(s.sessions, id)
			n++
		}
	}
	return n, nil
}

func requestBaseURL(r *http.Request) string {
	if r == nil {
		return "http://127.0.0.1:8081"
	}
	// Trust reverse proxy headers when present.
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		host = "127.0.0.1:8081"
	}
	return scheme + "://" + host
}

func baseURLFromListenAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil || strings.TrimSpace(port) == "" {
		return ""
	}
	switch strings.TrimSpace(host) {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return "http://" + host + ":" + strings.TrimSpace(port)
}

func (s *Server) publicBaseURL(r *http.Request) string {
	if s != nil {
		if v := normalizeOrigin(s.svc.Config.PublicAPIOrigin); v != "" {
			return v
		}
	}

	// Development: keep OAuth redirects stable (always the admin listen addr),
	// even if the dashboard is accessed through a different local origin.
	if s != nil {
		if base := baseURLFromListenAddr(s.addr); base != "" {
			// Avoid localhost vs 127.0.0.1 cookie mismatches: if the user is
			// browsing via one local hostname, keep redirects on that hostname too.
			reqBase := requestBaseURL(r)
			reqURL, _ := url.Parse(reqBase)
			baseURL, _ := url.Parse(base)
			if reqURL != nil && baseURL != nil {
				reqHost := strings.ToLower(strings.TrimSpace(reqURL.Hostname()))
				baseHost := strings.ToLower(strings.TrimSpace(baseURL.Hostname()))
				if isLocalHostname(reqHost) && isLocalHostname(baseHost) && reqHost != baseHost {
					baseURL.Host = reqHost + ":" + baseURL.Port()
					return baseURL.String()
				}
			}
			return base
		}
	}
	return requestBaseURL(r)
}

func (s *Server) apiBaseURL(r *http.Request) string {
	if s != nil {
		if v := normalizeOrigin(s.svc.Config.PublicAPIOrigin); v != "" {
			return v
		}
	}
	// Fallback: use the admin server "public" base (dev stable listen addr).
	return s.publicBaseURL(r)
}

func (s *Server) dashboardBaseURL(r *http.Request) string {
	if s != nil {
		if v := normalizeOrigin(s.svc.Config.PublicDashboardOrigin); v != "" {
			return v
		}
	}
	// Fallback: in dev the dashboard is served from the admin API origin.
	return requestBaseURL(r)
}

func isLocalHostname(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "127.0.0.1", "localhost":
		return true
	default:
		return false
	}
}

func (s *Server) dashboardHandler() http.Handler {
	// If dist exists, prefer it (single-process dev works after `bun run build`).
	dist := filepath.Join("apps", "dashboard", "dist")
	if fileExists(filepath.Join(dist, "index.html")) {
		return http.FileServer(http.Dir(dist))
	}

	// Otherwise, proxy to Vite for dev HMR. Browser still stays on this origin.
	targetURL, _ := url.Parse("http://127.0.0.1:5173")
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		s.logger.Error("dashboard proxy failed", slog.String("err", err.Error()))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("Dashboard dev server is not running.\n\nRun:\n  cd apps/dashboard && bun run dev\n\nOr build once:\n  cd apps/dashboard && bun run build\n"))
	}
	return proxy
}
