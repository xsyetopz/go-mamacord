package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
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

type discordOAuthClient struct {
	client       *http.Client
	clientID     string
	clientSecret string
}

// OAuthRateLimitError represents a Discord OAuth API rate-limit response.
// It is returned when Discord responds with HTTP 429 and provides a retry delay.
type RequestError struct {
	Status  int
	Message string
}

func (e *RequestError) Error() string { return e.Message }
func (e *RequestError) StatusCode() int {
	if e == nil || e.Status == 0 {
		return http.StatusBadRequest
	}
	return e.Status
}
func (e *RequestError) PublicMessage() string {
	if e == nil {
		return ""
	}
	return e.Message
}
func (e *RequestError) RetryDelay() time.Duration { return 0 }

type OAuthRateLimitError struct {
	RetryAfter time.Duration
	Global     bool
}

func (e *OAuthRateLimitError) Error() string {
	retry := e.RetryAfter
	if retry < 0 {
		retry = 0
	}
	return fmt.Sprintf("discord oauth rate limited (retry_after=%s, global=%t)", retry.Truncate(time.Millisecond), e.Global)
}

func (e *OAuthRateLimitError) Is(target error) bool {
	_, ok := target.(*OAuthRateLimitError)
	return ok
}

type OAuthToken struct {
	AccessToken string
	TokenType   string
	Scope       string
}

// OAuthPermissions is a defensive wrapper around the `permissions` value returned
// by Discord in the `/users/@me/guilds` response. Discord typically returns it
// as a string, but some clients/environments may surface it as a JSON number.
//
// We normalize to a decimal string so downstream permission parsing stays the
// same.
type OAuthPermissions string

func (p *OAuthPermissions) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*p = ""
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*p = OAuthPermissions(strings.TrimSpace(s))
		return nil
	}

	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*p = OAuthPermissions(strings.TrimSpace(n.String()))
		return nil
	}

	return fmt.Errorf("invalid oauth guild permissions %q", strings.TrimSpace(string(data)))
}

type OAuthGuild struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Icon        string           `json:"icon"`
	Owner       bool             `json:"owner"`
	Permissions OAuthPermissions `json:"permissions"`
}

func NewDiscordOAuthClient(clientID, clientSecret string) OAuthClient {
	return &discordOAuthClient{
		client:       &http.Client{Timeout: 10 * time.Second},
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
	}
}

func (c *discordOAuthClient) ExchangeCode(ctx context.Context, code string, redirectURL string) (OAuthToken, error) {
	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", strings.TrimSpace(code))
	form.Set("redirect_uri", strings.TrimSpace(redirectURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://discord.com/api/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return OAuthToken{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return OAuthToken{}, fmt.Errorf("discord oauth token exchange failed: %s", strings.TrimSpace(string(body)))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return OAuthToken{}, err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return OAuthToken{}, fmt.Errorf("discord oauth token response missing access_token")
	}
	return OAuthToken{
		AccessToken: strings.TrimSpace(payload.AccessToken),
		TokenType:   strings.TrimSpace(payload.TokenType),
		Scope:       strings.TrimSpace(payload.Scope),
	}, nil
}

func (c *discordOAuthClient) FetchUser(ctx context.Context, accessToken string) (OAuthUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/users/@me", nil)
	if err != nil {
		return OAuthUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	resp, err := c.client.Do(req)
	if err != nil {
		return OAuthUser{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return OAuthUser{}, fmt.Errorf("discord oauth user lookup failed: %s", strings.TrimSpace(string(body)))
	}
	var user OAuthUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return OAuthUser{}, err
	}
	return user, nil
}

func (c *discordOAuthClient) FetchGuilds(ctx context.Context, accessToken string) ([]OAuthGuild, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/users/@me/guilds", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		var payload struct {
			Message    string  `json:"message"`
			RetryAfter float64 `json:"retry_after"`
			Global     bool    `json:"global"`
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = json.Unmarshal(body, &payload)
		// Discord sends retry_after in seconds (often as a float).
		retry := time.Duration(payload.RetryAfter * float64(time.Second))
		if retry <= 0 {
			retry = time.Second
		}
		// Keep the raw body out of the public surface; callers can special-case
		// this typed error and show a friendly message.
		return nil, &OAuthRateLimitError{RetryAfter: retry, Global: payload.Global}
	}
	if resp.StatusCode/100 != 2 {
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			// Don't leak Discord JSON bodies to the dashboard.
			return nil, &RequestError{
				Status:  http.StatusUnauthorized,
				Message: "Discord sign-in expired or was revoked. Please sign in again.",
			}
		default:
			return nil, &RequestError{
				Status:  http.StatusBadGateway,
				Message: "Discord API request failed. Please try again in a moment.",
			}
		}
	}
	var guilds []OAuthGuild
	if err := json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
		return nil, err
	}
	return guilds, nil
}

func AsRateLimit(err error) (*OAuthRateLimitError, bool) {
	var rl *OAuthRateLimitError
	if errors.As(err, &rl) {
		return rl, true
	}
	return nil, false
}
