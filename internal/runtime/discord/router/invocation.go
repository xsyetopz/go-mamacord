package router

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

func PluginInvocationContext(user discord.User, member *discord.ResolvedMember, guildID *snowflake.ID, channel discord.InteractionChannel, guild func() (discord.Guild, bool), self func() (discord.OAuth2User, bool), locale string, isOwner bool) contract.Invocation {
	author := UserRef(user)
	channelRef := InteractionChannelRef(channel, SnowflakePtrToString(guildID))
	invocation := contract.Invocation{InvocationActorContext: contract.InvocationActorContext{Author: &author, Channel: &channelRef, Locale: locale}, InvocationExecutionContext: contract.InvocationExecutionContext{IsOwner: isOwner}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}}
	if member != nil {
		value := MemberRef(*member, SnowflakePtrToString(guildID))
		invocation.Member = &value
	}
	if value, ok := guild(); ok {
		ref := GuildRef(value)
		invocation.Guild = &ref
	} else if guildID != nil {
		invocation.Guild = &contract.GuildRef{ID: guildID.String()}
	}
	if value, ok := self(); ok {
		ref := UserRef(value.User)
		invocation.BotUser = &ref
	}
	return invocation
}
