package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type discordOAuthTokenStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s discordOAuthTokenStore) GetDiscordOAuthToken(ctx context.Context, userID uint64) (store.DiscordOAuthToken, bool, error) {
	if s.db == nil {
		return store.DiscordOAuthToken{}, false, errors.New("db unavailable")
	}
	if userID == 0 {
		return store.DiscordOAuthToken{}, false, errors.New("invalid user id")
	}

	row := s.db.QueryRowContext(
		ctx,
		`SELECT user_id, access_token_enc, refresh_token_enc, scope, expires_at, updated_at
		 FROM discord_oauth_tokens
		 WHERE user_id = ?`,
		userID,
	)
	var token store.DiscordOAuthToken
	var expiresAt int64
	var updatedAt int64
	if err := row.Scan(
		&token.UserID,
		&token.AccessTokenEnc,
		&token.RefreshTokenEnc,
		&token.Scope,
		&expiresAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.DiscordOAuthToken{}, false, nil
		}
		return store.DiscordOAuthToken{}, false, err
	}
	token.ExpiresAt = time.Unix(expiresAt, 0).UTC()
	token.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return token, true, nil
}

func (s discordOAuthTokenStore) PutDiscordOAuthToken(ctx context.Context, token store.DiscordOAuthToken) error {
	if s.db == nil {
		return errors.New("db unavailable")
	}
	if token.UserID == 0 {
		return errors.New("invalid user id")
	}
	if strings.TrimSpace(token.AccessTokenEnc) == "" || strings.TrimSpace(token.RefreshTokenEnc) == "" {
		return errors.New("oauth token is missing encrypted values")
	}
	scope := strings.TrimSpace(token.Scope)
	if scope == "" {
		return errors.New("oauth token scope is required")
	}

	expiresAt := token.ExpiresAt.UTC().Unix()
	if expiresAt <= 0 {
		return errors.New("oauth token expires_at is required")
	}
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO discord_oauth_tokens(user_id, access_token_enc, refresh_token_enc, scope, expires_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   access_token_enc=excluded.access_token_enc,
		   refresh_token_enc=excluded.refresh_token_enc,
		   scope=excluded.scope,
		   expires_at=excluded.expires_at,
		   updated_at=excluded.updated_at`,
		token.UserID,
		strings.TrimSpace(token.AccessTokenEnc),
		strings.TrimSpace(token.RefreshTokenEnc),
		scope,
		expiresAt,
		now.Unix(),
	)
	if err != nil {
		return fmt.Errorf("put discord oauth token: %w", err)
	}
	return nil
}

func (s discordOAuthTokenStore) DeleteDiscordOAuthToken(ctx context.Context, userID uint64) error {
	if s.db == nil {
		return errors.New("db unavailable")
	}
	if userID == 0 {
		return errors.New("invalid user id")
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM discord_oauth_tokens WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("delete discord oauth token: %w", err)
	}
	return nil
}
