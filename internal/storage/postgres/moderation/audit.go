package moderationpg

import (
	"context"
	"database/sql"
	"fmt"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
	"github.com/xsyetopz/go-mamacord/internal/storage/postgres/internal/sqlvalue"
	"time"
)

type auditStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s auditStore) Append(ctx context.Context, entry storage.AuditEntry) error {
	const query = `
INSERT INTO audit_log(guild_id, actor_id, action, target_type, target_id, created_at, meta_json)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.now().UTC()
	}

	guildID, err := sqlvalue.OptionalUint64(entry.GuildID, "guild_id")
	if err != nil {
		return err
	}
	actorID, err := sqlvalue.OptionalUint64(entry.ActorID, "actor_id")
	if err != nil {
		return err
	}
	targetID, err := sqlvalue.OptionalUint64(entry.TargetID, "target_id")
	if err != nil {
		return err
	}

	if _, err := s.db.ExecContext(
		ctx,
		query,
		guildID,
		actorID,
		entry.Action,
		targetTypePtr(entry.TargetType),
		targetID,
		createdAt.Unix(),
		emptyObjectIfBlank(entry.MetaJSON),
	); err != nil {
		return fmt.Errorf("append audit log: %w", err)
	}
	return nil
}

func targetTypePtr(v *storage.TargetType) any {
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
