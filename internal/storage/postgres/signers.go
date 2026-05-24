package postgresstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type signerStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s signerStore) ListTrustedSigners(ctx context.Context) ([]store.TrustedSigner, error) {
	const query = `SELECT key_id, public_key_b64, added_at FROM trusted_signers ORDER BY key_id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list trusted signers: %w", err)
	}
	defer rows.Close()

	var out []store.TrustedSigner
	for rows.Next() {
		var (
			keyID        string
			publicKeyB64 string
			addedAt      int64
		)
		if err := rows.Scan(&keyID, &publicKeyB64, &addedAt); err != nil {
			return nil, fmt.Errorf("scan trusted signer: %w", err)
		}
		out = append(out, store.TrustedSigner{
			KeyID:        strings.TrimSpace(keyID),
			PublicKeyB64: strings.TrimSpace(publicKeyB64),
			AddedAt:      time.Unix(addedAt, 0).UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trusted signers: %w", err)
	}
	return out, nil
}

func (s signerStore) PutTrustedSigner(ctx context.Context, signer store.TrustedSigner) error {
	keyID := strings.TrimSpace(signer.KeyID)
	if keyID == "" {
		return fmt.Errorf("key_id is required")
	}
	publicKeyB64 := strings.TrimSpace(signer.PublicKeyB64)
	if publicKeyB64 == "" {
		return fmt.Errorf("public_key_b64 is required")
	}
	addedAt := signer.AddedAt
	if addedAt.IsZero() {
		addedAt = s.now().UTC()
	}

	const query = `
INSERT INTO trusted_signers(key_id, public_key_b64, added_at)
VALUES ($1, $2, $3)
ON CONFLICT(key_id) DO UPDATE SET
	public_key_b64 = excluded.public_key_b64,
	added_at = excluded.added_at`

	if _, err := s.db.ExecContext(ctx, query, keyID, publicKeyB64, addedAt.Unix()); err != nil {
		return fmt.Errorf("put trusted signer: %w", err)
	}
	return nil
}

func (s signerStore) DeleteTrustedSigner(ctx context.Context, keyID string) error {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return fmt.Errorf("key_id is required")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM trusted_signers WHERE key_id = $1`, keyID); err != nil {
		return fmt.Errorf("delete trusted signer: %w", err)
	}
	return nil
}
