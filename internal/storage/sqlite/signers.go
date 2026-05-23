package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
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
		var keyID string
		var publicKeyB64 string
		var addedAt int64
		if scanErr := rows.Scan(&keyID, &publicKeyB64, &addedAt); scanErr != nil {
			return nil, fmt.Errorf("scan trusted signer: %w", scanErr)
		}

		out = append(out, store.TrustedSigner{
			KeyID:        keyID,
			PublicKeyB64: publicKeyB64,
			AddedAt:      time.Unix(addedAt, 0).UTC(),
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate trusted signers: %w", rowsErr)
	}

	return out, nil
}

func (s signerStore) PutTrustedSigner(ctx context.Context, signer store.TrustedSigner) error {
	addedAt := signer.AddedAt
	if addedAt.IsZero() {
		addedAt = s.now()
	}

	const query = `
INSERT INTO trusted_signers(key_id, public_key_b64, added_at)
VALUES (?, ?, ?)
ON CONFLICT(key_id) DO UPDATE SET
	public_key_b64 = excluded.public_key_b64,
	added_at = excluded.added_at`

	_, err := s.db.ExecContext(ctx, query, signer.KeyID, signer.PublicKeyB64, addedAt.Unix())
	if err != nil {
		return fmt.Errorf("put trusted signer: %w", err)
	}
	return nil
}

func (s signerStore) DeleteTrustedSigner(ctx context.Context, keyID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM trusted_signers WHERE key_id = ?", keyID)
	if err != nil {
		return fmt.Errorf("delete trusted signer: %w", err)
	}
	return nil
}
