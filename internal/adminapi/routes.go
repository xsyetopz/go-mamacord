package adminapi

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
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
	api.HandleFunc("/api/guilds", s.withAuth(s.guilds.HandleGuilds))
	api.HandleFunc("/api/guilds/dashboard", s.withAuth(s.guilds.HandleGuildDashboard))
	api.HandleFunc("/api/guilds/config", s.withAuth(s.withCSRF(s.guilds.HandleGuildConfig)))
	api.HandleFunc("/api/guilds/channels", s.withAuth(s.guilds.HandleGuildChannels))
	api.HandleFunc("/api/guilds/roles", s.withAuth(s.guilds.HandleGuildRoles))
	api.HandleFunc("/api/guilds/members", s.withAuth(s.guilds.HandleGuildMembers))
	api.HandleFunc("/api/guilds/emojis", s.withAuth(s.guilds.HandleGuildEmojis))
	api.HandleFunc("/api/guilds/stickers", s.withAuth(s.guilds.HandleGuildStickers))
	api.HandleFunc("/api/guilds/moderation/warnings", s.withAuth(s.guilds.HandleGuildWarnings))
	api.HandleFunc("/api/guilds/moderation/warn", s.withAuth(s.withCSRF(s.guilds.HandleGuildWarn)))
	api.HandleFunc("/api/guilds/moderation/unwarn", s.withAuth(s.withCSRF(s.guilds.HandleGuildUnwarn)))
	api.HandleFunc("/api/guilds/manager/slowmode", s.withAuth(s.withCSRF(s.guilds.HandleGuildSlowmode)))
	api.HandleFunc("/api/guilds/manager/nick", s.withAuth(s.withCSRF(s.guilds.HandleGuildNickname)))
	api.HandleFunc("/api/guilds/manager/roles/create", s.withAuth(s.withCSRF(s.guilds.HandleGuildRoleCreate)))
	api.HandleFunc("/api/guilds/manager/roles/edit", s.withAuth(s.withCSRF(s.guilds.HandleGuildRoleEdit)))
	api.HandleFunc("/api/guilds/manager/roles/delete", s.withAuth(s.withCSRF(s.guilds.HandleGuildRoleDelete)))
	api.HandleFunc("/api/guilds/manager/roles/member", s.withAuth(s.withCSRF(s.guilds.HandleGuildRoleMember)))
	api.HandleFunc("/api/guilds/manager/purge", s.withAuth(s.withCSRF(s.guilds.HandleGuildPurge)))
	api.HandleFunc("/api/guilds/manager/emojis/create", s.withAuth(s.withCSRF(s.guilds.HandleGuildEmojiCreate)))
	api.HandleFunc("/api/guilds/manager/emojis/edit", s.withAuth(s.withCSRF(s.guilds.HandleGuildEmojiEdit)))
	api.HandleFunc("/api/guilds/manager/emojis/delete", s.withAuth(s.withCSRF(s.guilds.HandleGuildEmojiDelete)))
	api.HandleFunc("/api/guilds/manager/stickers/create", s.withAuth(s.withCSRF(s.guilds.HandleGuildStickerCreate)))
	api.HandleFunc("/api/guilds/manager/stickers/edit", s.withAuth(s.withCSRF(s.guilds.HandleGuildStickerEdit)))
	api.HandleFunc("/api/guilds/manager/stickers/delete", s.withAuth(s.withCSRF(s.guilds.HandleGuildStickerDelete)))
	api.HandleFunc("/api/install/start", s.withAuth(s.guilds.HandleInstallStart))
	api.HandleFunc("/api/install/callback", s.withAuth(s.guilds.HandleInstallCallback))

	api.HandleFunc("/api/owner/status", s.withOwner(s.status.HandleStatus))
	api.HandleFunc("/api/owner/modules", s.withOwner(s.plugins.HandleModules))
	api.HandleFunc("/api/owner/modules/set", s.withOwner(s.withCSRF(s.plugins.HandleSetModule)))
	api.HandleFunc("/api/owner/modules/reset", s.withOwner(s.withCSRF(s.plugins.HandleResetModule)))
	api.HandleFunc("/api/owner/modules/reload", s.withOwner(s.withCSRF(s.plugins.HandleReloadModules)))

	api.HandleFunc("/api/owner/plugins", s.withOwner(s.plugins.HandlePlugins))
	api.HandleFunc("/api/owner/plugins/reload", s.withOwner(s.withCSRF(s.plugins.HandleReloadPlugins)))
	api.HandleFunc("/api/owner/plugins/scaffold", s.withOwner(s.withCSRF(s.plugins.HandleScaffoldPlugin)))
	api.HandleFunc("/api/owner/plugins/sign", s.withOwner(s.withCSRF(s.plugins.HandleSignPlugin)))
	api.HandleFunc("/api/owner/plugins/sources", s.withOwner(s.withCSRF(s.plugins.HandleMarketplaceSources)))
	api.HandleFunc("/api/owner/plugins/sources/sync", s.withOwner(s.withCSRF(s.plugins.HandleMarketplaceSourceSync)))
	api.HandleFunc("/api/owner/plugins/search", s.withOwner(s.plugins.HandleMarketplaceSearch))
	api.HandleFunc("/api/owner/plugins/install", s.withOwner(s.withCSRF(s.plugins.HandleMarketplaceInstall)))
	api.HandleFunc("/api/owner/plugins/update", s.withOwner(s.withCSRF(s.plugins.HandleMarketplaceUpdate)))
	api.HandleFunc("/api/owner/plugins/uninstall", s.withOwner(s.withCSRF(s.plugins.HandleMarketplaceUninstall)))
	api.HandleFunc("/api/owner/plugins/trust/signer", s.withOwner(s.withCSRF(s.plugins.HandleMarketplaceTrustSigner)))
	api.HandleFunc("/api/owner/plugins/trust/vendor", s.withOwner(s.withCSRF(s.plugins.HandleMarketplaceTrustVendor)))

	api.HandleFunc("/api/owner/config/modules", s.withOwner(s.status.HandleModulesConfig))
	api.HandleFunc("/api/owner/config/permissions", s.withOwner(s.status.HandlePermissionsConfig))
	api.HandleFunc("/api/owner/config/trusted-keys", s.withOwner(s.status.HandleTrustedKeys))

	api.HandleFunc("/api/owner/migrations/status", s.withOwner(s.status.HandleMigrationStatus))
	api.HandleFunc("/api/owner/migrations/up", s.withOwner(s.withCSRF(s.status.HandleMigrationUp)))

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
	for _, allowed := range s.svc.Config.Dashboard.AllowedOrigins {
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
		if v := normalizeOrigin(s.svc.Config.Dashboard.APIOrigin); v != "" {
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
		if v := normalizeOrigin(s.svc.Config.Dashboard.APIOrigin); v != "" {
			return v
		}
	}
	return s.publicBaseURL(r)
}

func (s *Server) dashboardBaseURL(r *http.Request) string {
	if s != nil {
		if v := normalizeOrigin(s.svc.Config.Dashboard.PublicOrigin); v != "" {
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
	if dashboardFileExists(filepath.Join(dist, "index.html")) {
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

func dashboardFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
