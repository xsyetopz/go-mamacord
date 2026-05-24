package storage

import (
	"context"
	"time"
)

type TargetType string

const (
	TargetTypeUser  TargetType = "user"
	TargetTypeGuild TargetType = "guild"
)

type Restriction struct {
	TargetType TargetType
	TargetID   uint64
	Reason     string
	CreatedBy  uint64
	CreatedAt  time.Time
}

type Warning struct {
	ID          string
	GuildID     uint64
	UserID      uint64
	ModeratorID uint64
	Reason      string
	CreatedAt   time.Time
}

type AuditEntry struct {
	GuildID    *uint64
	ActorID    *uint64
	Action     string
	TargetType *TargetType
	TargetID   *uint64
	CreatedAt  time.Time
	MetaJSON   string
}

type RestrictionStore interface {
	GetRestriction(ctx context.Context, targetType TargetType, targetID uint64) (Restriction, bool, error)
	PutRestriction(ctx context.Context, r Restriction) error
	DeleteRestriction(ctx context.Context, targetType TargetType, targetID uint64) error
}

type WarningStore interface {
	CountWarnings(ctx context.Context, guildID, userID uint64) (int, error)
	ListWarnings(ctx context.Context, guildID, userID uint64, limit int) ([]Warning, error)
	CreateWarning(ctx context.Context, w Warning) error
	DeleteWarning(ctx context.Context, id string) error
}

type AuditStore interface {
	Append(ctx context.Context, entry AuditEntry) error
}

type TrustedSigner struct {
	KeyID        string
	PublicKeyB64 string
	AddedAt      time.Time
}

type TrustedSignerStore interface {
	ListTrustedSigners(ctx context.Context) ([]TrustedSigner, error)
	PutTrustedSigner(ctx context.Context, signer TrustedSigner) error
	DeleteTrustedSigner(ctx context.Context, keyID string) error
}

type MarketplaceSource struct {
	SourceID    string
	Kind        string
	GitURL      string
	GitRef      string
	GitSubdir   string
	TokenEnvVar string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MarketplaceSourceStore interface {
	GetMarketplaceSource(ctx context.Context, sourceID string) (MarketplaceSource, bool, error)
	ListMarketplaceSources(ctx context.Context) ([]MarketplaceSource, error)
	PutMarketplaceSource(ctx context.Context, source MarketplaceSource) error
	DeleteMarketplaceSource(ctx context.Context, sourceID string) error
}

type MarketplaceSourceSync struct {
	SourceID     string
	LastSyncedAt *time.Time
	LastRevision string
	LastError    string
}

type MarketplaceSourceSyncStore interface {
	GetMarketplaceSourceSync(ctx context.Context, sourceID string) (MarketplaceSourceSync, bool, error)
	ListMarketplaceSourceSyncs(ctx context.Context) ([]MarketplaceSourceSync, error)
	PutMarketplaceSourceSync(ctx context.Context, sync MarketplaceSourceSync) error
	DeleteMarketplaceSourceSync(ctx context.Context, sourceID string) error
}

type PluginInstall struct {
	PluginID          string
	InstallKind       string
	SourceID          string
	GitURL            string
	GitRef            string
	GitRevision       string
	SourcePath        string
	BundleRelativeDir string
	InstalledAt       time.Time
	InstalledBy       *uint64
	InstalledHashB64  string
}

type PluginInstallStore interface {
	GetPluginInstall(ctx context.Context, pluginID string) (PluginInstall, bool, error)
	ListPluginInstalls(ctx context.Context) ([]PluginInstall, error)
	PutPluginInstall(ctx context.Context, install PluginInstall) error
	DeletePluginInstall(ctx context.Context, pluginID string) error
}

type TrustedVendor struct {
	VendorID   string
	Name       string
	WebsiteURL string
	SupportURL string
	AddedAt    time.Time
	UpdatedAt  time.Time
}

type TrustedVendorStore interface {
	GetTrustedVendor(ctx context.Context, vendorID string) (TrustedVendor, bool, error)
	ListTrustedVendors(ctx context.Context) ([]TrustedVendor, error)
	PutTrustedVendor(ctx context.Context, vendor TrustedVendor) error
	DeleteTrustedVendor(ctx context.Context, vendorID string) error
}

type TrustedVendorKey struct {
	VendorID string
	KeyID    string
}

type TrustedVendorKeyStore interface {
	ListTrustedVendorKeys(ctx context.Context, vendorID string) ([]TrustedVendorKey, error)
	ReplaceTrustedVendorKeys(ctx context.Context, vendorID string, keys []TrustedVendorKey) error
}

type PluginKVStore interface {
	GetPluginKV(ctx context.Context, guildID uint64, pluginID, key string) (valueJSON string, ok bool, err error)
	PutPluginKV(ctx context.Context, guildID uint64, pluginID, key, valueJSON string) error
	DeletePluginKV(ctx context.Context, guildID uint64, pluginID, key string) error
}

type ModuleState struct {
	ModuleID  string
	Enabled   bool
	UpdatedAt time.Time
	UpdatedBy *uint64
}

type ModuleStateStore interface {
	GetModuleState(ctx context.Context, moduleID string) (ModuleState, bool, error)
	ListModuleStates(ctx context.Context) ([]ModuleState, error)
	PutModuleState(ctx context.Context, state ModuleState) error
	DeleteModuleState(ctx context.Context, moduleID string) error
}

type UserSeen struct {
	UserID      uint64
	CreatedAt   time.Time
	IsBot       bool
	IsSystem    bool
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

type UserStore interface {
	UpsertUserSeen(ctx context.Context, u UserSeen) error
	TouchUserSeen(ctx context.Context, userID uint64, seenAt time.Time) error
}

type GuildSeen struct {
	GuildID   uint64
	OwnerID   uint64
	CreatedAt time.Time
	JoinedAt  time.Time
	LeftAt    *time.Time
	Name      string
	UpdatedAt time.Time
}

type GuildStore interface {
	UpsertGuildSeen(ctx context.Context, g GuildSeen) error
	MarkGuildLeft(ctx context.Context, guildID uint64, leftAt time.Time) error
	UpdateGuildOwner(ctx context.Context, guildID uint64, ownerID uint64, updatedAt time.Time) error
}

type GuildMemberStore interface {
	MarkMemberJoined(ctx context.Context, guildID, userID uint64, joinedAt time.Time) error
	MarkMemberLeft(ctx context.Context, guildID, userID uint64, leftAt time.Time) error
}

type UserSettings struct {
	UserID      uint64
	Timezone    string
	DMChannelID *uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UserSettingsStore interface {
	GetUserSettings(ctx context.Context, userID uint64) (UserSettings, bool, error)
	UpsertUserTimezone(ctx context.Context, userID uint64, timezone string) error
	ClearUserTimezone(ctx context.Context, userID uint64) error
	UpsertUserDMChannelID(ctx context.Context, userID uint64, dmChannelID uint64) error
}

type ReminderDelivery string

const (
	ReminderDeliveryDM      ReminderDelivery = "dm"
	ReminderDeliveryChannel ReminderDelivery = "channel"
)

type Reminder struct {
	ID           string
	UserID       uint64
	Schedule     string
	Kind         string
	Note         string
	Delivery     ReminderDelivery
	GuildID      *uint64
	ChannelID    *uint64
	Enabled      bool
	NextRunAt    time.Time
	LastRunAt    *time.Time
	FailureCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ReminderStore interface {
	CreateReminder(ctx context.Context, r Reminder) error
	ListReminders(ctx context.Context, userID uint64, limit int) ([]Reminder, error)
	DeleteReminder(ctx context.Context, userID uint64, reminderID string) (bool, error)

	ClaimDueReminders(
		ctx context.Context,
		now time.Time,
		leaseID string,
		leaseDuration time.Duration,
		limit int,
	) ([]Reminder, error)

	FinishReminderRun(
		ctx context.Context,
		reminderID string,
		leaseID string,
		lastRunAt time.Time,
		nextRunAt time.Time,
		failureCount int,
		enabled bool,
	) error
}

type CheckIn struct {
	ID        string
	UserID    uint64
	Mood      int
	CreatedAt time.Time
}

type CheckInStore interface {
	CreateCheckIn(ctx context.Context, c CheckIn) error
	ListCheckIns(ctx context.Context, userID uint64, limit int) ([]CheckIn, error)
}

type DiscordOAuthToken struct {
	UserID          uint64
	AccessTokenEnc  string
	RefreshTokenEnc string
	Scope           string
	ExpiresAt       time.Time
	UpdatedAt       time.Time
}

type DiscordOAuthTokenStore interface {
	GetDiscordOAuthToken(ctx context.Context, userID uint64) (DiscordOAuthToken, bool, error)
	PutDiscordOAuthToken(ctx context.Context, token DiscordOAuthToken) error
	DeleteDiscordOAuthToken(ctx context.Context, userID uint64) error
}

type PluginOAuthGrant struct {
	UserID    uint64
	PluginID  string
	Scope     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PluginOAuthGrantStore interface {
	GetPluginOAuthGrant(ctx context.Context, userID uint64, pluginID string) (PluginOAuthGrant, bool, error)
	ListPluginOAuthGrants(ctx context.Context, userID uint64) ([]PluginOAuthGrant, error)
	PutPluginOAuthGrant(ctx context.Context, grant PluginOAuthGrant) error
	DeletePluginOAuthGrant(ctx context.Context, userID uint64, pluginID string) error
	CountPluginOAuthGrants(ctx context.Context, userID uint64) (int, error)
}
