package plugin

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/disgoorg/disgo/discord"

	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	luaplugin "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

type ResponseMode int

const (
	ResponseSlash ResponseMode = iota
	ResponseComponent
	ResponseModalSubmit
)

const discordMaxMessageContentLen = 2000
const discordMaxEmbeds = 10
const (
	discordMaxEmbedTitleLen       = 256
	discordMaxEmbedDescriptionLen = 4096
	discordMaxEmbedFields         = 25
	discordMaxEmbedFieldNameLen   = 256
	discordMaxEmbedFieldValueLen  = 1024
)

const (
	discordMaxComponentRows    = 5
	discordMaxComponentsPerRow = 5
)

const discordMaxModalTitleLen = 45
const discordMaxModalComponents = 5

const discordMaxTextInputLabelLen = 45
const discordMaxTextInputDescriptionLen = 100
const discordMaxTextInputPlaceholderLen = 100
const discordMaxTextInputValueLen = 4000

const discordMaxButtonLabelLen = 80
const discordMaxSelectPlaceholderLen = 150
const discordMaxSelectValues = 25
const (
	discordMaxSelectOptionLabelLen       = 100
	discordMaxSelectOptionValueLen       = 100
	discordMaxSelectOptionDescriptionLen = 100
)

type ActionKind int

const (
	ActionNone ActionKind = iota
	ActionMessage
	ActionUpdate
	ActionModal
)

type Action struct {
	Kind   ActionKind
	Create discord.MessageCreate
	Update discord.MessageUpdate
	Modal  discord.ModalCreate
}

func ParseAction(pluginID string, raw luaplugin.EncodedValue, defaultEphemeral bool, mode ResponseMode) (Action, error) {
	decoded, err := raw.Decode()
	if err != nil {
		return Action{}, fmt.Errorf("decode plugin response: %w", err)
	}

	switch v := decoded.(type) {
	case nil:
		return Action{Kind: ActionNone}, nil
	case string:
		return pluginActionFromString(pluginID, v, defaultEphemeral, mode)
	case map[string]any:
		return pluginActionFromMap(pluginID, v, defaultEphemeral, mode)
	default:
		return Action{}, fmt.Errorf("unsupported plugin response type %T", decoded)
	}
}

func ParseAutomationMessage(pluginID string, raw luaplugin.EncodedValue) (discord.MessageCreate, error) {
	decoded, err := raw.Decode()
	if err != nil {
		return discord.MessageCreate{}, fmt.Errorf("decode plugin message: %w", err)
	}

	switch v := decoded.(type) {
	case nil:
		return discord.MessageCreate{}, errors.New("missing message")
	case string:
		act, err := pluginActionFromString(pluginID, v, false, ResponseSlash)
		if err != nil {
			return discord.MessageCreate{}, err
		}
		if act.Kind != ActionMessage {
			return discord.MessageCreate{}, errors.New("unsupported message type")
		}
		if emptyMessageCreate(act.Create) {
			return discord.MessageCreate{}, errors.New("message is empty")
		}
		return act.Create, nil
	case map[string]any:
		act, err := pluginActionFromMap(pluginID, v, false, ResponseSlash)
		if err == nil {
			if act.Kind != ActionMessage {
				return discord.MessageCreate{}, errors.New("unsupported message type")
			}
			if emptyMessageCreate(act.Create) {
				return discord.MessageCreate{}, errors.New("message is empty")
			}
			return act.Create, nil
		}
		msg, err := parseMessageCreate(pluginID, v)
		if err != nil {
			return discord.MessageCreate{}, err
		}
		if emptyMessageCreate(msg) {
			return discord.MessageCreate{}, errors.New("message is empty")
		}
		return msg, nil
	default:
		return discord.MessageCreate{}, fmt.Errorf("unsupported message type %T", decoded)
	}
}

func pluginActionFromString(
	_ string,
	content string,
	defaultEphemeral bool,
	mode ResponseMode,
) (Action, error) {
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) > discordMaxMessageContentLen {
		return Action{}, errors.New("content too long")
	}

	switch mode {
	case ResponseComponent:
		return Action{
			Kind:   ActionUpdate,
			Update: discord.MessageUpdate{Content: &content, AllowedMentions: &discord.AllowedMentions{}},
		}, nil
	case ResponseModalSubmit:
		fallthrough
	case ResponseSlash:
		mc := discord.MessageCreate{
			Content:         content,
			AllowedMentions: &discord.AllowedMentions{},
		}
		if defaultEphemeral {
			mc.Flags = discord.MessageFlagEphemeral
		}
		return Action{Kind: ActionMessage, Create: mc}, nil
	default:
		return Action{}, errors.New("unknown response mode")
	}
}

func pluginActionFromMap(
	pluginID string,
	m map[string]any,
	defaultEphemeral bool,
	mode ResponseMode,
) (Action, error) {
	if _, ok := m["actions"]; ok {
		return Action{}, errors.New("actions are automation-only")
	}
	if presentRaw, ok := m["present"]; ok {
		msg, err := parsePresent(pluginID, presentRaw)
		if err != nil {
			return Action{}, err
		}
		msg.Flags = messageFlagsFromEphemeral(m, defaultEphemeral)
		if compsRaw, hasComponents := m["components"]; hasComponents {
			comps, compsErr := parseMessageComponents(pluginID, compsRaw)
			if compsErr != nil {
				return Action{}, compsErr
			}
			msg.Components = comps
		}
		return Action{Kind: ActionMessage, Create: msg}, nil
	}

	typ := strings.ToLower(strings.TrimSpace(asStringDefault(m, "type", "")))
	if typ == "" {
		typ = defaultPluginResponseType(mode)
	}

	switch typ {
	case "message":
		msg, err := parseMessageCreate(pluginID, m)
		if err != nil {
			return Action{}, err
		}
		msg.Flags = messageFlagsFromEphemeral(m, defaultEphemeral)
		return Action{Kind: ActionMessage, Create: msg}, nil
	case "update":
		if mode == ResponseSlash && !isDeferredPluginResponse(m) {
			return Action{}, errors.New("update not supported for slash commands")
		}
		// On modal submit, Update is only valid if the modal was triggered from a button; disgo/discord will reject if not.
		upd, err := parseMessageUpdate(pluginID, m)
		if err != nil {
			return Action{}, err
		}
		return Action{Kind: ActionUpdate, Update: upd}, nil
	case "modal":
		if mode == ResponseModalSubmit {
			return Action{}, errors.New("modal not supported from modal submit")
		}
		modal, err := parseModalCreate(pluginID, m)
		if err != nil {
			return Action{}, err
		}
		return Action{Kind: ActionModal, Modal: modal}, nil
	default:
		return Action{}, fmt.Errorf("unknown response type %q", typ)
	}
}

func isDeferredPluginResponse(m map[string]any) bool {
	b, ok := asBool(m, "__deferred")
	return ok && b
}

func defaultPluginResponseType(mode ResponseMode) string {
	switch mode {
	case ResponseComponent:
		return "update"
	case ResponseSlash, ResponseModalSubmit:
		return "message"
	default:
		return ""
	}
}

func messageFlagsFromEphemeral(m map[string]any, defaultEphemeral bool) discord.MessageFlags {
	b, ok := asBool(m, "ephemeral")
	if ok {
		if b {
			return discord.MessageFlagEphemeral
		}
		return 0
	}
	if defaultEphemeral {
		return discord.MessageFlagEphemeral
	}
	return 0
}

func parseMessageCreate(pluginID string, m map[string]any) (discord.MessageCreate, error) {
	msg := discord.MessageCreate{
		AllowedMentions: &discord.AllowedMentions{},
	}

	if s, ok := asString(m, "content"); ok {
		if utf8.RuneCountInString(s) > discordMaxMessageContentLen {
			return discord.MessageCreate{}, errors.New("content too long")
		}
		msg.Content = s
	}
	if embedsRaw, ok := m["embeds"]; ok {
		embeds, err := parseEmbeds(embedsRaw)
		if err != nil {
			return discord.MessageCreate{}, err
		}
		msg.Embeds = embeds
	}
	if compsRaw, ok := m["components"]; ok {
		comps, err := parseMessageComponents(pluginID, compsRaw)
		if err != nil {
			return discord.MessageCreate{}, err
		}
		msg.Components = comps
	}

	return msg, nil
}

func parseMessageUpdate(pluginID string, m map[string]any) (discord.MessageUpdate, error) {
	upd := discord.MessageUpdate{
		AllowedMentions: &discord.AllowedMentions{},
	}

	if s, ok := asString(m, "content"); ok {
		if utf8.RuneCountInString(s) > discordMaxMessageContentLen {
			return discord.MessageUpdate{}, errors.New("content too long")
		}
		upd.Content = &s
	}
	if embedsRaw, ok := m["embeds"]; ok {
		embeds, err := parseEmbeds(embedsRaw)
		if err != nil {
			return discord.MessageUpdate{}, err
		}
		upd.Embeds = &embeds
	}
	if compsRaw, ok := m["components"]; ok {
		comps, err := parseMessageComponents(pluginID, compsRaw)
		if err != nil {
			return discord.MessageUpdate{}, err
		}
		upd.Components = &comps
	}

	return upd, nil
}

func parseEmbeds(raw any) ([]discord.Embed, error) {
	list, ok := raw.([]any)
	if !ok {
		return nil, errors.New("embeds must be an array")
	}
	if len(list) > discordMaxEmbeds {
		return nil, errors.New("too many embeds")
	}

	out := make([]discord.Embed, 0, len(list))
	for _, item := range list {
		e, err := parseEmbed(item)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func parseEmbed(raw any) (discord.Embed, error) {
	m, isMap := raw.(map[string]any)
	if !isMap {
		return discord.Embed{}, errors.New("embed must be an object")
	}

	var e discord.Embed
	if s, ok := asString(m, "title"); ok {
		if utf8.RuneCountInString(s) > discordMaxEmbedTitleLen {
			return discord.Embed{}, errors.New("embed.title too long")
		}
		e.Title = s
	}
	if s, ok := asString(m, "description"); ok {
		if utf8.RuneCountInString(s) > discordMaxEmbedDescriptionLen {
			return discord.Embed{}, errors.New("embed.description too long")
		}
		e.Description = s
	}
	if s, ok := asString(m, "url"); ok {
		if !isHTTPSURL(s) {
			return discord.Embed{}, errors.New("embed.url must be https")
		}
		e.URL = s
	}
	if n, ok := asInt(m, "color"); ok {
		e.Color = n
	}
	if fieldsRaw, ok := m["fields"]; ok {
		fields, err := parseEmbedFields(fieldsRaw)
		if err != nil {
			return discord.Embed{}, err
		}
		e.Fields = fields
	}
	if s, ok := asString(m, "image_url"); ok {
		if !isHTTPSURL(s) {
			return discord.Embed{}, errors.New("embed.image_url must be https")
		}
		e.Image = &discord.EmbedResource{URL: s}
	}
	if s, ok := asString(m, "thumbnail_url"); ok {
		if !isHTTPSURL(s) {
			return discord.Embed{}, errors.New("embed.thumbnail_url must be https")
		}
		e.Thumbnail = &discord.EmbedResource{URL: s}
	}
	if footerRaw, ok := m["footer"]; ok {
		footer, err := parseEmbedFooter(footerRaw)
		if err != nil {
			return discord.Embed{}, err
		}
		e.Footer = footer
	}
	if authorRaw, ok := m["author"]; ok {
		author, err := parseEmbedAuthor(authorRaw)
		if err != nil {
			return discord.Embed{}, err
		}
		e.Author = author
	}
	return e, nil
}

func parseEmbedFooter(raw any) (*discord.EmbedFooter, error) {
	if raw == nil {
		return nil, nil
	}
	if text, ok := raw.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, nil
		}
		return &discord.EmbedFooter{Text: text}, nil
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("embed.footer must be a string or object")
	}

	text, _ := asString(m, "text")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}

	footer := &discord.EmbedFooter{Text: text}
	if iconURL, ok := asString(m, "icon_url"); ok {
		if !isHTTPSURL(iconURL) {
			return nil, errors.New("embed.footer.icon_url must be https")
		}
		footer.IconURL = iconURL
	}
	return footer, nil
}

func parseEmbedAuthor(raw any) (*discord.EmbedAuthor, error) {
	if raw == nil {
		return nil, nil
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("embed.author must be an object")
	}

	name, _ := asString(m, "name")
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}

	author := &discord.EmbedAuthor{Name: name}
	if url, ok := asString(m, "url"); ok {
		if !isHTTPSURL(url) {
			return nil, errors.New("embed.author.url must be https")
		}
		author.URL = url
	}
	if iconURL, ok := asString(m, "icon_url"); ok {
		if !isHTTPSURL(iconURL) {
			return nil, errors.New("embed.author.icon_url must be https")
		}
		author.IconURL = iconURL
	}
	return author, nil
}

func parseEmbedFields(raw any) ([]discord.EmbedField, error) {
	list, isList := raw.([]any)
	if !isList {
		return nil, errors.New("embed.fields must be an array")
	}
	if len(list) > discordMaxEmbedFields {
		return nil, errors.New("too many embed fields")
	}
	out := make([]discord.EmbedField, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("embed field must be an object")
		}
		name, _ := asString(m, "name")
		value, _ := asString(m, "value")
		if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		if utf8.RuneCountInString(name) > discordMaxEmbedFieldNameLen {
			return nil, errors.New("embed.fields.name too long")
		}
		if utf8.RuneCountInString(value) > discordMaxEmbedFieldValueLen {
			return nil, errors.New("embed.fields.value too long")
		}
		f := discord.EmbedField{Name: name, Value: value}
		if b, hasInline := asBool(m, "inline"); hasInline {
			bb := b
			f.Inline = &bb
		}
		out = append(out, f)
	}
	return out, nil
}

func parseMessageComponents(pluginID string, raw any) ([]discord.LayoutComponent, error) {
	if emptyObject, isObject := raw.(map[string]any); isObject && len(emptyObject) == 0 {
		return []discord.LayoutComponent{}, nil
	}
	rows, isRows := raw.([]any)
	if !isRows {
		return nil, errors.New("components must be an array of rows")
	}
	if len(rows) > discordMaxComponentRows {
		return nil, errors.New("too many component rows")
	}

	out := make([]discord.LayoutComponent, 0, len(rows))
	for _, rowRaw := range rows {
		row, ok := rowRaw.([]any)
		if !ok {
			return nil, errors.New("component row must be an array")
		}
		if len(row) > discordMaxComponentsPerRow {
			return nil, errors.New("too many components in row")
		}
		comps := make([]discord.InteractiveComponent, 0, len(row))
		for _, compRaw := range row {
			comp, err := parseInteractiveComponent(pluginID, compRaw)
			if err != nil {
				return nil, err
			}
			comps = append(comps, comp)
		}
		out = append(out, discord.NewActionRow(comps...))
	}
	return out, nil
}

func parseInteractiveComponent(pluginID string, raw any) (discord.InteractiveComponent, error) {
	cm, isMap := raw.(map[string]any)
	if !isMap {
		return nil, errors.New("component must be an object")
	}

	typ := strings.ToLower(strings.TrimSpace(asStringDefault(cm, "type", "")))
	switch typ {
	case "button":
		return parseButton(pluginID, cm)
	case "string_select":
		return parseStringSelect(pluginID, cm)
	case "user_select":
		return parseUserSelect(pluginID, cm)
	case "role_select":
		return parseRoleSelect(pluginID, cm)
	case "mentionable_select":
		return parseMentionableSelect(pluginID, cm)
	case "channel_select":
		return parseChannelSelect(pluginID, cm)
	default:
		return nil, fmt.Errorf("unsupported component type %q", typ)
	}
}

func parseButton(pluginID string, m map[string]any) (discord.ButtonComponent, error) {
	style := strings.ToLower(strings.TrimSpace(asStringDefault(m, "style", "primary")))
	label := strings.TrimSpace(asStringDefault(m, "label", ""))
	url := strings.TrimSpace(asStringDefault(m, "url", ""))

	if utf8.RuneCountInString(label) > discordMaxButtonLabelLen {
		return discord.ButtonComponent{}, errors.New("button.label too long")
	}

	var btn discord.ButtonComponent
	switch style {
	case "primary":
		customID, err := buildPluginCustomID(pluginID, m)
		if err != nil {
			return discord.ButtonComponent{}, err
		}
		btn = discord.NewPrimaryButton(label, customID)
	case "secondary":
		customID, err := buildPluginCustomID(pluginID, m)
		if err != nil {
			return discord.ButtonComponent{}, err
		}
		btn = discord.NewSecondaryButton(label, customID)
	case "success":
		customID, err := buildPluginCustomID(pluginID, m)
		if err != nil {
			return discord.ButtonComponent{}, err
		}
		btn = discord.NewSuccessButton(label, customID)
	case "danger":
		customID, err := buildPluginCustomID(pluginID, m)
		if err != nil {
			return discord.ButtonComponent{}, err
		}
		btn = discord.NewDangerButton(label, customID)
	case "link":
		if url == "" {
			return discord.ButtonComponent{}, errors.New("link button requires url")
		}
		if !isHTTPSURL(url) {
			return discord.ButtonComponent{}, errors.New("link button url must be https")
		}
		btn = discord.NewLinkButton(label, url)
	default:
		return discord.ButtonComponent{}, fmt.Errorf("unknown button style %q", style)
	}

	if b, ok := asBool(m, "disabled"); ok {
		btn.Disabled = b
	}
	return btn, nil
}

func parseStringSelect(pluginID string, m map[string]any) (discord.StringSelectMenuComponent, error) {
	customID, err := buildPluginCustomID(pluginID, m)
	if err != nil {
		return discord.StringSelectMenuComponent{}, err
	}
	placeholder := asStringDefault(m, "placeholder", "")
	if utf8.RuneCountInString(placeholder) > discordMaxSelectPlaceholderLen {
		return discord.StringSelectMenuComponent{}, errors.New("string_select.placeholder too long")
	}

	opts, err := parseStringSelectOptions(m["options"])
	if err != nil {
		return discord.StringSelectMenuComponent{}, err
	}

	menu := discord.NewStringSelectMenu(customID, placeholder, opts...)
	applyStringSelectMenuCommon(&menu, m)
	return menu, nil
}

func parseStringSelectOptions(raw any) ([]discord.StringSelectMenuOption, error) {
	if raw == nil {
		return nil, errors.New("string_select requires options")
	}
	optsAny, isArray := raw.([]any)
	if !isArray {
		return nil, errors.New("string_select.options must be an array")
	}
	if len(optsAny) == 0 || len(optsAny) > discordMaxSelectValues {
		return nil, errors.New("string_select.options must have 1..25 items")
	}

	opts := make([]discord.StringSelectMenuOption, 0, len(optsAny))
	for _, o := range optsAny {
		om, isMap := o.(map[string]any)
		if !isMap {
			return nil, errors.New("string_select option must be an object")
		}
		label, _ := asString(om, "label")
		value, _ := asString(om, "value")
		if strings.TrimSpace(label) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		if utf8.RuneCountInString(label) > discordMaxSelectOptionLabelLen {
			return nil, errors.New("string_select.options.label too long")
		}
		if utf8.RuneCountInString(value) > discordMaxSelectOptionValueLen {
			return nil, errors.New("string_select.options.value too long")
		}
		opt := discord.NewStringSelectMenuOption(label, value)
		if d, ok := asString(om, "description"); ok {
			if utf8.RuneCountInString(d) > discordMaxSelectOptionDescriptionLen {
				return nil, errors.New("string_select.options.description too long")
			}
			opt.Description = d
		}
		opts = append(opts, opt)
	}
	return opts, nil
}

func applyStringSelectMenuCommon(menu *discord.StringSelectMenuComponent, m map[string]any) {
	if menu == nil {
		return
	}
	if n, ok := asInt(m, "max_values"); ok {
		menu.MaxValues = clampSelectValues(n)
	}
	if n, ok := asInt(m, "min_values"); ok {
		n = clampSelectValues(n)
		menu.MinValues = &n
	}
	if b, ok := asBool(m, "disabled"); ok {
		menu.Disabled = b
	}
}

func clampSelectValues(n int) int {
	if n < 0 {
		return 0
	}
	if n > discordMaxSelectValues {
		return discordMaxSelectValues
	}
	return n
}

func parseUserSelect(pluginID string, m map[string]any) (discord.UserSelectMenuComponent, error) {
	customID, err := buildPluginCustomID(pluginID, m)
	if err != nil {
		return discord.UserSelectMenuComponent{}, err
	}
	placeholder := asStringDefault(m, "placeholder", "")
	if utf8.RuneCountInString(placeholder) > discordMaxSelectPlaceholderLen {
		return discord.UserSelectMenuComponent{}, errors.New("user_select.placeholder too long")
	}

	menu := discord.NewUserSelectMenu(customID, placeholder)
	applySelectMenuCommon(&menu.MinValues, &menu.MaxValues, &menu.Disabled, m)
	return menu, nil
}

func parseRoleSelect(pluginID string, m map[string]any) (discord.RoleSelectMenuComponent, error) {
	customID, err := buildPluginCustomID(pluginID, m)
	if err != nil {
		return discord.RoleSelectMenuComponent{}, err
	}
	placeholder := asStringDefault(m, "placeholder", "")
	if utf8.RuneCountInString(placeholder) > discordMaxSelectPlaceholderLen {
		return discord.RoleSelectMenuComponent{}, errors.New("role_select.placeholder too long")
	}

	menu := discord.NewRoleSelectMenu(customID, placeholder)
	applySelectMenuCommon(&menu.MinValues, &menu.MaxValues, &menu.Disabled, m)
	return menu, nil
}

func parseMentionableSelect(pluginID string, m map[string]any) (discord.MentionableSelectMenuComponent, error) {
	customID, err := buildPluginCustomID(pluginID, m)
	if err != nil {
		return discord.MentionableSelectMenuComponent{}, err
	}
	placeholder := asStringDefault(m, "placeholder", "")
	if utf8.RuneCountInString(placeholder) > discordMaxSelectPlaceholderLen {
		return discord.MentionableSelectMenuComponent{}, errors.New("mentionable_select.placeholder too long")
	}

	menu := discord.NewMentionableSelectMenu(customID, placeholder)
	applySelectMenuCommon(&menu.MinValues, &menu.MaxValues, &menu.Disabled, m)
	return menu, nil
}

func parseChannelSelect(pluginID string, m map[string]any) (discord.ChannelSelectMenuComponent, error) {
	customID, err := buildPluginCustomID(pluginID, m)
	if err != nil {
		return discord.ChannelSelectMenuComponent{}, err
	}
	placeholder := asStringDefault(m, "placeholder", "")
	if utf8.RuneCountInString(placeholder) > discordMaxSelectPlaceholderLen {
		return discord.ChannelSelectMenuComponent{}, errors.New("channel_select.placeholder too long")
	}

	menu := discord.NewChannelSelectMenu(customID, placeholder)
	applySelectMenuCommon(&menu.MinValues, &menu.MaxValues, &menu.Disabled, m)

	if raw, hasTypes := m["channel_types"]; hasTypes {
		arr, isArray := raw.([]any)
		if !isArray {
			return discord.ChannelSelectMenuComponent{}, errors.New("channel_select.channel_types must be an array")
		}
		if len(arr) > discordMaxSelectValues {
			return discord.ChannelSelectMenuComponent{}, errors.New("channel_select.channel_types too long")
		}

		var types []discord.ChannelType
		for _, v := range arr {
			n, ok := anyToInt(v)
			if !ok {
				return discord.ChannelSelectMenuComponent{}, errors.New("channel_select.channel_types must be numbers")
			}
			if !isAllowedChannelType(n) {
				return discord.ChannelSelectMenuComponent{}, fmt.Errorf(
					"channel_select.channel_types contains invalid value %d",
					n,
				)
			}
			types = append(types, discord.ChannelType(n))
		}
		menu.ChannelTypes = types
	}

	return menu, nil
}

func applySelectMenuCommon(minValues **int, maxValues *int, disabled *bool, m map[string]any) {
	if n, ok := asInt(m, "max_values"); ok {
		*maxValues = clampSelectValues(n)
	}
	if n, ok := asInt(m, "min_values"); ok {
		clamped := clampSelectValues(n)
		*minValues = &clamped
	}
	if b, ok := asBool(m, "disabled"); ok {
		*disabled = b
	}
}

func anyToInt(v any) (int, bool) {
	switch vv := v.(type) {
	case float64:
		if math.IsNaN(vv) || math.IsInf(vv, 0) {
			return 0, false
		}
		return int(vv), true
	case int:
		return vv, true
	default:
		return 0, false
	}
}

func isAllowedChannelType(v int) bool {
	switch discord.ChannelType(v) {
	case discord.ChannelTypeGuildText,
		discord.ChannelTypeDM,
		discord.ChannelTypeGuildVoice,
		discord.ChannelTypeGroupDM,
		discord.ChannelTypeGuildCategory,
		discord.ChannelTypeGuildNews,
		discord.ChannelTypeGuildNewsThread,
		discord.ChannelTypeGuildPublicThread,
		discord.ChannelTypeGuildPrivateThread,
		discord.ChannelTypeGuildStageVoice,
		discord.ChannelTypeGuildDirectory,
		discord.ChannelTypeGuildForum,
		discord.ChannelTypeGuildMedia:
		return true
	default:
		return false
	}
}

func parseModalCreate(pluginID string, m map[string]any) (discord.ModalCreate, error) {
	title, hasTitle := asString(m, "title")
	if !hasTitle {
		return discord.ModalCreate{}, errors.New("modal requires title")
	}
	if utf8.RuneCountInString(title) > discordMaxModalTitleLen {
		return discord.ModalCreate{}, errors.New("modal.title too long")
	}

	localID, hasID := asString(m, "id")
	if !hasID || strings.TrimSpace(localID) == "" {
		return discord.ModalCreate{}, errors.New("modal requires id")
	}
	customID, err := pluginhost.BuildCustomID(pluginID, localID)
	if err != nil {
		return discord.ModalCreate{}, err
	}

	componentsRaw, hasComponents := m["components"]
	if !hasComponents {
		return discord.ModalCreate{}, errors.New("modal requires components")
	}

	fields, isArray := componentsRaw.([]any)
	if !isArray {
		return discord.ModalCreate{}, errors.New("modal.components must be an array")
	}
	if len(fields) == 0 || len(fields) > discordMaxModalComponents {
		return discord.ModalCreate{}, errors.New("modal.components must have 1..5 items")
	}

	var comps []discord.LayoutComponent
	for _, fieldRaw := range fields {
		fm, isMap := fieldRaw.(map[string]any)
		if !isMap {
			return discord.ModalCreate{}, errors.New("modal component must be an object")
		}
		c, fieldErr := parseModalField(pluginID, fm)
		if fieldErr != nil {
			return discord.ModalCreate{}, fieldErr
		}
		comps = append(comps, c)
	}

	return discord.NewModalCreate(customID, title, comps...), nil
}

func parseModalField(pluginID string, m map[string]any) (discord.LabelComponent, error) {
	label, hasLabel := asString(m, "label")
	if !hasLabel {
		return discord.LabelComponent{}, errors.New("modal field requires label")
	}
	if utf8.RuneCountInString(label) > discordMaxTextInputLabelLen {
		return discord.LabelComponent{}, errors.New("modal field label too long")
	}
	localID, hasID := asString(m, "id")
	if !hasID || strings.TrimSpace(localID) == "" {
		return discord.LabelComponent{}, errors.New("modal field requires id")
	}
	customID, err := pluginhost.BuildCustomID(pluginID, localID)
	if err != nil {
		return discord.LabelComponent{}, err
	}

	style := strings.ToLower(strings.TrimSpace(asStringDefault(m, "style", "short")))
	input, err := newTextInputComponent(customID, style)
	if err != nil {
		return discord.LabelComponent{}, err
	}
	if propsErr := applyTextInputProps(&input, m); propsErr != nil {
		return discord.LabelComponent{}, propsErr
	}

	lc := discord.NewLabel(label, input)
	if s, ok := asString(m, "description"); ok {
		if utf8.RuneCountInString(s) > discordMaxTextInputDescriptionLen {
			return discord.LabelComponent{}, errors.New("modal field description too long")
		}
		lc.Description = s
	}
	return lc, nil
}

func newTextInputComponent(customID, style string) (discord.TextInputComponent, error) {
	switch style {
	case "short":
		return discord.NewShortTextInput(customID), nil
	case "paragraph":
		return discord.NewParagraphTextInput(customID), nil
	default:
		return discord.TextInputComponent{}, fmt.Errorf("unknown text input style %q", style)
	}
}

func applyTextInputProps(input *discord.TextInputComponent, m map[string]any) error {
	if input == nil {
		return errors.New("text input is nil")
	}

	if b, ok := asBool(m, "required"); ok {
		input.Required = b
	}
	if s, ok := asString(m, "placeholder"); ok {
		if utf8.RuneCountInString(s) > discordMaxTextInputPlaceholderLen {
			return errors.New("modal field placeholder too long")
		}
		input.Placeholder = s
	}
	if s, ok := asString(m, "value"); ok {
		if utf8.RuneCountInString(s) > discordMaxTextInputValueLen {
			return errors.New("modal field value too long")
		}
		input.Value = s
	}
	if n, ok := asInt(m, "max_length"); ok {
		if n < 0 {
			n = 0
		}
		input.MaxLength = n
	}
	if n, ok := asInt(m, "min_length"); ok {
		if n < 0 {
			n = 0
		}
		input.MinLength = &n
	}
	return nil
}

func parsePresent(_ string, raw any) (discord.MessageCreate, error) {
	m, isMap := raw.(map[string]any)
	if !isMap {
		return discord.MessageCreate{}, errors.New("present must be an object")
	}

	title := strings.TrimSpace(asStringDefault(m, "title", ""))
	body := strings.TrimSpace(asStringDefault(m, "body", ""))
	kind := strings.ToLower(strings.TrimSpace(asStringDefault(m, "kind", "info")))
	if utf8.RuneCountInString(title) > discordMaxEmbedTitleLen {
		return discord.MessageCreate{}, errors.New("present.title too long")
	}
	if utf8.RuneCountInString(body) > discordMaxEmbedDescriptionLen {
		return discord.MessageCreate{}, errors.New("present.body too long")
	}

	switch kind {
	case "success", "ok":
		kind = string(interactions.KindSuccess)
	case "warning", "warn":
		kind = string(interactions.KindWarning)
	case "error", "err":
		kind = string(interactions.KindError)
	case "info":
		kind = string(interactions.KindInfo)
	default:
		kind = string(interactions.KindInfo)
	}

	e := interactions.NoticeEmbed(interactions.Kind(kind), title, body)

	if fieldsRaw, ok := m["fields"]; ok {
		fields, err := parseEmbedFields(fieldsRaw)
		if err != nil {
			return discord.MessageCreate{}, err
		}
		e.Fields = fields
	}

	return discord.MessageCreate{
		Embeds:          []discord.Embed{e},
		AllowedMentions: &discord.AllowedMentions{},
	}, nil
}

func buildPluginCustomID(pluginID string, component map[string]any) (string, error) {
	localID, ok := asString(component, "id")
	if !ok || strings.TrimSpace(localID) == "" {
		return "", errors.New("component requires id")
	}
	return pluginhost.BuildCustomID(pluginID, localID)
}

func asString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func asStringDefault(m map[string]any, key, def string) string {
	if s, ok := asString(m, key); ok {
		return s
	}
	return def
}

func asBool(m map[string]any, key string) (bool, bool) {
	v, ok := m[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func asInt(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}

	switch vv := v.(type) {
	case float64:
		if math.IsNaN(vv) || math.IsInf(vv, 0) {
			return 0, false
		}
		return int(vv), true
	case int:
		return vv, true
	default:
		return 0, false
	}
}

func asInt64(m map[string]any, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}

	switch vv := v.(type) {
	case float64:
		if math.IsNaN(vv) || math.IsInf(vv, 0) {
			return 0, false
		}
		return int64(vv), true
	case int:
		return int64(vv), true
	case int64:
		return vv, true
	default:
		return 0, false
	}
}

func emptyMessageCreate(msg discord.MessageCreate) bool {
	return strings.TrimSpace(msg.Content) == "" && len(msg.Embeds) == 0 && len(msg.Components) == 0
}

func isHTTPSURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	return strings.HasPrefix(strings.ToLower(raw), "https://")
}
