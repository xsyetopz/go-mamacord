package control

type RoleResult struct {
	ID          uint64
	Name        string
	Mention     string
	Color       int
	Hoist       bool
	Mentionable bool
	Position    int
	Managed     bool
	Permissions int64
	CreatedAt   int64
}
type EmojiResult struct {
	ID   uint64
	Name string
}
type StickerResult struct {
	ID   uint64
	Name string
}
type RoleCreateSpec struct {
	GuildID     uint64
	Name        string
	Color       *int
	Hoist       *bool
	Mentionable *bool
}
type RoleEditSpec struct {
	GuildID     uint64
	RoleID      uint64
	Name        *string
	Color       *int
	Hoist       *bool
	Mentionable *bool
}
type RoleMemberSpec struct {
	GuildID uint64
	UserID  uint64
	RoleID  uint64
}
type PurgeSpec struct {
	ChannelID uint64
	Mode      string
	AnchorRaw string
	Count     int
}
type EmojiEditSpec struct {
	GuildID  uint64
	RawEmoji string
	Name     string
}
type EmojiDeleteSpec struct {
	GuildID  uint64
	RawEmoji string
}
type StickerEditSpec struct {
	GuildID     uint64
	RawID       string
	Name        string
	Description *string
}
type StickerDeleteSpec struct {
	GuildID uint64
	RawID   string
}
