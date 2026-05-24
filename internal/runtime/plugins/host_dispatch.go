package pluginhost

import (
	"context"
	"fmt"
	"strings"

	luaplugin "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

func defaultEphemeralForCommand(cmd Command, opts map[string]any) bool {
	if NormalizeCommandType(cmd.Type) != CommandTypeSlash {
		return cmd.Ephemeral
	}
	if opts == nil {
		return cmd.Ephemeral
	}

	sub := readPayloadString(opts, "__subcommand")
	if sub == "" {
		return cmd.Ephemeral
	}

	group := readPayloadString(opts, "__group")

	if group != "" {
		return defaultEphemeralFromGroups(cmd.Groups, group, sub, cmd.Ephemeral)
	}

	return defaultEphemeralFromSubcommands(cmd.Subcommands, sub, cmd.Ephemeral)
}

func readPayloadString(opts map[string]any, key string) string {
	if opts == nil {
		return ""
	}
	v, ok := opts[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func defaultEphemeralFromGroups(groups []CommandGroup, group, sub string, fallback bool) bool {
	for _, g := range groups {
		if strings.TrimSpace(g.Name) != group {
			continue
		}
		return defaultEphemeralFromSubcommands(g.Subcommands, sub, fallback)
	}
	return fallback
}

func defaultEphemeralFromSubcommands(subs []Subcommand, sub string, fallback bool) bool {
	for _, sc := range subs {
		if strings.TrimSpace(sc.Name) != sub {
			continue
		}
		if sc.Ephemeral != nil {
			return *sc.Ephemeral
		}
		return fallback
	}
	return fallback
}

func autocompleteRouteID(cmd Command, group, subcommand, option string) string {
	group = strings.TrimSpace(group)
	subcommand = strings.TrimSpace(subcommand)
	option = strings.TrimSpace(option)
	if option == "" {
		return ""
	}

	if group != "" {
		for _, cmdGroup := range cmd.Groups {
			if strings.TrimSpace(cmdGroup.Name) != group {
				continue
			}
			return autocompleteRouteIDFromOptions(subcommandOptions(cmdGroup.Subcommands, subcommand), option)
		}
		return ""
	}
	if subcommand != "" {
		return autocompleteRouteIDFromOptions(subcommandOptions(cmd.Subcommands, subcommand), option)
	}
	return autocompleteRouteIDFromOptions(cmd.Options, option)
}

func subcommandOptions(subcommands []Subcommand, name string) []CommandOption {
	for _, subcommand := range subcommands {
		if strings.TrimSpace(subcommand.Name) == name {
			return subcommand.Options
		}
	}
	return nil
}

func autocompleteRouteIDFromOptions(options []CommandOption, option string) string {
	for _, opt := range options {
		if strings.TrimSpace(opt.Name) == option {
			return strings.TrimSpace(opt.Autocomplete)
		}
	}
	return ""
}
func (m *Host) HandleSlash(ctx context.Context, cmdName string, payload Payload) (luaplugin.EncodedValue, bool, string, error) {
	return m.handleCommand(ctx, CommandTypeSlash, cmdName, payload)
}

func (m *Host) HandleUserCommand(ctx context.Context, cmdName string, payload Payload) (luaplugin.EncodedValue, bool, string, error) {
	return m.handleCommand(ctx, CommandTypeUser, cmdName, payload)
}

func (m *Host) HandleMessageCommand(ctx context.Context, cmdName string, payload Payload) (luaplugin.EncodedValue, bool, string, error) {
	return m.handleCommand(ctx, CommandTypeMessage, cmdName, payload)
}

func (m *Host) handleCommand(ctx context.Context, kind, cmdName string, payload Payload) (luaplugin.EncodedValue, bool, string, error) {
	m.mu.RLock()
	cmd, ok := m.commands[commandLookupKey(kind, cmdName)]
	if !ok {
		m.mu.RUnlock()
		return nil, false, "", fmt.Errorf("unknown plugin %s command %q", NormalizeCommandType(kind), cmdName)
	}

	pl, ok := m.plugins[cmd.PluginID]
	if !ok || pl.VM == nil {
		m.mu.RUnlock()
		return nil, false, "", fmt.Errorf("plugin %q not loaded", cmd.PluginID)
	}
	m.mu.RUnlock()

	var (
		res      luaplugin.EncodedValue
		hasValue bool
		err      error
	)
	if pl.VM.HasDefinition() {
		res, hasValue, err = pl.VM.CallEncodedRoute(ctx, routeKindForCommandType(kind), cmdName, luaplugin.Payload{
			GuildID:     payload.GuildID,
			ChannelID:   payload.ChannelID,
			UserID:      payload.UserID,
			Locale:      payload.Locale,
			IsOwner:     payload.IsOwner,
			Options:     payload.Options,
			Interaction: payload.Interaction,
		})
	} else {
		res, hasValue, err = pl.VM.CallEncodedHandle(ctx, "Handle", cmdName, luaplugin.Payload{
			GuildID:   payload.GuildID,
			ChannelID: payload.ChannelID,
			UserID:    payload.UserID,
			Locale:    payload.Locale,
			IsOwner:   payload.IsOwner,
			Options:   payload.Options,
		})
	}
	if err != nil {
		return nil, false, pl.ID, err
	}

	defaultEphemeral := defaultEphemeralForCommand(cmd.Command, payload.Options)
	if !hasValue {
		return nil, defaultEphemeral, pl.ID, nil
	}
	return res, defaultEphemeral, pl.ID, nil
}

func routeKindForCommandType(kind string) luaplugin.RouteKind {
	switch NormalizeCommandType(kind) {
	case CommandTypeUser:
		return luaplugin.RouteUserCommand
	case CommandTypeMessage:
		return luaplugin.RouteMessageCommand
	default:
		return luaplugin.RouteCommand
	}
}

func (m *Host) HandleAutocomplete(
	ctx context.Context,
	cmdName string,
	group string,
	subcommand string,
	option string,
	payload Payload,
) (luaplugin.EncodedValue, string, error) {
	m.mu.RLock()
	cmd, ok := m.commands[commandLookupKey(CommandTypeSlash, cmdName)]
	if !ok {
		m.mu.RUnlock()
		return nil, "", fmt.Errorf("unknown plugin slash command %q", cmdName)
	}

	pl, ok := m.plugins[cmd.PluginID]
	if !ok || pl.VM == nil {
		m.mu.RUnlock()
		return nil, "", fmt.Errorf("plugin %q not loaded", cmd.PluginID)
	}
	m.mu.RUnlock()

	if !pl.VM.HasDefinition() {
		return nil, pl.ID, fmt.Errorf("plugin %q does not support autocomplete", pl.ID)
	}

	routeID := autocompleteRouteID(cmd.Command, group, subcommand, option)
	if routeID == "" {
		return nil, pl.ID, fmt.Errorf("plugin command %q has no autocomplete route for option %q", cmdName, option)
	}

	res, err := pl.VM.CallAutocomplete(ctx, routeID, luaplugin.Payload{
		GuildID:     payload.GuildID,
		ChannelID:   payload.ChannelID,
		UserID:      payload.UserID,
		Locale:      payload.Locale,
		IsOwner:     payload.IsOwner,
		Options:     payload.Options,
		Interaction: payload.Interaction,
	})
	return res, pl.ID, err
}

func (m *Host) HandleComponent(ctx context.Context, pluginID, localID string, payload Payload) (luaplugin.EncodedValue, bool, error) {
	m.mu.RLock()
	pl, ok := m.plugins[pluginID]
	if !ok || pl.VM == nil {
		m.mu.RUnlock()
		return nil, false, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	vm := pl.VM
	m.mu.RUnlock()

	if vm.HasDefinition() {
		return vm.CallEncodedRoute(ctx, luaplugin.RouteComponent, localID, luaplugin.Payload{
			GuildID:   payload.GuildID,
			ChannelID: payload.ChannelID,
			UserID:    payload.UserID,
			Locale:    payload.Locale,
			IsOwner:   payload.IsOwner,
			Options:   payload.Options,
		})
	}

	if !vm.HasFunc("HandleComponent") {
		return nil, false, fmt.Errorf("plugin %q does not implement HandleComponent", pluginID)
	}

	return vm.CallEncodedHandle(ctx, "HandleComponent", localID, luaplugin.Payload{
		GuildID:   payload.GuildID,
		ChannelID: payload.ChannelID,
		UserID:    payload.UserID,
		Locale:    payload.Locale,
		IsOwner:   payload.IsOwner,
		Options:   payload.Options,
	})
}

func (m *Host) HandleModal(ctx context.Context, pluginID, localID string, payload Payload) (luaplugin.EncodedValue, bool, error) {
	m.mu.RLock()
	pl, ok := m.plugins[pluginID]
	if !ok || pl.VM == nil {
		m.mu.RUnlock()
		return nil, false, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	vm := pl.VM
	m.mu.RUnlock()

	if vm.HasDefinition() {
		return vm.CallEncodedRoute(ctx, luaplugin.RouteModal, localID, luaplugin.Payload{
			GuildID:   payload.GuildID,
			ChannelID: payload.ChannelID,
			UserID:    payload.UserID,
			Locale:    payload.Locale,
			IsOwner:   payload.IsOwner,
			Options:   payload.Options,
		})
	}

	if !vm.HasFunc("HandleModal") {
		return nil, false, fmt.Errorf("plugin %q does not implement HandleModal", pluginID)
	}

	return vm.CallEncodedHandle(ctx, "HandleModal", localID, luaplugin.Payload{
		GuildID:   payload.GuildID,
		ChannelID: payload.ChannelID,
		UserID:    payload.UserID,
		Locale:    payload.Locale,
		IsOwner:   payload.IsOwner,
		Options:   payload.Options,
	})
}

func (m *Host) HandleEvent(ctx context.Context, pluginID, eventName string, payload Payload) (luaplugin.EncodedValue, bool, error) {
	m.mu.RLock()
	pl, ok := m.plugins[pluginID]
	if !ok || pl.VM == nil {
		m.mu.RUnlock()
		return nil, false, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	vm := pl.VM
	m.mu.RUnlock()

	if vm.HasDefinition() {
		return vm.CallEncodedRoute(ctx, luaplugin.RouteEvent, eventName, luaplugin.Payload{
			GuildID:   payload.GuildID,
			ChannelID: payload.ChannelID,
			UserID:    payload.UserID,
			Locale:    payload.Locale,
			IsOwner:   payload.IsOwner,
			Options:   payload.Options,
		})
	}

	if !vm.HasFunc("HandleEvent") {
		return nil, false, fmt.Errorf("plugin %q does not implement HandleEvent", pluginID)
	}

	return vm.CallEncodedHandle(ctx, "HandleEvent", eventName, luaplugin.Payload{
		GuildID:   payload.GuildID,
		ChannelID: payload.ChannelID,
		UserID:    payload.UserID,
		Locale:    payload.Locale,
		IsOwner:   payload.IsOwner,
		Options:   payload.Options,
	})
}

func (m *Host) HandleJob(ctx context.Context, pluginID, jobID string, payload Payload) (luaplugin.EncodedValue, bool, error) {
	m.mu.RLock()
	pl, ok := m.plugins[pluginID]
	if !ok || pl.VM == nil {
		m.mu.RUnlock()
		return nil, false, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	vm := pl.VM
	m.mu.RUnlock()

	if vm.HasDefinition() {
		return vm.CallEncodedRoute(ctx, luaplugin.RouteJob, jobID, luaplugin.Payload{
			GuildID:   payload.GuildID,
			ChannelID: payload.ChannelID,
			UserID:    payload.UserID,
			Locale:    payload.Locale,
			IsOwner:   payload.IsOwner,
			Options:   payload.Options,
		})
	}

	if !vm.HasFunc("HandleJob") {
		return nil, false, fmt.Errorf("plugin %q does not implement HandleJob", pluginID)
	}

	return vm.CallEncodedHandle(ctx, "HandleJob", jobID, luaplugin.Payload{
		GuildID:   payload.GuildID,
		ChannelID: payload.ChannelID,
		UserID:    payload.UserID,
		Locale:    payload.Locale,
		IsOwner:   payload.IsOwner,
		Options:   payload.Options,
	})
}
