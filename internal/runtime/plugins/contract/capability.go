package contract

type Capability string

const (
	CapabilityStorageKV           Capability = "storage.kv"
	CapabilityStorageUserSettings Capability = "storage.user_settings"
	CapabilityStorageCheckIns     Capability = "storage.checkins"
	CapabilityStorageReminders    Capability = "storage.reminders"
	CapabilityStorageWarnings     Capability = "storage.warnings"
	CapabilityStorageAudit        Capability = "storage.audit"
	CapabilityDiscordUsers        Capability = "discord.users"
	CapabilityDiscordGuilds       Capability = "discord.guilds"
	CapabilityDiscordMessages     Capability = "discord.messages"
	CapabilityDiscordMembers      Capability = "discord.members"
	CapabilityDiscordChannels     Capability = "discord.channels"
	CapabilityDiscordRoles        Capability = "discord.roles"
	CapabilityDiscordEmojis       Capability = "discord.emojis"
	CapabilityDiscordStickers     Capability = "discord.stickers"
	CapabilityNetworkHTTP         Capability = "network.http"
	CapabilityResourcesRead       Capability = "resources.read"
)

func RequiredCapabilities(operation Operation) []Capability {
	if guarded, ok := operation.(*GuardedOperation); ok && guarded != nil {
		return RequiredCapabilities(guarded.Operation)
	}
	if bestEffort, ok := operation.(*BestEffortOperation); ok && bestEffort != nil {
		return RequiredCapabilities(bestEffort.Operation)
	}
	switch operation.(type) {
	case *KVPutOperation, *KVDeleteOperation:
		return []Capability{CapabilityStorageKV}
	case *SetTimezoneOperation, *ClearTimezoneOperation:
		return []Capability{CapabilityStorageUserSettings}
	case *CreateCheckInOperation:
		return []Capability{CapabilityStorageCheckIns}
	case *CreateReminderOperation, *DeleteReminderOperation:
		return []Capability{CapabilityStorageReminders}
	case *CreateWarningOperation, *DeleteWarningOperation:
		return []Capability{CapabilityStorageWarnings}
	case *AppendAuditOperation:
		return []Capability{CapabilityStorageAudit}
	case *SendChannelOperation, *SendDMOperation, *PurgeMessagesOperation:
		return []Capability{CapabilityDiscordMessages}
	case *TimeoutMemberOperation, *SetNicknameOperation:
		return []Capability{CapabilityDiscordMembers}
	case *SetSlowmodeOperation:
		return []Capability{CapabilityDiscordChannels}
	case *CreateRoleOperation, *EditRoleOperation, *DeleteRoleOperation, *MemberRoleOperation:
		return []Capability{CapabilityDiscordRoles}
	case *CreateEmojiOperation, *EditEmojiOperation, *DeleteEmojiOperation:
		return []Capability{CapabilityDiscordEmojis}
	case *CreateStickerOperation, *EditStickerOperation, *DeleteStickerOperation:
		return []Capability{CapabilityDiscordStickers}
	default:
		return nil
	}
}
