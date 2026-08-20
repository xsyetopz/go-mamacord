package postgresstore_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	accountstore "github.com/xsyetopz/go-mamacord/internal/storage/accounts"
	automationstore "github.com/xsyetopz/go-mamacord/internal/storage/automation"
	marketstore "github.com/xsyetopz/go-mamacord/internal/storage/marketplace"
	moderationstore "github.com/xsyetopz/go-mamacord/internal/storage/moderation"
	pluginstore "github.com/xsyetopz/go-mamacord/internal/storage/plugins"
	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
	pgtest "github.com/xsyetopz/go-mamacord/internal/storage/postgres/testkit"
)

func mustNoErr(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

func TestPostgresMetadataPersistenceParity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := pgtest.OpenMigratedDB(t)
	t.Cleanup(func() { _ = db.Close() })

	s, err := postgresstore.New(db)
	if err != nil {
		t.Fatalf("postgresstore.New: %v", err)
	}

	now := time.Unix(1_700_000_000, 0).UTC()
	actorID := uint64(42)
	if err := s.ModuleStates().PutModuleState(ctx, pluginstore.ModuleState{
		ModuleID:  "weather",
		Enabled:   true,
		UpdatedAt: now,
		UpdatedBy: &actorID,
	}); err != nil {
		t.Fatalf("PutModuleState: %v", err)
	}
	state, ok, err := s.ModuleStates().GetModuleState(ctx, "weather")
	if err != nil {
		t.Fatalf("GetModuleState: %v", err)
	}
	if !ok {
		t.Fatal("expected module state to exist")
	}
	if state.ModuleID != "weather" || !state.Enabled || state.UpdatedBy == nil || *state.UpdatedBy != actorID {
		t.Fatalf("unexpected module state: %#v", state)
	}

	if err := s.TrustedSigners().PutTrustedSigner(ctx, marketstore.TrustedSigner{
		KeyID:        "official",
		PublicKeyB64: "pubkey",
		AddedAt:      now,
	}); err != nil {
		t.Fatalf("PutTrustedSigner: %v", err)
	}
	signers, err := s.TrustedSigners().ListTrustedSigners(ctx)
	if err != nil {
		t.Fatalf("ListTrustedSigners: %v", err)
	}
	if len(signers) != 1 || signers[0].KeyID != "official" || signers[0].PublicKeyB64 != "pubkey" {
		t.Fatalf("unexpected trusted signers: %#v", signers)
	}

	source := marketstore.MarketplaceSource{
		SourceID:    "demo",
		Kind:        "git",
		GitURL:      "https://example.invalid/demo.git",
		GitRef:      "main",
		GitSubdir:   "plugins",
		TokenEnvVar: "DEMO_TOKEN",
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now.Add(time.Second),
	}
	if err := s.MarketplaceSources().PutMarketplaceSource(ctx, source); err != nil {
		t.Fatalf("PutMarketplaceSource: %v", err)
	}
	gotSource, ok, err := s.MarketplaceSources().GetMarketplaceSource(ctx, "demo")
	if err != nil {
		t.Fatalf("GetMarketplaceSource: %v", err)
	}
	if !ok || !reflect.DeepEqual(gotSource, source) {
		t.Fatalf("unexpected marketplace source: got %#v want %#v ok=%t", gotSource, source, ok)
	}

	syncedAt := now.Add(2 * time.Second)
	sync := marketstore.MarketplaceSourceSync{
		SourceID:     "demo",
		LastSyncedAt: &syncedAt,
		LastRevision: "abc123",
		LastError:    "none",
	}
	if err := s.MarketplaceSourceSyncs().PutMarketplaceSourceSync(ctx, sync); err != nil {
		t.Fatalf("PutMarketplaceSourceSync: %v", err)
	}
	gotSync, ok, err := s.MarketplaceSourceSyncs().GetMarketplaceSourceSync(ctx, "demo")
	if err != nil {
		t.Fatalf("GetMarketplaceSourceSync: %v", err)
	}
	if !ok || !reflect.DeepEqual(gotSync, sync) {
		t.Fatalf("unexpected marketplace sync: got %#v want %#v ok=%t", gotSync, sync, ok)
	}

	installedBy := uint64(7)
	install := marketstore.PluginInstall{
		PluginID: "weather",
		PluginInstallSource: marketstore.PluginInstallSource{
			InstallKind: "git",
			SourceID:    "demo",
			GitURL:      "https://example.invalid/demo.git",
			GitRef:      "main",
			GitRevision: "abc123",
			SourcePath:  "weather",
		},
		PluginInstallArtifact: marketstore.PluginInstallArtifact{
			BundleRelativeDir: "bundles/git-abc123",
			InstalledHashB64:  "hash",
		},
		InstalledAt: now.Add(3 * time.Second),
		InstalledBy: &installedBy,
	}
	if err := s.PluginInstalls().PutPluginInstall(ctx, install); err != nil {
		t.Fatalf("PutPluginInstall: %v", err)
	}
	gotInstall, ok, err := s.PluginInstalls().GetPluginInstall(ctx, "weather")
	if err != nil {
		t.Fatalf("GetPluginInstall: %v", err)
	}
	if !ok || !reflect.DeepEqual(gotInstall, install) {
		t.Fatalf("unexpected plugin install: got %#v want %#v ok=%t", gotInstall, install, ok)
	}

	vendor := marketstore.TrustedVendor{
		VendorID:   "acme",
		Name:       "Acme",
		WebsiteURL: "https://example.invalid",
		SupportURL: "https://example.invalid/support",
		AddedAt:    now.Add(4 * time.Second),
		UpdatedAt:  now.Add(5 * time.Second),
	}
	if err := s.TrustedVendors().PutTrustedVendor(ctx, vendor); err != nil {
		t.Fatalf("PutTrustedVendor: %v", err)
	}
	gotVendor, ok, err := s.TrustedVendors().GetTrustedVendor(ctx, "acme")
	if err != nil {
		t.Fatalf("GetTrustedVendor: %v", err)
	}
	if !ok || !reflect.DeepEqual(gotVendor, vendor) {
		t.Fatalf("unexpected trusted vendor: got %#v want %#v ok=%t", gotVendor, vendor, ok)
	}

	keys := []marketstore.TrustedVendorKey{
		{VendorID: "acme", KeyID: "alpha"},
		{VendorID: "acme", KeyID: "beta"},
	}
	if err := s.TrustedVendorKeys().ReplaceTrustedVendorKeys(ctx, "acme", keys); err != nil {
		t.Fatalf("ReplaceTrustedVendorKeys: %v", err)
	}
	gotKeys, err := s.TrustedVendorKeys().ListTrustedVendorKeys(ctx, "acme")
	if err != nil {
		t.Fatalf("ListTrustedVendorKeys: %v", err)
	}
	if !reflect.DeepEqual(gotKeys, keys) {
		t.Fatalf("unexpected trusted vendor keys: got %#v want %#v", gotKeys, keys)
	}
}

func TestPostgresRuntimePersistenceParity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := pgtest.OpenMigratedDB(t)
	t.Cleanup(func() { _ = db.Close() })

	s, err := postgresstore.New(db)
	mustNoErr(t, err, "postgresstore.New")

	now := time.Unix(1_700_000_000, 0).UTC()
	actorID := uint64(42)

	mustNoErr(t, s.Restrictions().PutRestriction(ctx, moderationstore.Restriction{
		TargetType: moderationstore.TargetTypeGuild,
		TargetID:   100,
		Reason:     "cooldown",
		CreatedBy:  actorID,
		CreatedAt:  now,
	}), "PutRestriction")
	restriction, ok, err := s.Restrictions().GetRestriction(ctx, moderationstore.TargetTypeGuild, 100)
	mustNoErr(t, err, "GetRestriction")
	if !ok || !reflect.DeepEqual(restriction, moderationstore.Restriction{
		TargetType: moderationstore.TargetTypeGuild,
		TargetID:   100,
		Reason:     "cooldown",
		CreatedBy:  actorID,
		CreatedAt:  now,
	}) {
		t.Fatalf("unexpected restriction: got %#v ok=%t", restriction, ok)
	}
	mustNoErr(t, s.Restrictions().DeleteRestriction(ctx, moderationstore.TargetTypeGuild, 100), "DeleteRestriction")
	_, ok, err = s.Restrictions().GetRestriction(ctx, moderationstore.TargetTypeGuild, 100)
	mustNoErr(t, err, "GetRestriction(after delete)")
	if ok {
		t.Fatal("expected restriction to be deleted")
	}

	mustNoErr(t, s.Warnings().CreateWarning(ctx, moderationstore.Warning{
		ID:          "warn-old",
		GuildID:     7,
		UserID:      9,
		ModeratorID: 11,
		Reason:      "old",
		CreatedAt:   now,
	}), "CreateWarning(old)")
	mustNoErr(t, s.Warnings().CreateWarning(ctx, moderationstore.Warning{
		ID:          "warn-new",
		GuildID:     7,
		UserID:      9,
		ModeratorID: 12,
		Reason:      "new",
		CreatedAt:   now.Add(time.Second),
	}), "CreateWarning(new)")
	count, err := s.Warnings().CountWarnings(ctx, 7, 9)
	mustNoErr(t, err, "CountWarnings")
	if count != 2 {
		t.Fatalf("unexpected warning count: %d", count)
	}
	warnings, err := s.Warnings().ListWarnings(ctx, 7, 9, 1)
	mustNoErr(t, err, "ListWarnings")
	if len(warnings) != 1 || warnings[0].ID != "warn-new" {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	mustNoErr(t, s.Warnings().DeleteWarning(ctx, "warn-old"), "DeleteWarning")
	count, err = s.Warnings().CountWarnings(ctx, 7, 9)
	mustNoErr(t, err, "CountWarnings(after delete)")
	if count != 1 {
		t.Fatalf("unexpected warning count after delete: %d", count)
	}

	targetType := moderationstore.TargetTypeUser
	targetID := uint64(123)
	mustNoErr(t, s.Audit().Append(ctx, moderationstore.AuditEntry{
		GuildID:    nil,
		ActorID:    &actorID,
		Action:     "warn.create",
		TargetType: &targetType,
		TargetID:   &targetID,
		CreatedAt:  now.Add(2 * time.Second),
		MetaJSON:   "",
	}), "Audit.Append")
	var auditAction, auditTargetType, auditMeta string
	var auditActorID, auditTargetID, auditCreatedAt int64
	if err := db.QueryRowContext(
		ctx,
		`SELECT action, actor_id, target_type, target_id, created_at, meta_json FROM audit_log ORDER BY id DESC LIMIT 1`,
	).Scan(&auditAction, &auditActorID, &auditTargetType, &auditTargetID, &auditCreatedAt, &auditMeta); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if auditAction != "warn.create" || auditTargetType != string(targetType) || auditTargetID != int64(targetID) || auditActorID != int64(actorID) || auditCreatedAt != now.Add(2*time.Second).Unix() || auditMeta != "{}" {
		t.Fatalf("unexpected audit row: action=%q actor_id=%d target_type=%q target_id=%d created_at=%d meta=%q", auditAction, auditActorID, auditTargetType, auditTargetID, auditCreatedAt, auditMeta)
	}

	mustNoErr(t, s.PluginKV().PutPluginKV(ctx, 55, "weather", "config", `{"units":"metric"}`), "PutPluginKV")
	value, ok, err := s.PluginKV().GetPluginKV(ctx, 55, "weather", "config")
	mustNoErr(t, err, "GetPluginKV")
	if !ok || value != `{"units":"metric"}` {
		t.Fatalf("unexpected plugin kv: value=%q ok=%t", value, ok)
	}
	mustNoErr(t, s.PluginKV().DeletePluginKV(ctx, 55, "weather", "config"), "DeletePluginKV")
	_, ok, err = s.PluginKV().GetPluginKV(ctx, 55, "weather", "config")
	mustNoErr(t, err, "GetPluginKV(after delete)")
	if ok {
		t.Fatal("expected plugin kv to be deleted")
	}

	mustNoErr(t, s.AdminSessions().PutAdminSession(ctx, accountstore.AdminSession{
		ID:          "sess-1",
		UserID:      77,
		Username:    "mod",
		Name:        "Moderator",
		AvatarURL:   "https://example.invalid/avatar.png",
		CSRFToken:   "csrf",
		AccessToken: "token",
		IsOwner:     true,
		ExpiresAt:   now.Add(time.Hour).Unix(),
	}), "PutAdminSession")
	session, ok, err := s.AdminSessions().GetAdminSession(ctx, "sess-1")
	mustNoErr(t, err, "GetAdminSession")
	if !ok || session.ID != "sess-1" || !session.IsOwner || session.UserID != 77 {
		t.Fatalf("unexpected admin session: %#v ok=%t", session, ok)
	}
	deletedCount, err := s.AdminSessions().DeleteExpiredAdminSessions(ctx, now.Unix())
	mustNoErr(t, err, "DeleteExpiredAdminSessions(before expiry)")
	if deletedCount != 0 {
		t.Fatalf("unexpected deleted expired session count: %d", deletedCount)
	}
	deletedCount, err = s.AdminSessions().DeleteExpiredAdminSessions(ctx, now.Add(2*time.Hour).Unix())
	mustNoErr(t, err, "DeleteExpiredAdminSessions(after expiry)")
	if deletedCount != 1 {
		t.Fatalf("unexpected deleted expired session count after expiry: %d", deletedCount)
	}

	mustNoErr(t, s.Users().UpsertUserSeen(ctx, accountstore.UserSeen{
		UserID:      1,
		CreatedAt:   time.Unix(1_600_000_000, 0).UTC(),
		IsBot:       false,
		IsSystem:    false,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}), "UpsertUserSeen")
	mustNoErr(t, s.Users().TouchUserSeen(ctx, 1, now.Add(10*time.Second)), "TouchUserSeen")
	var userCreatedAt, userFirstSeenAt, userLastSeenAt int64
	var userIsBot, userIsSystem bool
	if err := db.QueryRowContext(
		ctx,
		`SELECT created_at, is_bot, is_system, first_seen_at, last_seen_at FROM users WHERE user_id = $1`,
		1,
	).Scan(&userCreatedAt, &userIsBot, &userIsSystem, &userFirstSeenAt, &userLastSeenAt); err != nil {
		t.Fatalf("query users: %v", err)
	}
	if userCreatedAt != 1_600_000_000 || userIsBot || userIsSystem || userFirstSeenAt != now.Unix() || userLastSeenAt != now.Add(10*time.Second).Unix() {
		t.Fatalf("unexpected users row: created_at=%d is_bot=%t is_system=%t first_seen_at=%d last_seen_at=%d", userCreatedAt, userIsBot, userIsSystem, userFirstSeenAt, userLastSeenAt)
	}

	mustNoErr(t, s.Guilds().UpsertGuildSeen(ctx, accountstore.GuildSeen{
		GuildID:   10,
		OwnerID:   2,
		CreatedAt: time.Unix(1_500_000_000, 0).UTC(),
		JoinedAt:  now,
		Name:      "x",
		UpdatedAt: now,
	}), "UpsertGuildSeen")
	mustNoErr(t, s.Guilds().MarkGuildLeft(ctx, 10, now.Add(5*time.Second)), "MarkGuildLeft")
	mustNoErr(t, s.Guilds().UpdateGuildOwner(ctx, 10, 3, now.Add(6*time.Second)), "UpdateGuildOwner")
	var guildOwnerID int64
	var guildJoinedAt, guildLeftAt sql.NullInt64
	var guildName string
	var guildUpdatedAt int64
	if err := db.QueryRowContext(
		ctx,
		`SELECT owner_id, joined_at, left_at, name, updated_at FROM guilds WHERE guild_id = $1`,
		10,
	).Scan(&guildOwnerID, &guildJoinedAt, &guildLeftAt, &guildName, &guildUpdatedAt); err != nil {
		t.Fatalf("query guilds: %v", err)
	}
	if guildOwnerID != 3 || !guildJoinedAt.Valid || guildJoinedAt.Int64 != now.Unix() || !guildLeftAt.Valid || guildLeftAt.Int64 != now.Add(5*time.Second).Unix() || guildName != "x" || guildUpdatedAt != now.Add(6*time.Second).Unix() {
		t.Fatalf("unexpected guild row: owner_id=%d joined_at=%v left_at=%v name=%q updated_at=%d", guildOwnerID, guildJoinedAt, guildLeftAt, guildName, guildUpdatedAt)
	}

	mustNoErr(t, s.GuildMembers().MarkMemberJoined(ctx, 10, 1, now), "MarkMemberJoined")
	mustNoErr(t, s.GuildMembers().MarkMemberLeft(ctx, 10, 1, now.Add(time.Second)), "MarkMemberLeft")
	var memberJoinedAt int64
	var memberLeftAt sql.NullInt64
	if err := db.QueryRowContext(
		ctx,
		`SELECT joined_at, left_at FROM guild_members WHERE guild_id = $1 AND user_id = $2`,
		10,
		1,
	).Scan(&memberJoinedAt, &memberLeftAt); err != nil {
		t.Fatalf("query guild_members: %v", err)
	}
	if memberJoinedAt != now.Unix() || !memberLeftAt.Valid || memberLeftAt.Int64 != now.Add(time.Second).Unix() {
		t.Fatalf("unexpected guild_members row: joined_at=%d left_at=%v", memberJoinedAt, memberLeftAt)
	}

	mustNoErr(t, s.UserSettings().UpsertUserTimezone(ctx, 99, "Europe/Tallinn"), "UpsertUserTimezone")
	mustNoErr(t, s.UserSettings().UpsertUserDMChannelID(ctx, 99, 555), "UpsertUserDMChannelID")
	settings, ok, err := s.UserSettings().GetUserSettings(ctx, 99)
	mustNoErr(t, err, "GetUserSettings")
	if !ok || settings.UserID != 99 || settings.Timezone != "Europe/Tallinn" || settings.DMChannelID == nil || *settings.DMChannelID != 555 || settings.CreatedAt.IsZero() || settings.UpdatedAt.IsZero() {
		t.Fatalf("unexpected user settings: %#v ok=%t", settings, ok)
	}
	mustNoErr(t, s.UserSettings().ClearUserTimezone(ctx, 99), "ClearUserTimezone")
	settings, ok, err = s.UserSettings().GetUserSettings(ctx, 99)
	mustNoErr(t, err, "GetUserSettings(after clear)")
	if !ok || settings.Timezone != "" || settings.DMChannelID == nil || *settings.DMChannelID != 555 {
		t.Fatalf("unexpected user settings after clear: %#v ok=%t", settings, ok)
	}

	mustNoErr(t, s.CheckIns().CreateCheckIn(ctx, automationstore.CheckIn{
		ID:        "check-1",
		UserID:    200,
		Mood:      4,
		CreatedAt: now,
	}), "CreateCheckIn")
	checkins, err := s.CheckIns().ListCheckIns(ctx, 200, 10)
	mustNoErr(t, err, "ListCheckIns")
	if len(checkins) != 1 || !reflect.DeepEqual(checkins[0], automationstore.CheckIn{
		ID:        "check-1",
		UserID:    200,
		Mood:      4,
		CreatedAt: now,
	}) {
		t.Fatalf("unexpected checkins: %#v", checkins)
	}

	token := accountstore.DiscordOAuthToken{
		UserID:          501,
		AccessTokenEnc:  "access",
		RefreshTokenEnc: "refresh",
		Scope:           "identify guilds",
		ExpiresAt:       now.Add(30 * time.Minute),
	}
	mustNoErr(t, s.DiscordOAuthTokens().PutDiscordOAuthToken(ctx, token), "PutDiscordOAuthToken")
	gotToken, ok, err := s.DiscordOAuthTokens().GetDiscordOAuthToken(ctx, 501)
	mustNoErr(t, err, "GetDiscordOAuthToken")
	if !ok {
		t.Fatal("expected discord oauth token to exist")
	}
	if gotToken.UserID != token.UserID || gotToken.AccessTokenEnc != token.AccessTokenEnc || gotToken.RefreshTokenEnc != token.RefreshTokenEnc || gotToken.Scope != token.Scope || !gotToken.ExpiresAt.Equal(token.ExpiresAt) {
		t.Fatalf("unexpected discord oauth token: %#v", gotToken)
	}
	mustNoErr(t, s.DiscordOAuthTokens().DeleteDiscordOAuthToken(ctx, 501), "DeleteDiscordOAuthToken")
	_, ok, err = s.DiscordOAuthTokens().GetDiscordOAuthToken(ctx, 501)
	mustNoErr(t, err, "GetDiscordOAuthToken(after delete)")
	if ok {
		t.Fatal("expected discord oauth token to be deleted")
	}

	grantA := pluginstore.PluginOAuthGrant{
		UserID:    501,
		PluginID:  "weather",
		Scope:     "forecast:read",
		CreatedAt: now.Add(40 * time.Minute),
	}
	grantB := pluginstore.PluginOAuthGrant{
		UserID:    501,
		PluginID:  "wellness",
		Scope:     "checkins:write",
		CreatedAt: now.Add(41 * time.Minute),
	}
	mustNoErr(t, s.PluginOAuthGrants().PutPluginOAuthGrant(ctx, grantA), "PutPluginOAuthGrant(weather)")
	mustNoErr(t, s.PluginOAuthGrants().PutPluginOAuthGrant(ctx, grantB), "PutPluginOAuthGrant(wellness)")
	gotGrant, ok, err := s.PluginOAuthGrants().GetPluginOAuthGrant(ctx, 501, "weather")
	mustNoErr(t, err, "GetPluginOAuthGrant")
	if !ok || gotGrant.UserID != grantA.UserID || gotGrant.PluginID != grantA.PluginID || gotGrant.Scope != grantA.Scope || !gotGrant.CreatedAt.Equal(grantA.CreatedAt) {
		t.Fatalf("unexpected plugin oauth grant: %#v ok=%t", gotGrant, ok)
	}
	grants, err := s.PluginOAuthGrants().ListPluginOAuthGrants(ctx, 501)
	mustNoErr(t, err, "ListPluginOAuthGrants")
	if len(grants) != 2 || grants[0].PluginID != "weather" || grants[1].PluginID != "wellness" {
		t.Fatalf("unexpected plugin oauth grants: %#v", grants)
	}
	grantCount, err := s.PluginOAuthGrants().CountPluginOAuthGrants(ctx, 501)
	mustNoErr(t, err, "CountPluginOAuthGrants")
	if grantCount != 2 {
		t.Fatalf("unexpected plugin oauth grant count: %d", grantCount)
	}
	mustNoErr(t, s.PluginOAuthGrants().DeletePluginOAuthGrant(ctx, 501, "weather"), "DeletePluginOAuthGrant")
	grantCount, err = s.PluginOAuthGrants().CountPluginOAuthGrants(ctx, 501)
	mustNoErr(t, err, "CountPluginOAuthGrants(after delete)")
	if grantCount != 1 {
		t.Fatalf("unexpected plugin oauth grant count after delete: %d", grantCount)
	}
}

func TestCanonicalPluginConfigMigrationAndVersionedCAS(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.OpenMigratedDB(t)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, `INSERT INTO plugin_kv(guild_id,plugin_id,key,value_json,updated_at) VALUES(991,'moderation','__guild_config','{"enabled":false}',1)`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "postgres", "009_canonical_plugin_config_key.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	var value string
	var version uint64
	if err = db.QueryRowContext(ctx, `SELECT value_json,version FROM plugin_kv WHERE guild_id=991 AND plugin_id='moderation' AND key='guild_config'`).Scan(&value, &version); err != nil {
		t.Fatal(err)
	}
	if value != "{\"enabled\":false}" || version != 1 {
		t.Fatalf("value=%q version=%d", value, version)
	}
	var oldCount int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM plugin_kv WHERE guild_id=991 AND plugin_id='moderation' AND key='__guild_config'`).Scan(&oldCount); err != nil || oldCount != 0 {
		t.Fatalf("old rows=%d err=%v", oldCount, err)
	}
	storage, err := postgresstore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	kv := storage.PluginKV().(pluginstore.VersionedPluginKVStore)
	next, swapped, err := kv.CompareAndSwapPluginKV(ctx, 992, "example", "counter", "1", 0)
	if err != nil || !swapped || next != 1 {
		t.Fatalf("initial CAS next=%d swapped=%v err=%v", next, swapped, err)
	}
	if _, swapped, err = kv.CompareAndSwapPluginKV(ctx, 992, "example", "counter", "2", 0); err != nil || swapped {
		t.Fatalf("stale insert swapped=%v err=%v", swapped, err)
	}
	next, swapped, err = kv.CompareAndSwapPluginKV(ctx, 992, "example", "counter", "2", 1)
	if err != nil || !swapped || next != 2 {
		t.Fatalf("update CAS next=%d swapped=%v err=%v", next, swapped, err)
	}
	if err = kv.PutPluginKV(ctx, 992, "example", "counter", "3"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := kv.GetPluginKVVersioned(ctx, 992, "example", "counter")
	if err != nil || !ok || got.ValueJSON != "3" || got.Version != 3 {
		t.Fatalf("get=%#v ok=%v err=%v", got, ok, err)
	}
	if deleted, err := kv.DeletePluginKVVersion(ctx, 992, "example", "counter", 2); err != nil || deleted {
		t.Fatalf("stale delete=%v err=%v", deleted, err)
	}
	if deleted, err := kv.DeletePluginKVVersion(ctx, 992, "example", "counter", 3); err != nil || !deleted {
		t.Fatalf("delete=%v err=%v", deleted, err)
	}
}
