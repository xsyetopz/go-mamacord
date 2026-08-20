package pluginhost

import (
	"context"
	"errors"
	"fmt"
	contextapi "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/execution/context"
	"strconv"
	"strings"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/persona"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	"github.com/xsyetopz/go-mamacord/internal/scheduling"
	automationstore "github.com/xsyetopz/go-mamacord/internal/storage/automation"
	"github.com/xsyetopz/go-mamacord/internal/timezone"
)

type EffectScope struct {
	PluginID    string
	GuildID     string
	ChannelID   string
	UserID      string
	Attachments []contract.AttachmentRef
}
type DiscordBridge interface {
	GetUser(context.Context, string) (contract.UserDetailsRef, bool, error)
	GetMember(context.Context, string, string) (contract.MemberDetailsRef, bool, error)
	GetGuild(context.Context, string) (contract.GuildDetailsRef, bool, error)
	Execute(context.Context, EffectScope, contract.Operation) error
}

type invocationRuntimeServices struct {
	host       *Host
	plugin     *Plugin
	invocation contract.Invocation
}

func (services invocationRuntimeServices) ReadResource(_ context.Context, resourcePath string) ([]byte, error) {
	if services.plugin == nil {
		return nil, errors.New("plugin resource registry unavailable")
	}
	content, ok := services.plugin.Resources[resourcePath]
	if !ok {
		return nil, errors.New("resource is not declared by the plugin")
	}
	return append([]byte(nil), content...), nil
}
func (services invocationRuntimeServices) Localize(ctx context.Context, request contextapi.LocalizationRequest) (string, error) {
	if services.plugin == nil {
		return "", errors.New("plugin localization registry unavailable")
	}
	data, err := valueObjectMap(request.Data)
	if err != nil {
		return "", err
	}
	if data == nil {
		data = map[string]any{}
	}
	userID := uint64(0)
	if services.invocation.Author != nil {
		userID, _ = parseID(services.invocation.Author.ID)
	}
	if _, exists := data["Pet"]; !exists && userID != 0 {
		data["Pet"] = persona.PetName(request.Locale, userID, request.MessageID)
	}
	if _, exists := data["Mommy"]; !exists {
		data["Mommy"] = persona.Mommy(request.Locale)
	}
	return services.plugin.I18n.Localize(i18n.Config{Locale: request.Locale, PluginID: request.PluginID, MessageID: request.MessageID, TemplateData: data})
}
func (services invocationRuntimeServices) GetUser(ctx context.Context, id string) (contract.UserDetailsRef, bool, error) {
	if services.host.bridge.Discord == nil {
		return contract.UserDetailsRef{}, false, errors.New("Discord reader unavailable")
	}
	return services.host.bridge.Discord.GetUser(ctx, id)
}
func (services invocationRuntimeServices) GetMember(ctx context.Context, guildID, userID string) (contract.MemberDetailsRef, bool, error) {
	if services.host.bridge.Discord == nil {
		return contract.MemberDetailsRef{}, false, errors.New("Discord reader unavailable")
	}
	return services.host.bridge.Discord.GetMember(ctx, guildID, userID)
}
func (services invocationRuntimeServices) GetGuild(ctx context.Context, id string) (contract.GuildDetailsRef, bool, error) {
	if services.host.bridge.Discord == nil {
		return contract.GuildDetailsRef{}, false, errors.New("Discord reader unavailable")
	}
	return services.host.bridge.Discord.GetGuild(ctx, id)
}
func (services invocationRuntimeServices) NormalizeTimezone(_ context.Context, raw string) (string, bool, error) {
	_, name, err := timezone.LoadLocation(strings.TrimSpace(raw))
	if err != nil {
		return "", false, nil
	}
	return name, true, nil
}
func (services invocationRuntimeServices) GetUserSettings(ctx context.Context, userID string) (contract.UserSettingsRef, bool, error) {
	id, err := parseID(userID)
	if err != nil {
		return contract.UserSettingsRef{}, false, err
	}
	storage, err := services.storage()
	if err != nil {
		return contract.UserSettingsRef{}, false, err
	}
	value, ok, err := storage.UserSettings().GetUserSettings(ctx, id)
	if err != nil || !ok {
		return contract.UserSettingsRef{}, ok, err
	}
	out := contract.UserSettingsRef{Timezone: value.Timezone, CreatedAt: value.CreatedAt.UTC().Unix(), UpdatedAt: value.UpdatedAt.UTC().Unix()}
	if value.DMChannelID != nil {
		out.DMChannelID = strconv.FormatUint(*value.DMChannelID, 10)
	}
	return out, true, nil
}
func (services invocationRuntimeServices) ListCheckIns(ctx context.Context, userID string, limit int) ([]contract.CheckInRef, error) {
	id, err := parseID(userID)
	if err != nil {
		return nil, err
	}
	storage, err := services.storage()
	if err != nil {
		return nil, err
	}
	items, err := storage.CheckIns().ListCheckIns(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	out := make([]contract.CheckInRef, len(items))
	for i, item := range items {
		out[i] = contract.CheckInRef{ID: item.ID, Mood: item.Mood, CreatedAt: item.CreatedAt.UTC().Unix()}
	}
	return out, nil
}
func (services invocationRuntimeServices) PlanReminder(ctx context.Context, userID, spec string, nowUnix int64) (contract.ReminderPlanRef, bool, error) {
	id, err := parseID(userID)
	if err != nil {
		return contract.ReminderPlanRef{}, false, err
	}
	schedule, err := scheduling.ParseSchedule(strings.TrimSpace(spec))
	if err != nil {
		return contract.ReminderPlanRef{}, false, nil
	}
	location := time.UTC
	if storage, storeErr := services.storage(); storeErr == nil && storage.UserSettings() != nil {
		if settings, ok, getErr := storage.UserSettings().GetUserSettings(ctx, id); getErr == nil && ok && strings.TrimSpace(settings.Timezone) != "" {
			if loaded, _, loadErr := timezone.LoadLocation(settings.Timezone); loadErr == nil {
				location = loaded
			}
		}
	}
	now := time.Unix(nowUnix, 0).UTC()
	return contract.ReminderPlanRef{Schedule: schedule.Spec(), NextRunAt: schedule.Next(now, location).UTC().Unix()}, true, nil
}
func (services invocationRuntimeServices) ListReminders(ctx context.Context, userID string, limit int) ([]contract.ReminderRef, error) {
	id, err := parseID(userID)
	if err != nil {
		return nil, err
	}
	storage, err := services.storage()
	if err != nil {
		return nil, err
	}
	items, err := storage.Reminders().ListReminders(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	out := make([]contract.ReminderRef, len(items))
	for i, item := range items {
		out[i] = reminderContract(item)
	}
	return out, nil
}
func (services invocationRuntimeServices) CountWarnings(ctx context.Context, guildID, userID string) (int, error) {
	guild, err := parseID(guildID)
	if err != nil {
		return 0, err
	}
	user, err := parseID(userID)
	if err != nil {
		return 0, err
	}
	storage, err := services.storage()
	if err != nil {
		return 0, err
	}
	return storage.Warnings().CountWarnings(ctx, guild, user)
}
func (services invocationRuntimeServices) ListWarnings(ctx context.Context, guildID, userID string, limit int) ([]contract.WarningRef, error) {
	guild, err := parseID(guildID)
	if err != nil {
		return nil, err
	}
	user, err := parseID(userID)
	if err != nil {
		return nil, err
	}
	storage, err := services.storage()
	if err != nil {
		return nil, err
	}
	items, err := storage.Warnings().ListWarnings(ctx, guild, user, limit)
	if err != nil {
		return nil, err
	}
	out := make([]contract.WarningRef, len(items))
	for i, item := range items {
		out[i] = contract.WarningRef{ID: item.ID, UserID: strconv.FormatUint(item.UserID, 10), ModeratorID: strconv.FormatUint(item.ModeratorID, 10), Reason: item.Reason, CreatedAt: item.CreatedAt.UTC().Unix()}
	}
	return out, nil
}
func (services invocationRuntimeServices) storage() (Store, error) {
	if services.host == nil || services.host.store == nil {
		return nil, errors.New("plugin storage unavailable")
	}
	return services.host.store, nil
}
func parseID(value string) (uint64, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid snowflake %q", value)
	}
	return id, nil
}
func reminderContract(item automationstore.Reminder) contract.ReminderRef {
	out := contract.ReminderRef{ReminderDefinition: contract.ReminderDefinition{ID: item.ID, Schedule: item.Schedule, Kind: item.Kind, Note: item.Note}, ReminderDestination: contract.ReminderDestination{Delivery: string(item.Delivery)}, ReminderScheduleState: contract.ReminderScheduleState{Enabled: item.Enabled, NextRunAt: item.NextRunAt.UTC().Unix(), FailureCount: item.FailureCount}, ReminderTimestamps: contract.ReminderTimestamps{CreatedAt: item.CreatedAt.UTC().Unix(), UpdatedAt: item.UpdatedAt.UTC().Unix()}}
	if item.GuildID != nil {
		out.GuildID = strconv.FormatUint(*item.GuildID, 10)
	}
	if item.ChannelID != nil {
		out.ChannelID = strconv.FormatUint(*item.ChannelID, 10)
	}
	if item.LastRunAt != nil {
		out.LastRunAt = item.LastRunAt.UTC().Unix()
	}
	return out
}
func valueObjectMap(value contract.Value) (map[string]any, error) {
	fields, ok := value.Object()
	if !ok {
		return nil, errors.New("localization data must be object")
	}
	out := make(map[string]any, len(fields))
	for _, field := range fields {
		converted, err := valueAny(field.Value)
		if err != nil {
			return nil, err
		}
		out[field.Key] = converted
	}
	return out, nil
}
func valueAny(value contract.Value) (any, error) {
	switch value.Kind() {
	case contract.ValueNull:
		return nil, nil
	case contract.ValueBool:
		v, _ := value.Bool()
		return v, nil
	case contract.ValueInt:
		v, _ := value.Int()
		return v, nil
	case contract.ValueFloat:
		v, _ := value.Float()
		return v, nil
	case contract.ValueString:
		v, _ := value.String()
		return v, nil
	case contract.ValueList:
		items, _ := value.List()
		out := make([]any, len(items))
		for i, item := range items {
			v, err := valueAny(item)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	case contract.ValueObject:
		return valueObjectMap(value)
	default:
		return nil, fmt.Errorf("unsupported value kind %q", value.Kind())
	}
}
