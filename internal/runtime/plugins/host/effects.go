package pluginhost

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/projection"
	contextapi "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/execution/context"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/generation"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/xsyetopz/go-mamacord/internal/buildinfo"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	automationstore "github.com/xsyetopz/go-mamacord/internal/storage/automation"
	moderationstore "github.com/xsyetopz/go-mamacord/internal/storage/moderation"
	pluginstore "github.com/xsyetopz/go-mamacord/internal/storage/plugins"
	"github.com/xsyetopz/go-mamacord/internal/timezone"
)

var errStateConflict = errors.New("plugin state changed concurrently")

type InvocationPlan struct {
	PluginID  string
	Route     contract.RouteID
	Defer     contract.DeferMode
	Ephemeral bool
}

func (host *Host) PlanCommand(kind, name string, path []string) (InvocationPlan, error) {
	host.mu.RLock()
	command, ok := host.commands[commandLookupKey(kind, name)]
	plugin := host.plugins[command.PluginID]
	host.mu.RUnlock()
	if !ok || plugin == nil {
		return InvocationPlan{}, fmt.Errorf("unknown plugin command %q", name)
	}
	definition, found := findCommandDefinition(plugin.Definition, contract.CommandKind(projection.NormalizeCommandType(kind)), path)
	if !found {
		return InvocationPlan{}, fmt.Errorf("plugin command path %v is not declared", path)
	}
	return InvocationPlan{PluginID: plugin.ID, Route: definition.Route, Defer: definition.Defer, Ephemeral: definition.Ephemeral}, nil
}

func (host *Host) PlanAutocomplete(kind, name string, path []string, optionName string) (InvocationPlan, error) {
	host.mu.RLock()
	command, ok := host.commands[commandLookupKey(kind, name)]
	plugin := host.plugins[command.PluginID]
	host.mu.RUnlock()
	if !ok || plugin == nil {
		return InvocationPlan{}, fmt.Errorf("unknown plugin command %q", name)
	}
	definition, found := findCommandDefinition(plugin.Definition, contract.CommandKind(projection.NormalizeCommandType(kind)), path)
	if !found {
		return InvocationPlan{}, errors.New("plugin autocomplete command is not declared")
	}
	for _, option := range definition.Options {
		if option.Name == optionName && option.Autocomplete != "" {
			return InvocationPlan{PluginID: plugin.ID, Route: option.Autocomplete}, nil
		}
	}
	return InvocationPlan{}, errors.New("plugin autocomplete route is not declared")
}
func (host *Host) PlanEvents(pluginID, event string) ([]InvocationPlan, error) {
	host.mu.RLock()
	plugin := host.plugins[pluginID]
	host.mu.RUnlock()
	if plugin == nil {
		return nil, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	var plans []InvocationPlan
	for _, cog := range plugin.Definition.Cogs {
		for _, item := range cog.Listeners {
			if item.Event == event {
				plans = append(plans, InvocationPlan{PluginID: pluginID, Route: item.Route})
			}
		}
	}
	if len(plans) == 0 {
		return nil, errors.New("plugin event route is not declared")
	}
	return plans, nil
}
func (host *Host) PlanTask(pluginID, taskID string) (InvocationPlan, error) {
	return host.planNonCommand(pluginID, func(def contract.Definition) (contract.RouteID, contract.DeferMode, bool) {
		for _, cog := range def.Cogs {
			for _, item := range cog.Tasks {
				if item.ID == taskID {
					return item.Route, contract.DeferNone, true
				}
			}
		}
		return "", "", false
	})
}
func (host *Host) PlanComponent(pluginID, id string) (InvocationPlan, error) {
	return host.planNonCommand(pluginID, func(def contract.Definition) (contract.RouteID, contract.DeferMode, bool) {
		for _, cog := range def.Cogs {
			for _, item := range cog.Components {
				if item.ID == id {
					return item.Route, item.Defer, true
				}
			}
		}
		return "", "", false
	})
}
func (host *Host) PlanModal(pluginID, id string) (InvocationPlan, error) {
	return host.planNonCommand(pluginID, func(def contract.Definition) (contract.RouteID, contract.DeferMode, bool) {
		for _, cog := range def.Cogs {
			for _, item := range cog.Modals {
				if item.ID == id {
					return item.Route, item.Defer, true
				}
			}
		}
		return "", "", false
	})
}
func (host *Host) planNonCommand(pluginID string, find func(contract.Definition) (contract.RouteID, contract.DeferMode, bool)) (InvocationPlan, error) {
	host.mu.RLock()
	plugin := host.plugins[pluginID]
	host.mu.RUnlock()
	if plugin == nil {
		return InvocationPlan{}, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	route, deferMode, ok := find(plugin.Definition)
	if !ok {
		return InvocationPlan{}, errors.New("plugin route is not declared")
	}
	return InvocationPlan{PluginID: pluginID, Route: route, Defer: deferMode}, nil
}
func findCommandDefinition(definition contract.Definition, kind contract.CommandKind, path []string) (contract.CommandDefinition, bool) {
	for _, cog := range definition.Cogs {
		for _, command := range cog.Commands {
			if command.Kind != kind || len(path) == 0 || command.Name != path[0] {
				continue
			}
			current := command
			valid := true
			for _, part := range path[1:] {
				found := false
				for _, child := range current.Children {
					if child.Name == part {
						current = child
						found = true
						break
					}
				}
				if !found {
					valid = false
					break
				}
			}
			if valid && current.Route != "" {
				return current, true
			}
		}
	}
	return contract.CommandDefinition{}, false
}

func (host *Host) invokePrepared(ctx context.Context, plugin *Plugin, invocation contract.Invocation) (contract.Outcome, error) {
	services := invocationRuntimeServices{host: host, plugin: plugin, invocation: invocation}
	return plugin.Runtime.Invoke(ctx, invocation, contextapi.InvocationServices{Localizer: services, Reader: services, Resources: services, HTTP: host.http, HTTPHosts: append([]string(nil), plugin.Manifest.Permissions.Network.Hosts...), Capabilities: append([]contract.Capability(nil), plugin.Capabilities...)}, host.pluginPrint(plugin.ID))
}

type Admission struct {
	host     *Host
	plugin   *Plugin
	base     contract.Invocation
	prepared contract.Invocation
	used     atomic.Bool
}

func (host *Host) Admit(ctx context.Context, pluginID string, invocation contract.Invocation) (*Admission, contract.Operation, error) {
	invocation, err := normalizeInvocation(pluginID, invocation)
	if err != nil {
		return nil, nil, err
	}
	plugin, err := host.plugin(pluginID)
	if err != nil {
		return nil, nil, err
	}
	prepared, err := host.prepareInvocation(ctx, plugin, invocation)
	if err != nil {
		return nil, nil, err
	}
	denial, allowed, err := host.authorizeInvocation(ctx, plugin, prepared)
	if err != nil {
		return nil, nil, err
	}
	if !allowed {
		return nil, denialForInvocation(prepared, denial), nil
	}
	return &Admission{host: host, plugin: plugin, base: invocation, prepared: prepared}, nil, nil
}
func (admission *Admission) Run(ctx context.Context, responseState contract.ResponseState) (contract.Operation, error) {
	if admission == nil || admission.host == nil || admission.plugin == nil {
		return nil, errors.New("plugin admission is invalid")
	}
	if admission.used.Swap(true) {
		return nil, errors.New("plugin admission was already used")
	}
	return admission.runPrepared(ctx, responseState, true)
}
func (admission *Admission) runPrepared(ctx context.Context, responseState contract.ResponseState, allowRefresh bool) (contract.Operation, error) {
	if allowRefresh {
		admission.host.mu.RLock()
		current := admission.host.plugins[admission.plugin.ID]
		admission.host.mu.RUnlock()
		if current != admission.plugin {
			return admission.readmit(ctx, responseState)
		}
	}
	prepared := admission.prepared
	prepared.ResponseState = responseState
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			var err error
			prepared, err = admission.host.prepareInvocation(ctx, admission.plugin, admission.base)
			if err != nil {
				return nil, err
			}
			prepared.ResponseState = responseState
			denial, allowed, err := admission.host.authorizeInvocation(ctx, admission.plugin, prepared)
			if err != nil {
				return nil, err
			}
			if !allowed {
				return denialForInvocation(prepared, denial), nil
			}
		}
		outcome, err := admission.host.invokePrepared(ctx, admission.plugin, prepared)
		if err != nil {
			if allowRefresh && generation.IsStale(err) {
				return admission.readmit(ctx, responseState)
			}
			return nil, err
		}
		terminal, retry, err := admission.host.executeOutcome(ctx, admission.plugin, prepared, outcome)
		if retry && errors.Is(err, errStateConflict) {
			continue
		}
		return terminal, err
	}
	return nil, errStateConflict
}
func (admission *Admission) readmit(ctx context.Context, responseState contract.ResponseState) (contract.Operation, error) {
	base := admission.base
	base.ResponseState = responseState
	fresh, denial, err := admission.host.Admit(ctx, admission.plugin.ID, base)
	if err != nil || fresh == nil {
		return denial, err
	}
	fresh.used.Store(true)
	return fresh.runPrepared(ctx, responseState, false)
}
func (host *Host) Run(ctx context.Context, pluginID string, invocation contract.Invocation) (contract.Operation, error) {
	admission, denial, err := host.Admit(ctx, pluginID, invocation)
	if err != nil || admission == nil {
		return denial, err
	}
	return admission.Run(ctx, invocation.ResponseState)
}
func (host *Host) plugin(id string) (*Plugin, error) {
	host.mu.RLock()
	plugin := host.plugins[id]
	host.mu.RUnlock()
	if plugin == nil || plugin.Runtime == nil {
		return nil, fmt.Errorf("plugin %q not loaded", id)
	}
	return plugin, nil
}
func (host *Host) pluginPrint(pluginID string) func(string) {
	return func(message string) { host.logger.Debug("plugin print", "plugin", pluginID, "message", message) }
}
func normalizeInvocation(pluginID string, invocation contract.Invocation) (contract.Invocation, error) {
	invocation.PluginID = pluginID
	if invocation.NowUnix <= 0 {
		invocation.NowUnix = time.Now().UTC().Unix()
	}
	if invocation.RandomSeed == 0 {
		var seed [8]byte
		if _, err := rand.Read(seed[:]); err != nil {
			return contract.Invocation{}, err
		}
		invocation.RandomSeed = binary.LittleEndian.Uint64(seed[:])
		if invocation.RandomSeed == 0 {
			invocation.RandomSeed = 1
		}
	}
	info := buildinfo.Current()
	invocation.Runtime = contract.RuntimeRef{Version: info.Version, Description: info.Description, Repository: info.Repository, MascotImageURL: info.MascotImageURL}
	return invocation, nil
}
func (host *Host) prepareInvocation(ctx context.Context, plugin *Plugin, invocation contract.Invocation) (contract.Invocation, error) {
	invocation, err := normalizeInvocation(plugin.ID, invocation)
	if err != nil {
		return contract.Invocation{}, err
	}
	if invocation.Guild != nil && len(plugin.Manifest.StateKeys) != 0 {
		state, err := host.loadState(ctx, plugin, invocation.Guild.ID)
		if err != nil {
			return contract.Invocation{}, err
		}
		invocation.State = state
	}
	return invocation, nil
}
func (host *Host) loadState(ctx context.Context, plugin *Plugin, guildIDText string) ([]contract.StateEntry, error) {
	guildID, err := parseID(guildIDText)
	if err != nil {
		return nil, err
	}
	if host.store == nil {
		return nil, errors.New("plugin storage unavailable")
	}
	versioned, ok := host.store.PluginKV().(pluginstore.VersionedPluginKVStore)
	if !ok {
		return nil, errors.New("plugin KV store does not support versioned state")
	}
	out := make([]contract.StateEntry, 0, len(plugin.Manifest.StateKeys))
	for _, key := range plugin.Manifest.StateKeys {
		value, exists, err := versioned.GetPluginKVVersioned(ctx, guildID, plugin.ID, key)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		decoded, err := decodeContractJSON(value.ValueJSON)
		if err != nil {
			return nil, fmt.Errorf("plugin state %q: %w", key, err)
		}
		out = append(out, contract.StateEntry{Key: key, Value: decoded, Version: value.Version})
	}
	return out, nil
}
func (host *Host) authorizeInvocation(ctx context.Context, plugin *Plugin, invocation contract.Invocation) (contract.Operation, bool, error) {
	checks := checksForRoute(plugin.Definition, invocation.Route)
	granted := map[contract.MemberPermission]bool{}
	if invocation.Member != nil {
		for _, permission := range invocation.Member.Permissions {
			granted[permission] = true
		}
	}
	for _, check := range checks {
		switch check.Kind {
		case contract.CheckGuildOnly:
			if invocation.Guild == nil {
				return nil, false, nil
			}
		case contract.CheckOwnerOnly:
			if !invocation.IsOwner {
				return nil, false, nil
			}
		case contract.CheckHasPermissions:
			if !granted[contract.PermissionAdministrator] {
				for _, permission := range check.Permissions {
					if !granted[permission] {
						return nil, false, nil
					}
				}
			}
		}
	}
	for _, check := range checks {
		if check.Kind != contract.CheckCustom {
			continue
		}
		checkInvocation := invocation
		checkInvocation.Route = check.Route
		checkInvocation.Kind = contract.InvocationCheck
		checkInvocation.ResponseState = ""
		checkInvocation.ModalOrigin = ""
		checkInvocation.Command = nil
		checkInvocation.Autocomplete = nil
		checkInvocation.Component = nil
		checkInvocation.Modal = nil
		checkInvocation.Event = nil
		checkInvocation.Task = nil
		checkInvocation.Check = &contract.CheckInput{ID: string(check.Route)}
		services := invocationRuntimeServices{host: host, plugin: plugin, invocation: checkInvocation}
		decision, err := plugin.Runtime.Check(ctx, checkInvocation, contextapi.InvocationServices{Localizer: services, Reader: services, Resources: services, HTTP: host.http, HTTPHosts: append([]string(nil), plugin.Manifest.Permissions.Network.Hosts...), Capabilities: append([]contract.Capability(nil), plugin.Capabilities...)}, host.pluginPrint(plugin.ID))
		if err != nil {
			return nil, false, err
		}
		switch decision.Kind {
		case contract.CheckAllowed:
		case contract.CheckDeniedSilent:
			return nil, false, nil
		case contract.CheckDeniedMessage:
			return cloneContractOperation(decision.Denial), false, nil
		default:
			return nil, false, errors.New("invalid check decision")
		}
	}
	return nil, true, nil
}
func denialForInvocation(invocation contract.Invocation, denial contract.Operation) contract.Operation {
	if invocation.Kind == contract.InvocationAutocomplete {
		return &contract.AutocompleteChoicesOperation{Choices: []contract.AutocompleteChoice{}}
	}
	if invocation.Kind == contract.InvocationEvent || invocation.Kind == contract.InvocationTask || denial == nil {
		return nil
	}
	message, ok := denial.(*contract.MessageOperation)
	if !ok || invocation.ResponseState != contract.ResponseDeferredUpdate {
		return cloneContractOperation(denial)
	}
	cloned := message.Message.DeepClone()
	return &contract.UpdateOperation{Patch: contract.MessagePatch{Content: contract.OptionalString{Set: true, Value: cloned.Content}, Embeds: contract.OptionalEmbeds{Set: true, Values: cloned.Embeds}, Components: contract.OptionalComponentRows{Set: true, Values: cloned.Components}}}
}

func checksForRoute(definition contract.Definition, route contract.RouteID) []contract.CheckDefinition {
	for _, cog := range definition.Cogs {
		base := append([]contract.CheckDefinition(nil), cog.Checks...)
		if checks, ok := commandChecksForRoute(cog.Commands, route, base, nil); ok {
			return checks
		}
		for _, item := range cog.Components {
			if item.Route == route {
				return append(base, item.Checks...)
			}
		}
		for _, item := range cog.Modals {
			if item.Route == route {
				return append(base, item.Checks...)
			}
		}
		for _, item := range cog.Listeners {
			if item.Route == route {
				return append(base, item.Checks...)
			}
		}
		for _, item := range cog.Tasks {
			if item.Route == route {
				return append(base, item.Checks...)
			}
		}
	}
	return nil
}
func commandChecksForRoute(commands []contract.CommandDefinition, route contract.RouteID, inherited []contract.CheckDefinition, inheritedPermissions []contract.MemberPermission) ([]contract.CheckDefinition, bool) {
	for _, command := range commands {
		checks := append(append([]contract.CheckDefinition(nil), inherited...), command.Checks...)
		permissions := append(append([]contract.MemberPermission(nil), inheritedPermissions...), command.DefaultMemberPermissions...)
		if command.Route == route || commandHasAutocompleteRoute(command, route) {
			if len(permissions) != 0 {
				checks = append(checks, contract.CheckDefinition{Kind: contract.CheckHasPermissions, Permissions: permissions})
			}
			return checks, true
		}
		if found, ok := commandChecksForRoute(command.Children, route, checks, permissions); ok {
			return found, true
		}
	}
	return nil, false
}
func commandHasAutocompleteRoute(command contract.CommandDefinition, route contract.RouteID) bool {
	for _, option := range command.Options {
		if option.Autocomplete == route {
			return true
		}
	}
	return false
}

func (host *Host) executeOutcome(ctx context.Context, plugin *Plugin, invocation contract.Invocation, outcome contract.Outcome) (contract.Operation, bool, error) {
	executed := false
	for _, operation := range outcome.Operations {
		switch value := operation.(type) {
		case *contract.GuardedOperation:
			err := host.executeEffect(ctx, plugin, invocation, value.Operation)
			if errors.Is(err, errStateConflict) {
				return nil, !executed, err
			}
			if err != nil {
				return cloneContractOperation(value.Failure), false, nil
			}
			executed = true
		case *contract.BestEffortOperation:
			if err := host.executeEffect(ctx, plugin, invocation, value.Operation); err != nil {
				host.logger.WarnContext(ctx, "best-effort plugin effect failed", "plugin", plugin.ID, "operation", fmt.Sprintf("%T", value.Operation), "err", err)
			}
			executed = true
		case *contract.MessageOperation, *contract.UpdateOperation, *contract.EditResponseOperation, *contract.ModalOperation, *contract.AutocompleteChoicesOperation:
			return cloneContractOperation(operation), false, nil
		default:
			err := host.executeEffect(ctx, plugin, invocation, operation)
			if errors.Is(err, errStateConflict) {
				return nil, !executed, err
			}
			if err != nil {
				return nil, false, err
			}
			executed = true
		}
	}
	return nil, false, nil
}
func (host *Host) executeEffect(ctx context.Context, plugin *Plugin, invocation contract.Invocation, operation contract.Operation) error {
	if plugin == nil {
		return errors.New("plugin is unavailable")
	}
	for _, required := range contract.RequiredCapabilities(operation) {
		if !slices.Contains(plugin.Capabilities, required) {
			return fmt.Errorf("capability %q is not granted", required)
		}
	}
	switch value := operation.(type) {
	case *contract.KVPutOperation:
		return host.putState(ctx, plugin.ID, invocation, value)
	case *contract.KVDeleteOperation:
		return host.deleteState(ctx, plugin.ID, invocation, value)
	case *contract.SetTimezoneOperation, *contract.ClearTimezoneOperation, *contract.CreateCheckInOperation, *contract.CreateReminderOperation, *contract.DeleteReminderOperation, *contract.CreateWarningOperation, *contract.DeleteWarningOperation, *contract.AppendAuditOperation:
		return host.executeStorageEffect(ctx, invocation, operation)
	default:
		if host.bridge.Discord == nil {
			return errors.New("Discord effect executor unavailable")
		}
		return host.bridge.Discord.Execute(ctx, EffectScope{PluginID: plugin.ID, GuildID: idOf(invocation.Guild), ChannelID: idOf(invocation.Channel), UserID: idOf(invocation.Author), Attachments: invocationAttachments(invocation)}, operation)
	}
}
func idOf(value any) string {
	switch typed := value.(type) {
	case *contract.GuildRef:
		if typed != nil {
			return typed.ID
		}
	case *contract.ChannelRef:
		if typed != nil {
			return typed.ID
		}
	case *contract.UserRef:
		if typed != nil {
			return typed.ID
		}
	}
	return ""
}
func (host *Host) putState(ctx context.Context, pluginID string, invocation contract.Invocation, operation *contract.KVPutOperation) error {
	if host.store == nil {
		return errors.New("plugin storage unavailable")
	}
	if invocation.Guild == nil {
		return errors.New("KV effect requires guild")
	}
	guildID, err := parseID(invocation.Guild.ID)
	if err != nil {
		return err
	}
	encoded, err := encodeContractJSON(operation.Value)
	if err != nil {
		return err
	}
	if operation.ExpectedVersion == nil {
		return host.store.PluginKV().PutPluginKV(ctx, guildID, pluginID, operation.Key, encoded)
	}
	versioned, ok := host.store.PluginKV().(pluginstore.VersionedPluginKVStore)
	if !ok {
		return errors.New("versioned KV unavailable")
	}
	_, swapped, err := versioned.CompareAndSwapPluginKV(ctx, guildID, pluginID, operation.Key, encoded, *operation.ExpectedVersion)
	if err != nil {
		return err
	}
	if !swapped {
		return errStateConflict
	}
	return nil
}
func (host *Host) deleteState(ctx context.Context, pluginID string, invocation contract.Invocation, operation *contract.KVDeleteOperation) error {
	if host.store == nil {
		return errors.New("plugin storage unavailable")
	}
	if invocation.Guild == nil {
		return errors.New("KV effect requires guild")
	}
	guildID, err := parseID(invocation.Guild.ID)
	if err != nil {
		return err
	}
	if operation.ExpectedVersion == nil {
		return host.store.PluginKV().DeletePluginKV(ctx, guildID, pluginID, operation.Key)
	}
	versioned, ok := host.store.PluginKV().(pluginstore.VersionedPluginKVStore)
	if !ok {
		return errors.New("versioned KV unavailable")
	}
	deleted, err := versioned.DeletePluginKVVersion(ctx, guildID, pluginID, operation.Key, *operation.ExpectedVersion)
	if err != nil {
		return err
	}
	if !deleted {
		return errStateConflict
	}
	return nil
}
func (host *Host) executeStorageEffect(ctx context.Context, invocation contract.Invocation, operation contract.Operation) error {
	if host.store == nil {
		return errors.New("plugin storage unavailable")
	}
	authorID, authorErr := parseID(idOf(invocation.Author))
	guildID, guildErr := parseID(idOf(invocation.Guild))
	switch value := operation.(type) {
	case *contract.SetTimezoneOperation:
		if authorErr != nil {
			return authorErr
		}
		_, normalized, err := timezone.LoadLocation(value.Timezone)
		if err != nil {
			return err
		}
		return host.store.UserSettings().UpsertUserTimezone(ctx, authorID, normalized)
	case *contract.ClearTimezoneOperation:
		if authorErr != nil {
			return authorErr
		}
		return host.store.UserSettings().ClearUserTimezone(ctx, authorID)
	case *contract.CreateCheckInOperation:
		if authorErr != nil {
			return authorErr
		}
		return host.store.CheckIns().CreateCheckIn(ctx, automationstore.CheckIn{ID: uuid.NewString(), UserID: authorID, Mood: value.Mood, CreatedAt: time.Unix(value.CreatedAt, 0).UTC()})
	case *contract.CreateReminderOperation:
		if authorErr != nil {
			return authorErr
		}
		reminder := automationstore.Reminder{
			ReminderIdentity:       automationstore.ReminderIdentity{ID: value.ReminderID, UserID: authorID},
			ReminderSchedule:       automationstore.ReminderSchedule{Schedule: value.Schedule, Kind: value.Kind, Note: value.Note},
			ReminderDeliveryTarget: automationstore.ReminderDeliveryTarget{Delivery: automationstore.ReminderDelivery(value.Delivery)},
			ReminderState:          automationstore.ReminderState{Enabled: true, NextRunAt: time.Unix(value.NextRunAt, 0).UTC()},
			ReminderTimestamps: automationstore.ReminderTimestamps{
				CreatedAt: time.Unix(invocation.NowUnix, 0).UTC(),
				UpdatedAt: time.Unix(invocation.NowUnix, 0).UTC(),
			},
		}
		if value.ChannelID != "" {
			channel, err := parseID(value.ChannelID)
			if err != nil {
				return err
			}
			reminder.ChannelID = &channel
		}
		if invocation.Guild != nil {
			guild, _ := parseID(invocation.Guild.ID)
			reminder.GuildID = &guild
		}
		return host.store.Reminders().CreateReminder(ctx, reminder)
	case *contract.DeleteReminderOperation:
		if authorErr != nil {
			return authorErr
		}
		deleted, err := host.store.Reminders().DeleteReminder(ctx, authorID, value.ReminderID)
		if err != nil {
			return err
		}
		if !deleted {
			return errors.New("reminder not found")
		}
		return nil
	case *contract.CreateWarningOperation:
		if authorErr != nil {
			return authorErr
		}
		if guildErr != nil {
			return guildErr
		}
		target, err := parseID(value.UserID)
		if err != nil {
			return err
		}
		return host.store.Warnings().CreateWarning(ctx, moderationstore.Warning{ID: uuid.NewString(), GuildID: guildID, UserID: target, ModeratorID: authorID, Reason: value.Reason, CreatedAt: time.Unix(value.CreatedAt, 0).UTC()})
	case *contract.DeleteWarningOperation:
		if authorErr != nil {
			return authorErr
		}
		if guildErr != nil {
			return guildErr
		}
		target, err := parseID(value.TargetUserID)
		if err != nil {
			return err
		}
		warnings, err := host.store.Warnings().ListWarnings(ctx, guildID, target, 100)
		if err != nil {
			return err
		}
		found := false
		for _, warning := range warnings {
			if warning.ID == value.WarningID {
				found = true
				break
			}
		}
		if !found {
			return errors.New("warning not found")
		}
		return host.store.Warnings().DeleteWarning(ctx, value.WarningID)
	case *contract.AppendAuditOperation:
		return host.appendAudit(ctx, invocation, value)
	default:
		return fmt.Errorf("unsupported storage operation %T", operation)
	}
}
func (host *Host) appendAudit(ctx context.Context, invocation contract.Invocation, value *contract.AppendAuditOperation) error {
	entry := moderationstore.AuditEntry{Action: value.Action, CreatedAt: time.Unix(value.CreatedAt, 0).UTC(), MetaJSON: "{}"}
	if invocation.Guild != nil {
		guild, err := parseID(invocation.Guild.ID)
		if err != nil {
			return err
		}
		entry.GuildID = &guild
	}
	if invocation.Author != nil {
		actor, err := parseID(invocation.Author.ID)
		if err != nil {
			return err
		}
		entry.ActorID = &actor
	}
	if value.TargetType != "" {
		target, err := parseID(value.TargetID)
		if err != nil {
			return err
		}
		kind := moderationstore.TargetType(value.TargetType)
		entry.TargetType = &kind
		entry.TargetID = &target
	}
	if value.Metadata.Kind() != "" {
		encoded, err := encodeContractJSON(value.Metadata)
		if err != nil {
			return err
		}
		entry.MetaJSON = encoded
	}
	return host.store.Audit().Append(ctx, entry)
}
func encodeContractJSON(value contract.Value) (string, error) {
	plain, err := valueAny(value)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(plain)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
func decodeContractJSON(raw string) (contract.Value, error) {
	if len(raw) > contract.MaxStateValueBytes {
		return contract.Value{}, errors.New("state JSON exceeds byte limit")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return contract.Value{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return contract.Value{}, errors.New("state JSON has invalid trailing data")
	}
	decoded, err := plainContractValue(value, 0, new(int))
	if err != nil {
		return contract.Value{}, err
	}
	if err := decoded.Validate(); err != nil {
		return contract.Value{}, err
	}
	return decoded, nil
}
func plainContractValue(raw any, depth int, items *int) (contract.Value, error) {
	if depth > contract.MaxValueDepth {
		return contract.Value{}, errors.New("state exceeds depth")
	}
	switch value := raw.(type) {
	case nil:
		return contract.NullValue(), nil
	case bool:
		return contract.BoolValue(value), nil
	case string:
		return contract.StringValue(value), nil
	case json.Number:
		if integer, err := strconv.ParseInt(string(value), 10, 64); err == nil {
			return contract.IntValue(integer), nil
		}
		number, err := strconv.ParseFloat(string(value), 64)
		if err != nil {
			return contract.Value{}, err
		}
		return contract.FloatValue(number)
	case []any:
		*items += len(value)
		out := make([]contract.Value, len(value))
		for i, item := range value {
			converted, err := plainContractValue(item, depth+1, items)
			if err != nil {
				return contract.Value{}, err
			}
			out[i] = converted
		}
		return contract.ListValue(out)
	case map[string]any:
		*items += len(value)
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fields := make([]contract.Field, 0, len(keys))
		for _, key := range keys {
			converted, err := plainContractValue(value[key], depth+1, items)
			if err != nil {
				return contract.Value{}, err
			}
			fields = append(fields, contract.Field{Key: key, Value: converted})
		}
		return contract.ObjectValue(fields)
	default:
		return contract.Value{}, fmt.Errorf("unsupported JSON value %T", raw)
	}
}

func cloneContractOperation(operation contract.Operation) contract.Operation {
	if operation == nil {
		return nil
	}
	return contract.Outcome{Operations: []contract.Operation{operation}}.DeepClone().Operations[0]
}

func invocationAttachments(invocation contract.Invocation) []contract.AttachmentRef {
	if invocation.Command == nil {
		return nil
	}
	out := []contract.AttachmentRef{}
	for _, option := range invocation.Command.Options {
		if option.Attachment != nil {
			out = append(out, *option.Attachment)
		}
	}
	return out
}
