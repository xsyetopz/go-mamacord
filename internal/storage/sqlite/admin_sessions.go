package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
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
		SELECT
			id,
			user_id,
			username,
			name,
			avatar_url,
			csrf_token,
			access_token,
			is_owner,
			expires_at
		FROM admin_sessions
		WHERE id = ?
	`, id)

	var sess store.AdminSession
	var isOwnerInt int
	if err := row.Scan(
		&sess.ID,
		&sess.UserID,
		&sess.Username,
		&sess.Name,
		&sess.AvatarURL,
		&sess.CSRFToken,
		&sess.AccessToken,
		&isOwnerInt,
		&sess.ExpiresAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.AdminSession{}, false, nil
		}
		return store.AdminSession{}, false, err
	}
	sess.IsOwner = isOwnerInt != 0
	return sess, true, nil
}

func (s adminSessionStore) PutAdminSession(ctx context.Context, sess store.AdminSession) error {
	if s.db == nil {
		return errors.New("db is required")
	}
	_, err := s.db.ExecContext(ctx, `
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_id=excluded.user_id,
			username=excluded.username,
			name=excluded.name,
			avatar_url=excluded.avatar_url,
			csrf_token=excluded.csrf_token,
			access_token=excluded.access_token,
			is_owner=excluded.is_owner,
			expires_at=excluded.expires_at
	`, sess.ID, sess.UserID, sess.Username, sess.Name, sess.AvatarURL, sess.CSRFToken, sess.AccessToken, boolToInt(sess.IsOwner), sess.ExpiresAt)
	return err
}

func (s adminSessionStore) DeleteAdminSession(ctx context.Context, id string) error {
	if s.db == nil {
		return errors.New("db is required")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE id = ?`, id)
	return err
}

func (s adminSessionStore) DeleteExpiredAdminSessions(ctx context.Context, nowUnix int64) (int64, error) {
	if s.db == nil {
		return 0, errors.New("db is required")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE expires_at <= ?`, nowUnix)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
