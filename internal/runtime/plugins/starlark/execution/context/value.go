package contextapi

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/internal/evaluation"
	"math/rand/v2"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

var localizationMessageIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,199}$`)

type contextValue struct {
	context    context.Context
	invocation contract.Invocation
	services   InvocationServices
	maxCalls   int
	calls      int
	random     *rand.Rand
}

func NewValue(ctx context.Context, invocation contract.Invocation, services InvocationServices, maxCalls int) *contextValue {
	return &contextValue{
		context:    ctx,
		invocation: invocation.DeepClone(),
		services:   services,
		maxCalls:   maxCalls,
		random:     rand.New(rand.NewPCG(invocation.RandomSeed, invocation.RandomSeed^0x9e3779b97f4a7c15)),
	}
}
func (*contextValue) String() string         { return "<mamacord context>" }
func (*contextValue) Type() string           { return "mamacord.context" }
func (*contextValue) Freeze()                {}
func (*contextValue) Truth() starlarkgo.Bool { return starlarkgo.True }
func (*contextValue) Hash() (uint32, error)  { return 0, errors.New("unhashable: mamacord.context") }
func (value *contextValue) AttrNames() []string {
	return []string{"plugin_id", "generation_id", "route", "kind", "locale", "now_unix", "runtime", "is_owner", "guild", "channel", "author", "bot_user", "member", "command_path", "component_id", "selected_values", "modal_fields", "option", "state", "state_version", "t", "random_int", "random_choice", "new_id", "get_user", "get_member", "get_guild", "normalize_timezone", "user_settings", "list_checkins", "plan_reminder", "list_reminders", "count_warnings", "list_warnings", "http_get_json", "resource"}
}
func (value *contextValue) Attr(name string) (starlarkgo.Value, error) {
	switch name {
	case "plugin_id":
		return starlarkgo.String(value.invocation.PluginID), nil
	case "generation_id":
		return starlarkgo.String(value.invocation.Generation), nil
	case "route":
		return starlarkgo.String(value.invocation.Route), nil
	case "kind":
		return starlarkgo.String(value.invocation.Kind), nil
	case "locale":
		return starlarkgo.String(value.invocation.Locale), nil
	case "now_unix":
		return starlarkgo.MakeInt64(value.invocation.NowUnix), nil
	case "runtime":
		result, err := frozenDict(map[string]starlarkgo.Value{
			"version":          starlarkgo.String(value.invocation.Runtime.Version),
			"description":      starlarkgo.String(value.invocation.Runtime.Description),
			"repository":       starlarkgo.String(value.invocation.Runtime.Repository),
			"mascot_image_url": starlarkgo.String(value.invocation.Runtime.MascotImageURL),
		})
		return result, err
	case "is_owner":
		return starlarkgo.Bool(value.invocation.IsOwner), nil
	case "guild":
		return guildValue(value.invocation.Guild), nil
	case "channel":
		return channelValue(value.invocation.Channel), nil
	case "author":
		return userValue(value.invocation.Author), nil
	case "bot_user":
		return userValue(value.invocation.BotUser), nil
	case "member":
		return memberValue(value.invocation.Member), nil
	case "command_path":
		if value.invocation.Command == nil {
			return starlarkgo.None, nil
		}
		items := make([]starlarkgo.Value, len(value.invocation.Command.Path))
		for i, item := range value.invocation.Command.Path {
			items[i] = starlarkgo.String(item)
		}
		tuple := starlarkgo.Tuple(items)
		tuple.Freeze()
		return tuple, nil
	case "component_id":
		if value.invocation.Component == nil {
			return starlarkgo.None, nil
		}
		return starlarkgo.String(value.invocation.Component.ID), nil
	case "selected_values":
		if value.invocation.Component == nil {
			return starlarkgo.None, nil
		}
		items := make([]starlarkgo.Value, 0, len(value.invocation.Component.Values))
		for _, option := range value.invocation.Component.Values {
			if option.Kind == contract.OptionString {
				items = append(items, starlarkgo.String(option.String))
			}
		}
		list := starlarkgo.NewList(items)
		list.Freeze()
		return list, nil
	case "modal_fields":
		if value.invocation.Modal == nil {
			return starlarkgo.None, nil
		}
		dict := starlarkgo.NewDict(len(value.invocation.Modal.Fields))
		for _, field := range value.invocation.Modal.Fields {
			if err := dict.SetKey(starlarkgo.String(field.Name), starlarkgo.String(field.Value)); err != nil {
				return nil, err
			}
		}
		dict.Freeze()
		return dict, nil
	case "option":
		return starlarkgo.NewBuiltin("context.option", value.option), nil
	case "state":
		return starlarkgo.NewBuiltin("context.state", value.state), nil
	case "state_version":
		return starlarkgo.NewBuiltin("context.state_version", value.stateVersion), nil
	case "t":
		return starlarkgo.NewBuiltin("context.t", value.localize), nil
	case "random_int":
		return starlarkgo.NewBuiltin("context.random_int", value.randomInt), nil
	case "random_choice":
		return starlarkgo.NewBuiltin("context.random_choice", value.randomChoice), nil
	case "new_id":
		return starlarkgo.NewBuiltin("context.new_id", value.newID), nil
	case "get_user":
		return starlarkgo.NewBuiltin("context.get_user", value.getUser), nil
	case "get_member":
		return starlarkgo.NewBuiltin("context.get_member", value.getMember), nil
	case "get_guild":
		return starlarkgo.NewBuiltin("context.get_guild", value.getGuild), nil
	case "normalize_timezone":
		return starlarkgo.NewBuiltin("context.normalize_timezone", value.normalizeTimezone), nil
	case "user_settings":
		return starlarkgo.NewBuiltin("context.user_settings", value.userSettings), nil
	case "list_checkins":
		return starlarkgo.NewBuiltin("context.list_checkins", value.listCheckIns), nil
	case "plan_reminder":
		return starlarkgo.NewBuiltin("context.plan_reminder", value.planReminder), nil
	case "list_reminders":
		return starlarkgo.NewBuiltin("context.list_reminders", value.listReminders), nil
	case "count_warnings":
		return starlarkgo.NewBuiltin("context.count_warnings", value.countWarnings), nil
	case "list_warnings":
		return starlarkgo.NewBuiltin("context.list_warnings", value.listWarnings), nil
	case "http_get_json":
		return starlarkgo.NewBuiltin("context.http_get_json", value.httpGetJSON), nil
	case "resource":
		return starlarkgo.NewBuiltin("context.resource", value.resource), nil
	default:
		return nil, nil
	}
}
func (value *contextValue) option(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var name string
	if err := starlarkgo.UnpackArgs("context.option", args, kwargs, "name", &name); err != nil {
		return nil, err
	}
	var options []contract.OptionValue
	if value.invocation.Command != nil {
		options = value.invocation.Command.Options
	} else if value.invocation.Autocomplete != nil {
		options = value.invocation.Autocomplete.Options
		if value.invocation.Autocomplete.Focused.Name == name {
			return optionValue(value.invocation.Autocomplete.Focused)
		}
	} else if value.invocation.Component != nil {
		options = value.invocation.Component.Values
	}
	for _, option := range options {
		if option.Name == name {
			return optionValue(option)
		}
	}
	return starlarkgo.None, nil
}

func (value *contextValue) newID(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	if err := starlarkgo.UnpackArgs("context.new_id", args, kwargs); err != nil {
		return nil, err
	}
	var raw uuid.UUID
	binary.BigEndian.PutUint64(raw[:8], value.random.Uint64())
	binary.BigEndian.PutUint64(raw[8:], value.random.Uint64())
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return starlarkgo.String(raw.String()), nil
}

func (value *contextValue) randomInt(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var minimum, maximum int64
	if err := starlarkgo.UnpackArgs("context.random_int", args, kwargs, "minimum", &minimum, "maximum", &maximum); err != nil {
		return nil, err
	}
	if maximum < minimum {
		return nil, errors.New("maximum must be at least minimum")
	}
	span := uint64(maximum) - uint64(minimum) + 1
	if span == 0 {
		return nil, errors.New("random integer range is too wide")
	}
	return starlarkgo.MakeInt64(minimum + int64(value.random.Uint64N(span))), nil
}

func (value *contextValue) randomChoice(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var items starlarkgo.Indexable
	if err := starlarkgo.UnpackArgs("context.random_choice", args, kwargs, "items", &items); err != nil {
		return nil, err
	}
	if items.Len() == 0 {
		return nil, errors.New("items cannot be empty")
	}
	if items.Len() > evaluation.MaxCollectionItems {
		return nil, fmt.Errorf("items exceed %d", evaluation.MaxCollectionItems)
	}
	return items.Index(int(value.random.Uint64N(uint64(items.Len())))), nil
}

func (value *contextValue) state(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var key string
	var fallback starlarkgo.Value = starlarkgo.None
	if err := starlarkgo.UnpackArgs("context.state", args, kwargs, "key", &key, "default?", &fallback); err != nil {
		return nil, err
	}
	for _, entry := range value.invocation.State {
		if entry.Key == key {
			return evaluation.RaisePersistentValue(entry.Value)
		}
	}
	return fallback, nil
}

func (value *contextValue) stateVersion(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var key string
	if err := starlarkgo.UnpackArgs("context.state_version", args, kwargs, "key", &key); err != nil {
		return nil, err
	}
	for _, entry := range value.invocation.State {
		if entry.Key == key {
			return starlarkgo.MakeUint64(entry.Version), nil
		}
	}
	return starlarkgo.MakeUint64(0), nil
}

func (value *contextValue) localize(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	if value.services.Localizer == nil {
		return nil, errors.New("localization service is unavailable")
	}
	if value.calls >= value.maxCalls {
		return nil, fmt.Errorf("host call limit %d exceeded", value.maxCalls)
	}
	var messageID, locale string
	var dataValue starlarkgo.Value
	if err := starlarkgo.UnpackArgs("context.t", args, kwargs, "message_id", &messageID, "data?", &dataValue, "locale?", &locale); err != nil {
		return nil, err
	}
	if !localizationMessageIDPattern.MatchString(messageID) {
		return nil, errors.New("localization message_id is invalid")
	}
	if locale != strings.TrimSpace(locale) || len(locale) > 35 || !utf8.ValidString(locale) {
		return nil, errors.New("localization locale is invalid")
	}
	if locale == "" {
		locale = value.invocation.Locale
	}
	data, err := contract.ObjectValue(nil)
	if err != nil {
		return nil, err
	}
	if dataValue != nil && dataValue != starlarkgo.None {
		data, err = evaluation.LowerPersistentValue(dataValue)
		if err != nil {
			return nil, fmt.Errorf("localization data: %w", err)
		}
		if data.Kind() != contract.ValueObject {
			return nil, errors.New("localization data must be a dict")
		}
	}
	value.calls++
	localized, err := value.services.Localizer.Localize(value.context, LocalizationRequest{PluginID: value.invocation.PluginID, Locale: locale, MessageID: messageID, Data: data})
	if err != nil {
		return nil, fmt.Errorf("localize %q: %w", messageID, err)
	}
	if !utf8.ValidString(localized) || len(localized) > 64*1024 {
		return nil, errors.New("localized text exceeds limit")
	}
	return starlarkgo.String(localized), nil
}

func optionValue(value contract.OptionValue) (starlarkgo.Value, error) {
	switch value.Kind {
	case contract.OptionString:
		return starlarkgo.String(value.String), nil
	case contract.OptionBoolean:
		return starlarkgo.Bool(value.Boolean), nil
	case contract.OptionInteger:
		return starlarkgo.MakeInt64(value.Integer), nil
	case contract.OptionNumber:
		return starlarkgo.Float(value.Number), nil
	case contract.OptionUser:
		return userValue(value.User), nil
	case contract.OptionChannel:
		return channelValue(value.Channel), nil
	case contract.OptionRole:
		return roleValue(value.Role), nil
	case contract.OptionMentionable:
		if value.Mentionable.User != nil {
			return userValue(value.Mentionable.User), nil
		}
		return roleValue(value.Mentionable.Role), nil
	case contract.OptionAttachment:
		if value.Attachment == nil {
			return starlarkgo.None, nil
		}
		return frozenDict(map[string]starlarkgo.Value{"id": starlarkgo.String(value.Attachment.ID), "filename": starlarkgo.String(value.Attachment.Filename), "url": starlarkgo.String(value.Attachment.URL), "content_type": starlarkgo.String(value.Attachment.ContentType), "size": starlarkgo.MakeInt64(value.Attachment.Size), "width": starlarkgo.MakeInt(value.Attachment.Width), "height": starlarkgo.MakeInt(value.Attachment.Height)})
	default:
		return nil, fmt.Errorf("unsupported option kind %q", value.Kind)
	}
}
func guildValue(value *contract.GuildRef) starlarkgo.Value {
	if value == nil {
		return starlarkgo.None
	}
	result, _ := frozenDict(map[string]starlarkgo.Value{"id": starlarkgo.String(value.ID), "name": starlarkgo.String(value.Name)})
	return result
}
func channelValue(value *contract.ChannelRef) starlarkgo.Value {
	if value == nil {
		return starlarkgo.None
	}
	result, _ := frozenDict(map[string]starlarkgo.Value{"id": starlarkgo.String(value.ID), "guild_id": starlarkgo.String(value.GuildID), "name": starlarkgo.String(value.Name), "kind": starlarkgo.String(value.Kind), "parent_id": starlarkgo.String(value.ParentID), "mention": starlarkgo.String(value.Mention), "permission_bits": starlarkgo.MakeUint64(value.PermissionBits), "created_at": starlarkgo.MakeInt64(value.CreatedAt)})
	return result
}
func userValue(value *contract.UserRef) starlarkgo.Value {
	if value == nil {
		return starlarkgo.None
	}
	result, _ := frozenDict(map[string]starlarkgo.Value{"id": starlarkgo.String(value.ID), "username": starlarkgo.String(value.Username), "name": starlarkgo.String(value.Name), "avatar_url": starlarkgo.String(value.AvatarURL), "bot": starlarkgo.Bool(value.Bot), "system": starlarkgo.Bool(value.System)})
	return result
}
func roleValue(value *contract.RoleRef) starlarkgo.Value {
	if value == nil {
		return starlarkgo.None
	}
	permissions := make([]starlarkgo.Value, len(value.Permissions))
	for i, p := range value.Permissions {
		permissions[i] = starlarkgo.String(p)
	}
	result, _ := frozenDict(map[string]starlarkgo.Value{"id": starlarkgo.String(value.ID), "guild_id": starlarkgo.String(value.GuildID), "name": starlarkgo.String(value.Name), "position": starlarkgo.MakeInt(value.Position), "permissions": starlarkgo.NewList(permissions), "mention": starlarkgo.String(value.Mention), "color": starlarkgo.MakeInt(value.Color), "hoist": starlarkgo.Bool(value.Hoist), "mentionable": starlarkgo.Bool(value.Mentionable), "managed": starlarkgo.Bool(value.Managed), "permission_bits": starlarkgo.MakeUint64(value.PermissionBits), "created_at": starlarkgo.MakeInt64(value.CreatedAt)})
	return result
}
func memberValue(value *contract.MemberRef) starlarkgo.Value {
	if value == nil {
		return starlarkgo.None
	}
	roles := make([]starlarkgo.Value, len(value.RoleIDs))
	for i, r := range value.RoleIDs {
		roles[i] = starlarkgo.String(r)
	}
	permissions := make([]starlarkgo.Value, len(value.Permissions))
	for i, p := range value.Permissions {
		permissions[i] = starlarkgo.String(p)
	}
	result, _ := frozenDict(map[string]starlarkgo.Value{"guild_id": starlarkgo.String(value.GuildID), "user": userValue(&value.User), "display_name": starlarkgo.String(value.DisplayName), "role_ids": starlarkgo.NewList(roles), "permissions": starlarkgo.NewList(permissions)})
	return result
}
func frozenDict(values map[string]starlarkgo.Value) (*starlarkgo.Dict, error) {
	dict := starlarkgo.NewDict(len(values))
	for key, value := range values {
		if err := dict.SetKey(starlarkgo.String(key), value); err != nil {
			return nil, err
		}
	}
	dict.Freeze()
	return dict, nil
}
