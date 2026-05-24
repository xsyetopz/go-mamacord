package postgresstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type guildStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s guildStore) UpsertGuildSeen(ctx context.Context, g store.GuildSeen) error {
	now := s.now().UTC()
	createdAt := g.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	joinedAt := g.JoinedAt
	if joinedAt.IsZero() {
		joinedAt = now
	}
	updatedAt := g.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}

	var leftAt any
	if g.LeftAt != nil && !g.LeftAt.IsZero() {
		leftAt = g.LeftAt.Unix()
	}

	const query = `
INSERT INTO guilds(guild_id, owner_id, created_at, joined_at, left_at, name, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT(guild_id) DO UPDATE SET
	owner_id = excluded.owner_id,
	joined_at = CASE
		WHEN excluded.left_at IS NULL AND guilds.left_at IS NOT NULL THEN excluded.joined_at
		ELSE guilds.joined_at
	END,
	left_at = excluded.left_at,
	name = excluded.name,
	updated_at = excluded.updated_at`

	guildIDDB, err := toInt64(g.GuildID, "guild_id")
	if err != nil {
		return err
	}
	ownerIDDB, err := toInt64(g.OwnerID, "owner_id")
	if err != nil {
		return err
	}

	if _, err := s.db.ExecContext(
		ctx,
		query,
		guildIDDB,
		ownerIDDB,
		createdAt.Unix(),
		joinedAt.Unix(),
		leftAt,
		g.Name,
		updatedAt.Unix(),
	); err != nil {
		return fmt.Errorf("upsert guild: %w", err)
	}
	return nil
}

func (s guildStore) MarkGuildLeft(ctx context.Context, guildID uint64, leftAt time.Time) error {
	if leftAt.IsZero() {
		leftAt = s.now().UTC()
	}
	updatedAt := s.now().UTC()

	const query = `UPDATE guilds SET left_at = $1, updated_at = $2 WHERE guild_id = $3`

	guildIDDB, err := toInt64(guildID, "guild_id")
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, query, leftAt.Unix(), updatedAt.Unix(), guildIDDB); err != nil {
		return fmt.Errorf("mark guild left: %w", err)
	}
	return nil
}

func (s guildStore) UpdateGuildOwner(ctx context.Context, guildID uint64, ownerID uint64, updatedAt time.Time) error {
	if updatedAt.IsZero() {
		updatedAt = s.now().UTC()
	}
	const query = `UPDATE guilds SET owner_id = $1, updated_at = $2 WHERE guild_id = $3`

	guildIDDB, err := toInt64(guildID, "guild_id")
	if err != nil {
		return err
	}
	ownerIDDB, err := toInt64(ownerID, "owner_id")
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, query, ownerIDDB, updatedAt.Unix(), guildIDDB); err != nil {
		return fmt.Errorf("update guild owner: %w", err)
	}
	return nil
}
