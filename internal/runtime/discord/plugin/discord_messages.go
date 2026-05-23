package plugin

import (
	"context"
	"errors"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"

	pluginhostlua "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

func (e Executor) SendDM(
	ctx context.Context,
	pluginID string,
	userID uint64,
	message any,
) (pluginhostlua.MessageResult, error) {
	if e.client() == nil {
		return pluginhostlua.MessageResult{}, errors.New("discord client unavailable")
	}
	if userID == 0 {
		return pluginhostlua.MessageResult{}, errors.New("invalid user id")
	}

	msg, err := ParseAutomationMessage(pluginID, message)
	if err != nil {
		return pluginhostlua.MessageResult{}, err
	}
	if msg.Flags&discord.MessageFlagEphemeral != 0 {
		return pluginhostlua.MessageResult{}, errors.New("ephemeral not supported for send_dm")
	}

	dmID, err := e.ensureDMChannel(ctx, userID)
	if err != nil {
		return pluginhostlua.MessageResult{}, err
	}

	created, err := e.client().Rest.CreateMessage(snowflake.ID(dmID), msg)
	if err != nil {
		return pluginhostlua.MessageResult{}, err
	}

	return pluginhostlua.MessageResult{
		MessageID: uint64(created.ID),
		ChannelID: uint64(created.ChannelID),
		UserID:    userID,
	}, nil
}

func (e Executor) SendChannel(
	ctx context.Context,
	pluginID string,
	channelID uint64,
	message any,
) (pluginhostlua.MessageResult, error) {
	if e.client() == nil {
		return pluginhostlua.MessageResult{}, errors.New("discord client unavailable")
	}
	if channelID == 0 {
		return pluginhostlua.MessageResult{}, errors.New("invalid channel id")
	}

	msg, err := ParseAutomationMessage(pluginID, message)
	if err != nil {
		return pluginhostlua.MessageResult{}, err
	}
	if msg.Flags&discord.MessageFlagEphemeral != 0 {
		return pluginhostlua.MessageResult{}, errors.New("ephemeral not supported for send_channel")
	}

	created, err := e.client().Rest.CreateMessage(snowflake.ID(channelID), msg)
	if err != nil {
		return pluginhostlua.MessageResult{}, err
	}

	return pluginhostlua.MessageResult{
		MessageID: uint64(created.ID),
		ChannelID: uint64(created.ChannelID),
	}, nil
}

func (e Executor) TimeoutMember(ctx context.Context, guildID, userID uint64, until time.Time) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if guildID == 0 || userID == 0 {
		return errors.New("invalid guild or user id")
	}

	_, err := e.client().Rest.UpdateMember(snowflake.ID(guildID), snowflake.ID(userID), discord.MemberUpdate{
		CommunicationDisabledUntil: omit.NewPtr(until.UTC()),
	})
	return err
}
