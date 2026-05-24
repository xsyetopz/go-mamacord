package postgresstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type adminSessionStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s adminSessionStore) GetAdminSession(ctx context.Context, id string) (store.AdminSession, bool, error) {
	if s.db == nil {
		return store.AdminSession{}, false, errors.New("db is required")
	}

	row := s.db.QueryRowContext(ctx, `
SELECT id, user_id, username, name, avatar_url, csrf_token, access_token, is_owner, expires_at
FROM admin_sessions
WHERE id = $1
`, id)

	var (
		sess    store.AdminSession
		userID  int64
		isOwner bool
	)
	if err := row.Scan(
		&sess.ID,
		&userID,
		&sess.Username,
		&sess.Name,
		&sess.AvatarURL,
		&sess.CSRFToken,
		&sess.AccessToken,
		&isOwner,
		&sess.ExpiresAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.AdminSession{}, false, nil
		}
		return store.AdminSession{}, false, fmt.Errorf("get admin session: %w", err)
	}

	userIDU64, err := toUint64(userID, "user_id")
	if err != nil {
		return store.AdminSession{}, false, err
	}
	sess.UserID = userIDU64
	sess.IsOwner = isOwner
	return sess, true, nil
}

func (s adminSessionStore) PutAdminSession(ctx context.Context, sess store.AdminSession) error {
	if s.db == nil {
		return errors.New("db is required")
	}

	userID, err := toInt64(sess.UserID, "user_id")
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO admin_sessions (
	id,
	user_id,
	username,
	name,
	avatar_url,
	csrf_token,
	access_token,
	is_owner,
	expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT(id) DO UPDATE SET
	user_id = excluded.user_id,
	username = excluded.username,
	name = excluded.name,
	avatar_url = excluded.avatar_url,
	csrf_token = excluded.csrf_token,
	access_token = excluded.access_token,
	is_owner = excluded.is_owner,
	expires_at = excluded.expires_at
`, sess.ID, userID, sess.Username, sess.Name, sess.AvatarURL, sess.CSRFToken, sess.AccessToken, sess.IsOwner, sess.ExpiresAt)
	if err != nil {
		return fmt.Errorf("put admin session: %w", err)
	}
	return nil
}

func (s adminSessionStore) DeleteAdminSession(ctx context.Context, id string) error {
	if s.db == nil {
		return errors.New("db is required")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete admin session: %w", err)
	}
	return nil
}

func (s adminSessionStore) DeleteExpiredAdminSessions(ctx context.Context, nowUnix int64) (int64, error) {
	if s.db == nil {
		return 0, errors.New("db is required")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE expires_at <= $1`, nowUnix)
	if err != nil {
		return 0, fmt.Errorf("delete expired admin sessions: %w", err)
	}
	count, _ := res.RowsAffected()
	return count, nil
}
