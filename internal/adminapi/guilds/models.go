package guilds

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Snowflake is a Discord ID. It is encoded as a JSON string to avoid
// precision loss in dashboard JavaScript, but accepts strings and numbers.
type Snowflake uint64

func (s Snowflake) String() string {
	return strconv.FormatUint(uint64(s), 10)
}

func (s Snowflake) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(strconv.FormatUint(uint64(s), 10))), nil
}

func (s *Snowflake) UnmarshalJSON(data []byte) error {
	if s == nil {
		return fmt.Errorf("snowflake: nil receiver")
	}
	if bytes.Equal(data, []byte("null")) {
		*s = 0
		return nil
	}
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			*s = 0
			return nil
		}
		value, err := strconv.ParseUint(asString, 10, 64)
		if err != nil {
			return fmt.Errorf("snowflake: invalid %q", asString)
		}
		*s = Snowflake(value)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		raw := strings.TrimSpace(number.String())
		if raw == "" {
			*s = 0
			return nil
		}
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("snowflake: invalid %q", raw)
		}
		*s = Snowflake(value)
		return nil
	}
	return fmt.Errorf("snowflake: invalid json %q", strings.TrimSpace(string(data)))
}

type UserGuildSummary struct {
	ID           Snowflake `json:"id"`
	Name         string    `json:"name"`
	IconURL      string    `json:"icon_url,omitempty"`
	Owner        bool      `json:"owner"`
	CanManage    bool      `json:"can_manage"`
	BotInstalled bool      `json:"bot_installed"`
}

type GuildDashboardResponse struct {
	Guild       UserGuildSummary  `json:"guild"`
	InstallURL  string            `json:"install_url"`
	SetupChecks []SetupCheck      `json:"setup_checks"`
	Manager     ManagerSection    `json:"manager"`
	Moderation  ModerationSection `json:"moderation"`
	Fun         PluginSection     `json:"fun"`
	Info        PluginSection     `json:"info"`
	Wellness    WellnessSection   `json:"wellness"`
}

type SetupCheck struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type PluginCommandState struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

type PluginSection struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Enabled       bool                 `json:"enabled"`
	GlobalEnabled bool                 `json:"global_enabled"`
	Commands      []PluginCommandState `json:"commands"`
}

type ManagerSection struct {
	PluginSection
	ChannelCount int `json:"channel_count"`
	RoleCount    int `json:"role_count"`
	EmojiCount   int `json:"emoji_count"`
	StickerCount int `json:"sticker_count"`
}

type ModerationSection struct {
	PluginSection
	WarningLimit     int `json:"warning_limit"`
	TimeoutThreshold int `json:"timeout_threshold"`
	TimeoutMinutes   int `json:"timeout_minutes"`
}

type WellnessSection struct {
	PluginSection
	AllowChannelReminders    bool      `json:"allow_channel_reminders"`
	DefaultReminderChannelID Snowflake `json:"default_reminder_channel_id,omitempty"`
}

type GuildChannelInfo struct {
	ID       Snowflake `json:"id"`
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	ParentID Snowflake `json:"parent_id,omitempty"`
}

type GuildRoleInfo struct {
	ID          Snowflake `json:"id"`
	Name        string    `json:"name"`
	Color       int       `json:"color"`
	Position    int       `json:"position"`
	Managed     bool      `json:"managed"`
	Mentionable bool      `json:"mentionable"`
}

type GuildMemberInfo struct {
	UserID      Snowflake   `json:"user_id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"display_name"`
	AvatarURL   string      `json:"avatar_url,omitempty"`
	Bot         bool        `json:"bot"`
	JoinedAt    int64       `json:"joined_at,omitempty"`
	RoleIDs     []Snowflake `json:"role_ids"`
}

type GuildEmojiInfo struct {
	ID       Snowflake `json:"id"`
	Name     string    `json:"name"`
	Animated bool      `json:"animated"`
}

type GuildStickerInfo struct {
	ID          Snowflake `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Tags        string    `json:"tags,omitempty"`
}

type WarningInfo struct {
	ID          string    `json:"id"`
	UserID      Snowflake `json:"user_id"`
	ModeratorID Snowflake `json:"moderator_id"`
	Reason      string    `json:"reason"`
	CreatedAt   string    `json:"created_at"`
}
