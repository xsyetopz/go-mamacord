package adminapi

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
)

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
	if isLocalHostname(host) {
		return true
	}

	return false
}

func requestBaseURL(r *http.Request) string {
	if r == nil {
		return "http://127.0.0.1:8081"
	}
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

	if s != nil {
		if base := baseURLFromListenAddr(s.addr); base != "" {
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
	return s.publicBaseURL(r)
}

func (s *Server) dashboardBaseURL(r *http.Request) string {
	if s != nil {
		if v := normalizeOrigin(s.svc.Config.PublicDashboardOrigin); v != "" {
			return v
		}
	}
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
	dist := filepath.Join("apps", "dashboard", "dist")
	if fileExists(filepath.Join(dist, "index.html")) {
		return http.FileServer(http.Dir(dist))
	}

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
