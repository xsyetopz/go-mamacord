package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type auditStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s auditStore) Append(ctx context.Context, entry store.AuditEntry) error {
	const query = `
INSERT INTO audit_log(guild_id, actor_id, action, target_type, target_id, created_at, meta_json)
VALUES (?, ?, ?, ?, ?, ?, ?)`

	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.now()
	}

	guildID, err := toAnyInt64Ptr(entry.GuildID, "guild_id")
	if err != nil {
		return err
	}
	actorID, err := toAnyInt64Ptr(entry.ActorID, "actor_id")
	if err != nil {
		return err
	}
	targetID, err := toAnyInt64Ptr(entry.TargetID, "target_id")
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(
		ctx,
		query,
		guildID,
		actorID,
		entry.Action,
		targetTypePtr(entry.TargetType),
		targetID,
		createdAt.Unix(),
		emptyObjectIfBlank(entry.MetaJSON),
	)
	if err != nil {
		return fmt.Errorf("append audit log: %w", err)
	}
	return nil
}

func targetTypePtr(v *store.TargetType) any {
	if v == nil {
		return nil
	}
	return string(*v)
}

func emptyObjectIfBlank(jsonText string) string {
	if jsonText == "" {
		return "{}"
	}
	return jsonText
}
