package adminapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

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
