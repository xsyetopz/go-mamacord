package contract

type UserDetailsRef struct {
	User        UserRef
	Mention     string
	AvatarURL   string
	BannerURL   string
	CreatedAt   int64
	AccentColor *int
}

type MemberDetailsRef struct {
	Member    MemberRef
	JoinedAt  int64
	AvatarURL string
	BannerURL string
}

type GuildProfile struct {
	OwnerID     string
	Description string
	IconURL     string
	BannerURL   string
}

type GuildResourceCounts struct {
	RolesCount    int
	EmojisCount   int
	StickersCount int
	MemberCount   int
	ChannelsCount int
}

type GuildDetailsRef struct {
	Guild GuildRef
	GuildProfile
	GuildResourceCounts
	CreatedAt int64
}

type UserSettingsRef struct {
	Timezone    string
	DMChannelID string
	CreatedAt   int64
	UpdatedAt   int64
}

type CheckInRef struct {
	ID        string
	Mood      int
	CreatedAt int64
}

type ReminderPlanRef struct {
	Schedule  string
	NextRunAt int64
}

type ReminderDefinition struct {
	ID       string
	Schedule string
	Kind     string
	Note     string
}

type ReminderDestination struct {
	Delivery  string
	GuildID   string
	ChannelID string
}

type ReminderScheduleState struct {
	Enabled      bool
	NextRunAt    int64
	LastRunAt    int64
	FailureCount int
}

type ReminderTimestamps struct {
	CreatedAt int64
	UpdatedAt int64
}

type ReminderRef struct {
	ReminderDefinition
	ReminderDestination
	ReminderScheduleState
	ReminderTimestamps
}

type WarningRef struct {
	ID          string
	UserID      string
	ModeratorID string
	Reason      string
	CreatedAt   int64
}
