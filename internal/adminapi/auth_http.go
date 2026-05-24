package adminapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

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
		if !subtleTokenCompare(r.Header.Get("X-CSRF-Token"), sess.CSRFToken) {
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
	s.stateStore[state] = oauthState{RedirectURL: redirectURL, ReturnBase: returnBase}
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
		writeJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
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
