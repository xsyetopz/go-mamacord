package discordruntime

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

func pluginInvocationContext(user discord.User, member *discord.ResolvedMember, guildID *snowflake.ID, channel discord.InteractionChannel, guild func() (discord.Guild, bool), self func() (discord.OAuth2User, bool), locale string, isOwner bool) contract.Invocation {
	author := router.UserRef(user)
	channelRef := router.InteractionChannelRef(channel, router.SnowflakePtrToString(guildID))
	invocation := contract.Invocation{Author: &author, Channel: &channelRef, Locale: locale, IsOwner: isOwner, ResponseState: contract.ResponseUnacknowledged}
	if member != nil {
		value := router.MemberRef(*member, router.SnowflakePtrToString(guildID))
		invocation.Member = &value
	}
	if value, ok := guild(); ok {
		ref := router.GuildRef(value)
		invocation.Guild = &ref
	} else if guildID != nil {
		invocation.Guild = &contract.GuildRef{ID: guildID.String()}
	}
	if value, ok := self(); ok {
		ref := router.UserRef(value.User)
		invocation.BotUser = &ref
	}
	return invocation
}
