package sqlitestore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	migrate "github.com/xsyetopz/go-mamacord/internal/migration"
	"github.com/xsyetopz/go-mamacord/internal/sqlite"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
	sqlitestore "github.com/xsyetopz/go-mamacord/internal/storage/sqlite"
)

func mustNoErr(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

func TestSQLitePersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()

	db, err := sqlite.Open(ctx, sqlite.Options{Path: filepath.Join(dir, "test.sqlite")})
	mustNoErr(t, err, "sqlite.Open")
	t.Cleanup(func() { _ = db.Close() })

	migrationsDir := filepath.Clean(filepath.Join("..", "..", "..", "migrations", "sqlite"))
	runner, err := migrate.New(migrate.Options{Dir: migrationsDir, BackupDir: filepath.Join(dir, "migration_backups")})
	mustNoErr(t, err, "migrate.New")
	_, err = runner.Up(ctx, db)
	mustNoErr(t, err, "runner.Up")

	s, err := sqlitestore.New(db)
	mustNoErr(t, err, "sqlitestore.New")

	now := time.Unix(1700000000, 0).UTC()

	mustNoErr(t, s.Users().UpsertUserSeen(ctx, store.UserSeen{
		UserID:      1,
		CreatedAt:   time.Unix(1600000000, 0).UTC(),
		IsBot:       false,
		IsSystem:    false,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}), "UpsertUserSeen")
	mustNoErr(t, s.Users().TouchUserSeen(ctx, 1, now.Add(10*time.Second)), "TouchUserSeen")

	mustNoErr(t, s.Guilds().UpsertGuildSeen(ctx, store.GuildSeen{
		GuildID:   10,
		OwnerID:   2,
		CreatedAt: time.Unix(1500000000, 0).UTC(),
		JoinedAt:  now,
		LeftAt:    nil,
		Name:      "x",
		UpdatedAt: now,
	}), "UpsertGuildSeen")
	mustNoErr(t, s.Guilds().MarkGuildLeft(ctx, 10, now.Add(5*time.Second)), "MarkGuildLeft")
	mustNoErr(t, s.Guilds().UpdateGuildOwner(ctx, 10, 3, now.Add(6*time.Second)), "UpdateGuildOwner")

	mustNoErr(t, s.GuildMembers().MarkMemberJoined(ctx, 10, 1, now), "MarkMemberJoined")
	mustNoErr(t, s.GuildMembers().MarkMemberLeft(ctx, 10, 1, now.Add(1*time.Second)), "MarkMemberLeft")
}
