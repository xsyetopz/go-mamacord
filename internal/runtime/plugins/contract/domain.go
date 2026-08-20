package contract

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type domainOperation interface {
	Operation
	validateDomain(Invocation) error
}

type SendChannelOperation struct {
	ChannelID string
	Message   Message
}

func (*SendChannelOperation) pluginOperation() {}
func (operation *SendChannelOperation) cloneOperation() Operation {
	if operation == nil {
		return (*SendChannelOperation)(nil)
	}
	copy := *operation
	copy.Message = operation.Message.DeepClone()
	return &copy
}
func (operation *SendChannelOperation) validateDomain(_ Invocation) error {
	if operation == nil {
		return errors.New("send channel operation is nil")
	}
	if err := validateSnowflake(operation.ChannelID, "channel id"); err != nil {
		return err
	}
	return operation.Message.Validate()
}

type SendDMOperation struct {
	UserID  string
	Message Message
}

func (*SendDMOperation) pluginOperation() {}
func (operation *SendDMOperation) cloneOperation() Operation {
	if operation == nil {
		return (*SendDMOperation)(nil)
	}
	copy := *operation
	copy.Message = operation.Message.DeepClone()
	return &copy
}
func (operation *SendDMOperation) validateDomain(_ Invocation) error {
	if operation == nil {
		return errors.New("send DM operation is nil")
	}
	if err := validateSnowflake(operation.UserID, "user id"); err != nil {
		return err
	}
	return operation.Message.Validate()
}

type TimeoutMemberOperation struct {
	UserID    string
	UntilUnix int64
}

func (*TimeoutMemberOperation) pluginOperation() {}
func (operation *TimeoutMemberOperation) cloneOperation() Operation {
	if operation == nil {
		return (*TimeoutMemberOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *TimeoutMemberOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("timeout member requires guild context")
	}
	if err := validateSnowflake(operation.UserID, "user id"); err != nil {
		return err
	}
	if operation.UntilUnix <= invocation.NowUnix {
		return errors.New("timeout must end in the future")
	}
	return nil
}

type SetSlowmodeOperation struct {
	ChannelID string
	Seconds   int
}

func (*SetSlowmodeOperation) pluginOperation() {}
func (operation *SetSlowmodeOperation) cloneOperation() Operation {
	if operation == nil {
		return (*SetSlowmodeOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *SetSlowmodeOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("slowmode requires guild context")
	}
	if err := validateSnowflake(operation.ChannelID, "channel id"); err != nil {
		return err
	}
	if operation.Seconds < 0 || operation.Seconds > 21600 {
		return errors.New("slowmode must be between 0 and 21600 seconds")
	}
	return nil
}

type SetNicknameOperation struct {
	UserID   string
	Nickname OptionalString
}

func (*SetNicknameOperation) pluginOperation() {}
func (operation *SetNicknameOperation) cloneOperation() Operation {
	if operation == nil {
		return (*SetNicknameOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *SetNicknameOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("nickname requires guild context")
	}
	if err := validateSnowflake(operation.UserID, "user id"); err != nil {
		return err
	}
	if !operation.Nickname.Set {
		return errors.New("nickname value is required")
	}
	if !utf8.ValidString(operation.Nickname.Value) || utf8.RuneCountInString(operation.Nickname.Value) > 32 {
		return errors.New("nickname must be valid UTF-8 and at most 32 characters")
	}
	return nil
}

type PurgeMode string

const (
	PurgeAll    PurgeMode = "all"
	PurgeBefore PurgeMode = "before"
	PurgeAfter  PurgeMode = "after"
	PurgeAround PurgeMode = "around"
)

type PurgeMessagesOperation struct {
	ChannelID       string
	Mode            PurgeMode
	AnchorMessageID string
	Count           int
}

func (*PurgeMessagesOperation) pluginOperation() {}
func (operation *PurgeMessagesOperation) cloneOperation() Operation {
	if operation == nil {
		return (*PurgeMessagesOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *PurgeMessagesOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("purge requires guild context")
	}
	if err := validateSnowflake(operation.ChannelID, "channel id"); err != nil {
		return err
	}
	if operation.Count < 1 || operation.Count > 100 {
		return errors.New("purge count must be between 1 and 100")
	}
	switch operation.Mode {
	case PurgeAll:
		if operation.AnchorMessageID != "" {
			return errors.New("all purge cannot have anchor")
		}
	case PurgeBefore, PurgeAfter, PurgeAround:
		if err := validateSnowflake(operation.AnchorMessageID, "anchor message id"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported purge mode %q", operation.Mode)
	}
	return nil
}

type CreateRoleOperation struct {
	Name        string
	Color       *int
	Hoist       *bool
	Mentionable *bool
}

func (*CreateRoleOperation) pluginOperation() {}
func (operation *CreateRoleOperation) cloneOperation() Operation {
	if operation == nil {
		return (*CreateRoleOperation)(nil)
	}
	copy := *operation
	copy.Color = cloneInt(operation.Color)
	copy.Hoist = cloneBool(operation.Hoist)
	copy.Mentionable = cloneBool(operation.Mentionable)
	return &copy
}
func (operation *CreateRoleOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("create role requires guild context")
	}
	if strings.TrimSpace(operation.Name) == "" {
		return errors.New("role name is required")
	}
	return validateRoleMutation(operation.Name, operation.Color)
}

type EditRoleOperation struct {
	RoleID      string
	Name        *string
	Color       *int
	Hoist       *bool
	Mentionable *bool
}

func (*EditRoleOperation) pluginOperation() {}
func (operation *EditRoleOperation) cloneOperation() Operation {
	if operation == nil {
		return (*EditRoleOperation)(nil)
	}
	copy := *operation
	copy.Name = cloneString(operation.Name)
	copy.Color = cloneInt(operation.Color)
	copy.Hoist = cloneBool(operation.Hoist)
	copy.Mentionable = cloneBool(operation.Mentionable)
	return &copy
}
func (operation *EditRoleOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("edit role requires guild context")
	}
	if err := validateSnowflake(operation.RoleID, "role id"); err != nil {
		return err
	}
	if operation.Name == nil && operation.Color == nil && operation.Hoist == nil && operation.Mentionable == nil {
		return errors.New("edit role has no fields")
	}
	name := ""
	if operation.Name != nil {
		name = *operation.Name
	}
	return validateRoleMutation(name, operation.Color)
}

type DeleteRoleOperation struct{ RoleID string }

func (*DeleteRoleOperation) pluginOperation() {}
func (operation *DeleteRoleOperation) cloneOperation() Operation {
	if operation == nil {
		return (*DeleteRoleOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *DeleteRoleOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("delete role requires guild context")
	}
	return validateSnowflake(operation.RoleID, "role id")
}

type MemberRoleOperation struct {
	UserID string
	RoleID string
	Add    bool
}

func (*MemberRoleOperation) pluginOperation() {}
func (operation *MemberRoleOperation) cloneOperation() Operation {
	if operation == nil {
		return (*MemberRoleOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *MemberRoleOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("member role requires guild context")
	}
	if err := validateSnowflake(operation.UserID, "user id"); err != nil {
		return err
	}
	return validateSnowflake(operation.RoleID, "role id")
}

type CreateEmojiOperation struct {
	Name         string
	AttachmentID string
}

func (*CreateEmojiOperation) pluginOperation() {}
func (operation *CreateEmojiOperation) cloneOperation() Operation {
	if operation == nil {
		return (*CreateEmojiOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *CreateEmojiOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("create emoji requires guild context")
	}
	if err := validateAssetName(operation.Name, 32); err != nil {
		return err
	}
	return validateInvocationAttachment(invocation, operation.AttachmentID)
}

type EditEmojiOperation struct {
	Emoji string
	Name  string
}

func (*EditEmojiOperation) pluginOperation() {}
func (operation *EditEmojiOperation) cloneOperation() Operation {
	if operation == nil {
		return (*EditEmojiOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *EditEmojiOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("edit emoji requires guild context")
	}
	if strings.TrimSpace(operation.Emoji) == "" || utf8.RuneCountInString(operation.Emoji) > 100 {
		return errors.New("emoji reference is invalid")
	}
	return validateAssetName(operation.Name, 32)
}

type DeleteEmojiOperation struct{ Emoji string }

func (*DeleteEmojiOperation) pluginOperation() {}
func (operation *DeleteEmojiOperation) cloneOperation() Operation {
	if operation == nil {
		return (*DeleteEmojiOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *DeleteEmojiOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("delete emoji requires guild context")
	}
	if strings.TrimSpace(operation.Emoji) == "" || utf8.RuneCountInString(operation.Emoji) > 100 {
		return errors.New("emoji reference is invalid")
	}
	return nil
}

type CreateStickerOperation struct {
	Name         string
	Description  string
	EmojiTag     string
	AttachmentID string
}

func (*CreateStickerOperation) pluginOperation() {}
func (operation *CreateStickerOperation) cloneOperation() Operation {
	if operation == nil {
		return (*CreateStickerOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *CreateStickerOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("create sticker requires guild context")
	}
	if err := validateAssetName(operation.Name, 30); err != nil {
		return err
	}
	if utf8.RuneCountInString(operation.Description) > 100 {
		return errors.New("sticker description exceeds 100 characters")
	}
	if strings.TrimSpace(operation.EmojiTag) == "" || utf8.RuneCountInString(operation.EmojiTag) > 200 {
		return errors.New("sticker emoji tag is invalid")
	}
	return validateInvocationAttachment(invocation, operation.AttachmentID)
}

type EditStickerOperation struct {
	StickerID   string
	Name        *string
	Description *string
	EmojiTag    *string
}

func (*EditStickerOperation) pluginOperation() {}
func (operation *EditStickerOperation) cloneOperation() Operation {
	if operation == nil {
		return (*EditStickerOperation)(nil)
	}
	copy := *operation
	copy.Name = cloneString(operation.Name)
	copy.Description = cloneString(operation.Description)
	copy.EmojiTag = cloneString(operation.EmojiTag)
	return &copy
}
func (operation *EditStickerOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("edit sticker requires guild context")
	}
	if err := validateSnowflake(operation.StickerID, "sticker id"); err != nil {
		return err
	}
	if operation.Name == nil && operation.Description == nil && operation.EmojiTag == nil {
		return errors.New("edit sticker has no fields")
	}
	if operation.Name != nil {
		if err := validateAssetName(*operation.Name, 30); err != nil {
			return err
		}
	}
	if operation.Description != nil && utf8.RuneCountInString(*operation.Description) > 100 {
		return errors.New("sticker description exceeds 100 characters")
	}
	if operation.EmojiTag != nil && strings.TrimSpace(*operation.EmojiTag) == "" {
		return errors.New("sticker emoji tag is invalid")
	}
	return nil
}

type DeleteStickerOperation struct{ StickerID string }

func (*DeleteStickerOperation) pluginOperation() {}
func (operation *DeleteStickerOperation) cloneOperation() Operation {
	if operation == nil {
		return (*DeleteStickerOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *DeleteStickerOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil {
		return errors.New("delete sticker requires guild context")
	}
	return validateSnowflake(operation.StickerID, "sticker id")
}

type SetTimezoneOperation struct{ Timezone string }

func (*SetTimezoneOperation) pluginOperation() {}
func (operation *SetTimezoneOperation) cloneOperation() Operation {
	if operation == nil {
		return (*SetTimezoneOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *SetTimezoneOperation) validateDomain(invocation Invocation) error {
	if invocation.Author == nil {
		return errors.New("timezone requires author")
	}
	if strings.TrimSpace(operation.Timezone) == "" || utf8.RuneCountInString(operation.Timezone) > 100 {
		return errors.New("timezone is invalid")
	}
	return nil
}

type ClearTimezoneOperation struct{}

func (*ClearTimezoneOperation) pluginOperation() {}
func (operation *ClearTimezoneOperation) cloneOperation() Operation {
	if operation == nil {
		return (*ClearTimezoneOperation)(nil)
	}
	return &ClearTimezoneOperation{}
}
func (*ClearTimezoneOperation) validateDomain(invocation Invocation) error {
	if invocation.Author == nil {
		return errors.New("clear timezone requires author")
	}
	return nil
}

type CreateCheckInOperation struct {
	Mood      int
	CreatedAt int64
}

func (*CreateCheckInOperation) pluginOperation() {}
func (operation *CreateCheckInOperation) cloneOperation() Operation {
	if operation == nil {
		return (*CreateCheckInOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *CreateCheckInOperation) validateDomain(invocation Invocation) error {
	if invocation.Author == nil {
		return errors.New("check-in requires author")
	}
	if operation.Mood < 1 || operation.Mood > 5 {
		return errors.New("mood must be between 1 and 5")
	}
	if operation.CreatedAt <= 0 {
		return errors.New("check-in timestamp is required")
	}
	return nil
}

type CreateReminderOperation struct {
	ReminderID string
	Schedule   string
	Kind       string
	Note       string
	Delivery   string
	ChannelID  string
	NextRunAt  int64
}

func (*CreateReminderOperation) pluginOperation() {}
func (operation *CreateReminderOperation) cloneOperation() Operation {
	if operation == nil {
		return (*CreateReminderOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *CreateReminderOperation) validateDomain(invocation Invocation) error {
	if invocation.Author == nil {
		return errors.New("reminder requires author")
	}
	if strings.TrimSpace(operation.ReminderID) == "" || utf8.RuneCountInString(operation.ReminderID) > 100 {
		return errors.New("reminder id is invalid")
	}
	if strings.TrimSpace(operation.Schedule) == "" || strings.TrimSpace(operation.Kind) == "" || operation.NextRunAt <= 0 {
		return errors.New("reminder fields are invalid")
	}
	if operation.Delivery != "dm" && operation.Delivery != "channel" {
		return errors.New("reminder delivery is invalid")
	}
	if operation.Delivery == "channel" {
		if invocation.Guild == nil {
			return errors.New("channel reminder requires guild")
		}
		return validateSnowflake(operation.ChannelID, "channel id")
	}
	if operation.ChannelID != "" {
		return errors.New("DM reminder cannot have channel id")
	}
	return nil
}

type DeleteReminderOperation struct{ ReminderID string }

func (*DeleteReminderOperation) pluginOperation() {}
func (operation *DeleteReminderOperation) cloneOperation() Operation {
	if operation == nil {
		return (*DeleteReminderOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *DeleteReminderOperation) validateDomain(invocation Invocation) error {
	if invocation.Author == nil {
		return errors.New("delete reminder requires author")
	}
	if strings.TrimSpace(operation.ReminderID) == "" || utf8.RuneCountInString(operation.ReminderID) > 100 {
		return errors.New("reminder id is invalid")
	}
	return nil
}

type CreateWarningOperation struct {
	UserID    string
	Reason    string
	CreatedAt int64
}

func (*CreateWarningOperation) pluginOperation() {}
func (operation *CreateWarningOperation) cloneOperation() Operation {
	if operation == nil {
		return (*CreateWarningOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *CreateWarningOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil || invocation.Author == nil {
		return errors.New("warning requires guild and author")
	}
	if err := validateSnowflake(operation.UserID, "user id"); err != nil {
		return err
	}
	if strings.TrimSpace(operation.Reason) == "" || utf8.RuneCountInString(operation.Reason) > 1000 {
		return errors.New("warning reason is invalid")
	}
	if operation.CreatedAt <= 0 {
		return errors.New("warning timestamp is required")
	}
	return nil
}

type DeleteWarningOperation struct {
	WarningID    string
	TargetUserID string
}

func (*DeleteWarningOperation) pluginOperation() {}
func (operation *DeleteWarningOperation) cloneOperation() Operation {
	if operation == nil {
		return (*DeleteWarningOperation)(nil)
	}
	copy := *operation
	return &copy
}
func (operation *DeleteWarningOperation) validateDomain(invocation Invocation) error {
	if invocation.Guild == nil || invocation.Author == nil {
		return errors.New("delete warning requires guild and author")
	}
	if strings.TrimSpace(operation.WarningID) == "" || utf8.RuneCountInString(operation.WarningID) > 100 {
		return errors.New("warning id is invalid")
	}
	return validateSnowflake(operation.TargetUserID, "target user id")
}

type AuditTargetType string

const (
	AuditTargetNone  AuditTargetType = ""
	AuditTargetUser  AuditTargetType = "user"
	AuditTargetGuild AuditTargetType = "guild"
)

type AppendAuditOperation struct {
	Action     string
	TargetType AuditTargetType
	TargetID   string
	CreatedAt  int64
	Metadata   Value
}

func (*AppendAuditOperation) pluginOperation() {}
func (operation *AppendAuditOperation) cloneOperation() Operation {
	if operation == nil {
		return (*AppendAuditOperation)(nil)
	}
	copy := *operation
	copy.Metadata = operation.Metadata.Clone()
	return &copy
}
func (operation *AppendAuditOperation) validateDomain(invocation Invocation) error {
	if strings.TrimSpace(operation.Action) == "" || utf8.RuneCountInString(operation.Action) > 100 {
		return errors.New("audit action is invalid")
	}
	if operation.CreatedAt <= 0 {
		return errors.New("audit timestamp is required")
	}
	switch operation.TargetType {
	case AuditTargetNone:
		if operation.TargetID != "" {
			return errors.New("audit target id requires target type")
		}
	case AuditTargetUser, AuditTargetGuild:
		if err := validateSnowflake(operation.TargetID, "audit target id"); err != nil {
			return err
		}
	default:
		return errors.New("audit target type is invalid")
	}
	if operation.Metadata.Kind() != "" {
		if err := operation.Metadata.ValidateState(); err != nil {
			return fmt.Errorf("audit metadata: %w", err)
		}
	}
	if invocation.Guild == nil && operation.TargetType != AuditTargetNone {
		return errors.New("scoped audit requires guild")
	}
	return nil
}

func validateSnowflake(value, field string) error {
	if value == "" || strings.HasPrefix(value, "0") {
		return fmt.Errorf("%s is invalid", field)
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return fmt.Errorf("%s is invalid", field)
	}
	return nil
}
func validateRoleMutation(name string, color *int) error {
	if name != "" && (strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 100) {
		return errors.New("role name is invalid")
	}
	if color != nil && (*color < 0 || *color > 0xFFFFFF) {
		return errors.New("role color is outside RGB range")
	}
	return nil
}
func validateAssetName(value string, max int) error {
	if strings.TrimSpace(value) == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > max {
		return errors.New("asset name is invalid")
	}
	return nil
}
func validateInvocationAttachment(invocation Invocation, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("attachment id is required")
	}
	var options []OptionValue
	if invocation.Command != nil {
		options = invocation.Command.Options
	}
	for _, option := range options {
		if option.Kind == OptionAttachment && option.Attachment != nil && option.Attachment.ID == id {
			return nil
		}
	}
	return errors.New("attachment is not present in invocation")
}
func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
