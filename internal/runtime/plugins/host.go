package pluginhost

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/omit"

	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	luaplugin "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Host struct {
	mu sync.RWMutex

	logger *slog.Logger
	dirs   []string

	prodMode             bool
	allowUnsignedPlugins bool
	trustedKeysFile      string
	permissionsFile      string

	store   Store
	discord Discord
	policy  permissions.Policy
	i18n    *i18n.Registry

	plugins  map[string]*Plugin
	commands map[string]PluginCommand

	eventSubs map[string][]string
	jobs      []PluginJob
}

type Store interface {
	TrustedSigners() store.TrustedSignerStore
	PluginKV() store.PluginKVStore
	UserSettings() store.UserSettingsStore
	Reminders() store.ReminderStore
	CheckIns() store.CheckInStore
	Warnings() store.WarningStore
	Audit() store.AuditStore
}

type Discord interface {
	SelfUser(ctx context.Context) (luaplugin.UserResult, error)
	GetUser(ctx context.Context, userID uint64) (luaplugin.UserResult, error)
	GetMember(ctx context.Context, guildID, userID uint64) (luaplugin.MemberResult, error)
	GetGuild(ctx context.Context, guildID uint64) (luaplugin.GuildResult, error)
	GetRole(ctx context.Context, guildID, roleID uint64) (luaplugin.RoleResult, error)
	GetChannel(ctx context.Context, channelID uint64) (luaplugin.ChannelResult, error)
	CreateChannel(ctx context.Context, spec luaplugin.ChannelCreateSpec) (luaplugin.ChannelResult, error)
	EditChannel(ctx context.Context, spec luaplugin.ChannelEditSpec) (luaplugin.ChannelResult, error)
	DeleteChannel(ctx context.Context, channelID uint64) error
	SetChannelOverwrite(ctx context.Context, spec luaplugin.PermissionOverwriteSpec) error
	DeleteChannelOverwrite(ctx context.Context, channelID, overwriteID uint64) error
	GetMessage(ctx context.Context, spec luaplugin.MessageGetSpec) (luaplugin.MessageInfo, error)
	SendDM(ctx context.Context, pluginID string, userID uint64, message any) (luaplugin.MessageResult, error)
	SendChannel(ctx context.Context, pluginID string, channelID uint64, message any) (luaplugin.MessageResult, error)
	TimeoutMember(ctx context.Context, guildID, userID uint64, until time.Time) error
	SetSlowmode(ctx context.Context, channelID uint64, seconds int) error
	SetNickname(ctx context.Context, guildID, userID uint64, nickname *string) error
	CreateRole(ctx context.Context, spec luaplugin.RoleCreateSpec) (luaplugin.RoleResult, error)
	EditRole(ctx context.Context, spec luaplugin.RoleEditSpec) (luaplugin.RoleResult, error)
	DeleteRole(ctx context.Context, guildID, roleID uint64) error
	AddRole(ctx context.Context, spec luaplugin.RoleMemberSpec) error
	RemoveRole(ctx context.Context, spec luaplugin.RoleMemberSpec) error
	ListMessages(ctx context.Context, spec luaplugin.MessageListSpec) ([]luaplugin.MessageInfo, error)
	DeleteMessage(ctx context.Context, spec luaplugin.MessageDeleteSpec) error
	BulkDeleteMessages(ctx context.Context, channelID uint64, messageIDs []uint64) (int, error)
	PurgeMessages(ctx context.Context, spec luaplugin.PurgeSpec) (int, error)
	CrosspostMessage(ctx context.Context, spec luaplugin.MessageGetSpec) (luaplugin.MessageInfo, error)
	PinMessage(ctx context.Context, spec luaplugin.MessageGetSpec) error
	UnpinMessage(ctx context.Context, spec luaplugin.MessageGetSpec) error
	GetReactions(ctx context.Context, spec luaplugin.ReactionListSpec) ([]luaplugin.UserResult, error)
	AddReaction(ctx context.Context, spec luaplugin.ReactionSpec) error
	RemoveOwnReaction(ctx context.Context, spec luaplugin.ReactionSpec) error
	RemoveUserReaction(ctx context.Context, spec luaplugin.ReactionUserSpec) error
	ClearReactions(ctx context.Context, spec luaplugin.MessageGetSpec) error
	ClearReactionsForEmoji(ctx context.Context, spec luaplugin.ReactionSpec) error
	CreateThreadFromMessage(ctx context.Context, spec luaplugin.ThreadCreateFromMessageSpec) (luaplugin.ThreadResult, error)
	CreateThreadInChannel(ctx context.Context, spec luaplugin.ThreadCreateSpec) (luaplugin.ThreadResult, error)
	JoinThread(ctx context.Context, threadID uint64) error
	LeaveThread(ctx context.Context, threadID uint64) error
	AddThreadMember(ctx context.Context, threadID, userID uint64) error
	RemoveThreadMember(ctx context.Context, threadID, userID uint64) error
	UpdateThread(ctx context.Context, spec luaplugin.ThreadUpdateSpec) (luaplugin.ThreadResult, error)
	CreateInvite(ctx context.Context, spec luaplugin.InviteCreateSpec) (luaplugin.InviteResult, error)
	GetInvite(ctx context.Context, code string) (luaplugin.InviteResult, error)
	DeleteInvite(ctx context.Context, code string) error
	ListChannelInvites(ctx context.Context, channelID uint64) ([]luaplugin.InviteResult, error)
	ListGuildInvites(ctx context.Context, guildID uint64) ([]luaplugin.InviteResult, error)
	CreateWebhook(ctx context.Context, spec luaplugin.WebhookCreateSpec) (luaplugin.WebhookResult, error)
	GetWebhook(ctx context.Context, webhookID uint64) (luaplugin.WebhookResult, error)
	ListChannelWebhooks(ctx context.Context, channelID uint64) ([]luaplugin.WebhookResult, error)
	EditWebhook(ctx context.Context, spec luaplugin.WebhookEditSpec) (luaplugin.WebhookResult, error)
	DeleteWebhook(ctx context.Context, webhookID uint64) error
	ExecuteWebhook(ctx context.Context, pluginID string, spec luaplugin.WebhookExecuteSpec) (luaplugin.MessageResult, error)
	CreateEmoji(ctx context.Context, spec luaplugin.EmojiCreateSpec) (luaplugin.EmojiResult, error)
	EditEmoji(ctx context.Context, spec luaplugin.EmojiEditSpec) (luaplugin.EmojiResult, error)
	DeleteEmoji(ctx context.Context, spec luaplugin.EmojiDeleteSpec) error
	CreateSticker(ctx context.Context, spec luaplugin.StickerCreateSpec) (luaplugin.StickerResult, error)
	EditSticker(ctx context.Context, spec luaplugin.StickerEditSpec) (luaplugin.StickerResult, error)
	DeleteSticker(ctx context.Context, spec luaplugin.StickerDeleteSpec) error
}

type Options struct {
	Dirs                []string
	ProdMode            bool
	AllowUnsignedPlugin bool
	TrustedKeysFile     string
	PermissionsFile     string
	Store               Store
	Discord             Discord
	Logger              *slog.Logger
	I18n                *i18n.Registry
}

type Plugin struct {
	ID  string
	Dir string

	Manifest  Manifest
	Signature *Signature
	Effective permissions.Permissions
	Commands  []Command
	Events    []string
	Jobs      []Job

	VM *luaplugin.VM
}

type PluginCommand struct {
	PluginID string
	Command  Command
}

type PluginJob struct {
	PluginID string
	JobID    string
	Schedule string
}

type Payload struct {
	GuildID     string
	ChannelID   string
	UserID      string
	Locale      string
	IsOwner     bool
	Options     map[string]any
	Interaction luaplugin.Interaction
}

func NewHost(opts Options) (*Host, error) {
	dirs := make([]string, 0, len(opts.Dirs))
	for _, dir := range opts.Dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		dirs = append(dirs, dir)
	}
	if len(dirs) == 0 {
		return nil, errors.New("at least one plugins dir is required")
	}
	if opts.Logger == nil {
		return nil, errors.New("logger is required")
	}

	policy, err := permissions.LoadPolicyFile(opts.PermissionsFile)
	if err != nil {
		return nil, err
	}

	return &Host{
		logger:               opts.Logger.With(slog.String("component", "plugins")),
		dirs:                 dirs,
		prodMode:             opts.ProdMode,
		allowUnsignedPlugins: opts.AllowUnsignedPlugin,
		trustedKeysFile:      opts.TrustedKeysFile,
		permissionsFile:      opts.PermissionsFile,
		store:                opts.Store,
		discord:              opts.Discord,
		policy:               policy,
		i18n:                 opts.I18n,
		plugins:              map[string]*Plugin{},
		commands:             map[string]PluginCommand{},
		eventSubs:            map[string][]string{},
	}, nil
}

func (m *Host) LoadAll(ctx context.Context) error {
	pluginDirs, err := m.readPluginDirEntries()
	if err != nil || pluginDirs == nil {
		return err
	}

	policy, err := permissions.LoadPolicyFile(m.permissionsFile)
	if err != nil {
		return err
	}

	m.resetPluginLocales()

	keys, err := LoadTrustedKeys(ctx, m.trustedKeysFile, m.store)
	if err != nil {
		return err
	}

	nextPlugins, nextCommands := m.loadPluginsFromEntries(ctx, pluginDirs, keys, policy)
	nextEvents, nextJobs := buildSubscriptions(nextPlugins)
	oldPlugins := m.swapState(nextPlugins, nextCommands, nextEvents, nextJobs, policy)
	closePlugins(oldPlugins)
	return nil
}

func (m *Host) readPluginDirEntries() ([]string, error) {
	pluginDirs := []string{}
	for _, root := range m.dirs {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read plugins dir %q: %w", root, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pluginDirs = append(pluginDirs, filepath.Join(root, entry.Name()))
		}
	}
	return pluginDirs, nil
}

func (m *Host) resetPluginLocales() {
	if m.i18n != nil {
		m.i18n.ResetPluginLocales()
	}
}

func (m *Host) loadPluginsFromEntries(
	ctx context.Context,
	pluginDirs []string,
	keys map[string]ed25519.PublicKey,
	policy permissions.Policy,
) (map[string]*Plugin, map[string]PluginCommand) {
	nextPlugins := map[string]*Plugin{}
	nextCommands := map[string]PluginCommand{}

	for _, pluginDir := range pluginDirs {
		pluginDir = strings.TrimSpace(pluginDir)
		if pluginDir == "" {
			continue
		}
		pl, cmds, err := m.loadOne(ctx, pluginDir, keys, policy)
		if err != nil {
			m.logger.WarnContext(
				ctx,
				"failed to load plugin",
				slog.String("dir", pluginDir),
				slog.String("err", err.Error()),
			)
			continue
		}
		if pl == nil {
			continue
		}

		if _, exists := nextPlugins[pl.ID]; exists {
			m.logger.WarnContext(ctx, "duplicate plugin id, skipping", slog.String("plugin", pl.ID))
			if pl.VM != nil {
				pl.VM.Close()
			}
			continue
		}

		nextPlugins[pl.ID] = pl
		m.loadPluginLocales(ctx, pl.ID, pluginDir)
		addCommands(ctx, m.logger, nextCommands, pl.ID, cmds)
	}

	return nextPlugins, nextCommands
}

func (m *Host) loadPluginLocales(ctx context.Context, pluginID string, pluginDir string) {
	if m.i18n == nil {
		return
	}

	localesDir := filepath.Join(pluginDir, "locales")
	fi, statErr := os.Stat(localesDir)
	if statErr != nil || !fi.IsDir() {
		return
	}

	if entries, err := os.ReadDir(localesDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			locale := strings.TrimSpace(entry.Name())
			if locale == "" || i18n.IsSupportedDiscordLocale(locale) {
				continue
			}

			path := filepath.Join(localesDir, locale, "messages.json")
			if _, msgFileErr := os.Stat(path); msgFileErr != nil {
				continue
			}

			m.logger.WarnContext(
				ctx,
				"unknown plugin locale, ignoring",
				slog.String("plugin", pluginID),
				slog.String("locale", locale),
				slog.String("path", path),
			)
		}
	}

	if err := m.i18n.LoadPluginLocales(pluginID, localesDir); err != nil {
		m.logger.WarnContext(
			ctx,
			"failed to load plugin locales",
			slog.String("plugin", pluginID),
			slog.String("err", err.Error()),
		)
	}
}

func addCommands(
	ctx context.Context,
	logger *slog.Logger,
	nextCommands map[string]PluginCommand,
	pluginID string,
	cmds []PluginCommand,
) {
	for _, cmd := range cmds {
		if cmd.Command.Name == "" {
			continue
		}
		key := commandLookupKey(cmd.Command.Type, cmd.Command.Name)
		if _, exists := nextCommands[key]; exists {
			logger.WarnContext(
				ctx,
				"duplicate command name, skipping",
				slog.String("command", cmd.Command.Name),
				slog.String("type", NormalizeCommandType(cmd.Command.Type)),
				slog.String("plugin", pluginID),
			)
			continue
		}
		nextCommands[key] = cmd
	}
}

func commandLookupKey(kind, name string) string {
	return NormalizeCommandType(kind) + ":" + strings.ToLower(strings.TrimSpace(name))
}

func (m *Host) swapState(
	nextPlugins map[string]*Plugin,
	nextCommands map[string]PluginCommand,
	nextEvents map[string][]string,
	nextJobs []PluginJob,
	policy permissions.Policy,
) map[string]*Plugin {
	m.mu.Lock()
	oldPlugins := m.plugins
	m.plugins = nextPlugins
	m.commands = nextCommands
	m.eventSubs = nextEvents
	m.jobs = nextJobs
	m.policy = policy
	m.mu.Unlock()
	return oldPlugins
}

func closePlugins(oldPlugins map[string]*Plugin) {
	for _, pl := range oldPlugins {
		if pl != nil && pl.VM != nil {
			pl.VM.Close()
		}
	}
}

func buildSubscriptions(pls map[string]*Plugin) (map[string][]string, []PluginJob) {
	ev := map[string][]string{}
	var jobs []PluginJob

	for _, pl := range pls {
		if pl == nil {
			continue
		}

		for _, raw := range pl.Events {
			name := strings.ToLower(strings.TrimSpace(raw))
			if name == "" {
				continue
			}
			ev[name] = append(ev[name], pl.ID)
		}

		for _, job := range pl.Jobs {
			id := strings.TrimSpace(job.ID)
			spec := strings.TrimSpace(job.Schedule)
			if id == "" || spec == "" {
				continue
			}
			jobs = append(jobs, PluginJob{
				PluginID: pl.ID,
				JobID:    id,
				Schedule: spec,
			})
		}
	}

	for name := range ev {
		sort.Strings(ev[name])
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].PluginID != jobs[j].PluginID {
			return jobs[i].PluginID < jobs[j].PluginID
		}
		return jobs[i].JobID < jobs[j].JobID
	})

	return ev, jobs
}

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

func (m *Host) Commands() map[string]PluginCommand {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]PluginCommand, len(m.commands))
	maps.Copy(out, m.commands)
	return out
}

func (m *Host) Jobs() []PluginJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return append([]PluginJob(nil), m.jobs...)
}

func (m *Host) EventSubscribers(eventName string) []string {
	eventName = strings.ToLower(strings.TrimSpace(eventName))
	if eventName == "" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return append([]string(nil), m.eventSubs[eventName]...)
}

func (m *Host) EffectivePermissions(pluginID string) (permissions.Permissions, bool) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return permissions.Permissions{}, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	pl, ok := m.plugins[pluginID]
	if !ok || pl == nil {
		return permissions.Permissions{}, false
	}
	return pl.Effective, true
}

func (m *Host) CommandCreates() []discord.ApplicationCommandCreate {
	return m.CommandCreatesWithLocalizations(nil, nil)
}

type CommandLocalizer func(pluginID, locale, messageID string) (string, bool)

func (m *Host) CommandCreatesWithLocalizations(
	locales []string,
	localize CommandLocalizer,
) []discord.ApplicationCommandCreate {
	return m.CommandCreatesFiltered(nil, locales, localize)
}

func (m *Host) CommandCreatesFiltered(
	allowedPluginIDs map[string]struct{},
	locales []string,
	localize CommandLocalizer,
) []discord.ApplicationCommandCreate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.commands) == 0 {
		return nil
	}

	names := make([]string, 0, len(m.commands))
	for name := range m.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]discord.ApplicationCommandCreate, 0, len(names))
	for _, key := range names {
		cmd := m.commands[key]
		if len(allowedPluginIDs) != 0 {
			if _, ok := allowedPluginIDs[cmd.PluginID]; !ok {
				continue
			}
		}
		out = append(out, commandToCreate(cmd.PluginID, cmd.Command, locales, localize))
	}
	return out
}

func (m *Host) HandleSlash(ctx context.Context, cmdName string, payload Payload) (any, bool, string, error) {
	return m.handleCommand(ctx, CommandTypeSlash, cmdName, payload)
}

func (m *Host) HandleUserCommand(ctx context.Context, cmdName string, payload Payload) (any, bool, string, error) {
	return m.handleCommand(ctx, CommandTypeUser, cmdName, payload)
}

func (m *Host) HandleMessageCommand(ctx context.Context, cmdName string, payload Payload) (any, bool, string, error) {
	return m.handleCommand(ctx, CommandTypeMessage, cmdName, payload)
}

func (m *Host) handleCommand(ctx context.Context, kind, cmdName string, payload Payload) (any, bool, string, error) {
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
		res      any
		hasValue bool
		err      error
	)
	if pl.VM.HasDefinition() {
		res, hasValue, err = pl.VM.CallRoute(ctx, routeKindForCommandType(kind), cmdName, luaplugin.Payload{
			GuildID:     payload.GuildID,
			ChannelID:   payload.ChannelID,
			UserID:      payload.UserID,
			Locale:      payload.Locale,
			IsOwner:     payload.IsOwner,
			Options:     payload.Options,
			Interaction: payload.Interaction,
		})
	} else {
		res, hasValue, err = pl.VM.CallHandle(ctx, "Handle", cmdName, luaplugin.Payload{
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
) (any, string, error) {
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

	res, _, err := pl.VM.CallRoute(ctx, luaplugin.RouteAutocomplete, routeID, luaplugin.Payload{
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

func (m *Host) HandleComponent(ctx context.Context, pluginID, localID string, payload Payload) (any, bool, error) {
	m.mu.RLock()
	pl, ok := m.plugins[pluginID]
	if !ok || pl.VM == nil {
		m.mu.RUnlock()
		return nil, false, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	vm := pl.VM
	m.mu.RUnlock()

	if vm.HasDefinition() {
		return vm.CallRoute(ctx, luaplugin.RouteComponent, localID, luaplugin.Payload{
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

	return vm.CallHandle(ctx, "HandleComponent", localID, luaplugin.Payload{
		GuildID:   payload.GuildID,
		ChannelID: payload.ChannelID,
		UserID:    payload.UserID,
		Locale:    payload.Locale,
		IsOwner:   payload.IsOwner,
		Options:   payload.Options,
	})
}

func (m *Host) HandleModal(ctx context.Context, pluginID, localID string, payload Payload) (any, bool, error) {
	m.mu.RLock()
	pl, ok := m.plugins[pluginID]
	if !ok || pl.VM == nil {
		m.mu.RUnlock()
		return nil, false, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	vm := pl.VM
	m.mu.RUnlock()

	if vm.HasDefinition() {
		return vm.CallRoute(ctx, luaplugin.RouteModal, localID, luaplugin.Payload{
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

	return vm.CallHandle(ctx, "HandleModal", localID, luaplugin.Payload{
		GuildID:   payload.GuildID,
		ChannelID: payload.ChannelID,
		UserID:    payload.UserID,
		Locale:    payload.Locale,
		IsOwner:   payload.IsOwner,
		Options:   payload.Options,
	})
}

func (m *Host) HandleEvent(ctx context.Context, pluginID, eventName string, payload Payload) (any, bool, error) {
	m.mu.RLock()
	pl, ok := m.plugins[pluginID]
	if !ok || pl.VM == nil {
		m.mu.RUnlock()
		return nil, false, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	vm := pl.VM
	m.mu.RUnlock()

	if vm.HasDefinition() {
		return vm.CallRoute(ctx, luaplugin.RouteEvent, eventName, luaplugin.Payload{
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

	return vm.CallHandle(ctx, "HandleEvent", eventName, luaplugin.Payload{
		GuildID:   payload.GuildID,
		ChannelID: payload.ChannelID,
		UserID:    payload.UserID,
		Locale:    payload.Locale,
		IsOwner:   payload.IsOwner,
		Options:   payload.Options,
	})
}

func (m *Host) HandleJob(ctx context.Context, pluginID, jobID string, payload Payload) (any, bool, error) {
	m.mu.RLock()
	pl, ok := m.plugins[pluginID]
	if !ok || pl.VM == nil {
		m.mu.RUnlock()
		return nil, false, fmt.Errorf("plugin %q not loaded", pluginID)
	}
	vm := pl.VM
	m.mu.RUnlock()

	if vm.HasDefinition() {
		return vm.CallRoute(ctx, luaplugin.RouteJob, jobID, luaplugin.Payload{
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

	return vm.CallHandle(ctx, "HandleJob", jobID, luaplugin.Payload{
		GuildID:   payload.GuildID,
		ChannelID: payload.ChannelID,
		UserID:    payload.UserID,
		Locale:    payload.Locale,
		IsOwner:   payload.IsOwner,
		Options:   payload.Options,
	})
}

type PluginInfo struct {
	ID        string
	Name      string
	Version   string
	Dir       string
	Signed    bool
	Bundled   bool
	Effective permissions.Permissions
	Commands  []Command
}

func (m *Host) Infos() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]PluginInfo, 0, len(m.plugins))
	for _, pl := range m.plugins {
		if pl == nil {
			continue
		}
		out = append(out, PluginInfo{
			ID:        pl.ID,
			Name:      pl.Manifest.Name,
			Version:   pl.Manifest.Version,
			Dir:       pl.Dir,
			Signed:    pl.Signature != nil,
			Bundled:   m.isBundledDir(pl.Dir),
			Effective: pl.Effective,
			Commands:  append([]Command(nil), pl.Commands...),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *Host) isBundledDir(dir string) bool {
	if m == nil {
		return false
	}
	dir = strings.TrimSpace(dir)
	if dir == "" || len(m.dirs) == 0 {
		return false
	}
	return dir == strings.TrimSpace(m.dirs[0])
}

func (m *Host) loadOne(
	_ context.Context,
	pluginDir string,
	keys map[string]ed25519.PublicKey,
	policy permissions.Policy,
) (*Plugin, []PluginCommand, error) {
	manifestPath := filepath.Join(pluginDir, "plugin.json")
	manifest, err := ReadManifest(manifestPath)
	if err != nil {
		return nil, nil, err
	}
	if permErr := manifest.Permissions.Validate(); permErr != nil {
		return nil, nil, fmt.Errorf("permissions: %w", permErr)
	}

	signaturePath := filepath.Join(pluginDir, "signature.json")
	var sig *Signature
	if s, sigErr := ReadSignature(signaturePath); sigErr == nil {
		sig = &s
	} else if !os.IsNotExist(sigErr) {
		return nil, nil, sigErr
	}

	if m.prodMode && !m.allowUnsignedPlugins {
		if sig == nil {
			return nil, nil, errors.New("missing signature.json")
		}

		if verifyErr := VerifyDirSignature(pluginDir, *sig, keys); verifyErr != nil {
			return nil, nil, verifyErr
		}
	}

	script := filepath.Join(pluginDir, "plugin.lua")
	granted := policy.Granted(manifest.ID)
	effective := permissions.Effective(manifest.Permissions, granted)

	vm, err := luaplugin.NewFromFile(script, luaplugin.Options{
		Logger:      m.logger,
		PluginID:    manifest.ID,
		PluginDir:   pluginDir,
		Permissions: effective,
		Discord:     m.discord,
		I18n:        m.i18n,
		Store:       m.store,
	})
	if err != nil {
		return nil, nil, err
	}

	descriptor, hasDescriptor := vm.Definition()

	commands := append([]Command(nil), manifest.Commands...)
	events := append([]string(nil), manifest.Events...)
	jobs := append([]Job(nil), manifest.Jobs...)
	if hasDescriptor {
		commands = commandsFromDefinition(descriptor)
		events = append([]string(nil), descriptor.Events...)
		jobs = jobsFromDefinition(descriptor)
	}

	pl := &Plugin{
		ID:        manifest.ID,
		Dir:       pluginDir,
		Manifest:  manifest,
		Signature: sig,
		Effective: effective,
		Commands:  commands,
		Events:    events,
		Jobs:      jobs,
		VM:        vm,
	}

	var cmds []PluginCommand
	for _, cmd := range pl.Commands {
		if cmd.Name == "" {
			continue
		}
		cmds = append(cmds, PluginCommand{
			PluginID: pl.ID,
			Command:  cmd,
		})
	}

	return pl, cmds, nil
}

func commandToCreate(
	pluginID string,
	cmd Command,
	locales []string,
	localize CommandLocalizer,
) discord.ApplicationCommandCreate {
	name := cmd.Name
	if NormalizeCommandType(cmd.Type) != CommandTypeSlash {
		name = strings.TrimSpace(cmd.Name)
	}
	var options []discord.ApplicationCommandOption
	if NormalizeCommandType(cmd.Type) == CommandTypeSlash && (len(cmd.Subcommands) > 0 || len(cmd.Groups) > 0) {
		options = append(options, buildSubcommands(pluginID, cmd.Subcommands, locales, localize)...)
		options = append(options, buildGroups(pluginID, cmd.Groups, locales, localize)...)
	} else if NormalizeCommandType(cmd.Type) == CommandTypeSlash {
		options = append(options, buildOptions(pluginID, cmd.Options, locales, localize)...)
	}

	perms, hasPerms := commandPermissions(cmd.DefaultMemberPermissions)
	switch NormalizeCommandType(cmd.Type) {
	case CommandTypeUser:
		create := discord.UserCommandCreate{Name: name}
		if hasPerms {
			create.DefaultMemberPermissions = omit.NewPtr(perms)
		}
		return create
	case CommandTypeMessage:
		create := discord.MessageCommandCreate{Name: name}
		if hasPerms {
			create.DefaultMemberPermissions = omit.NewPtr(perms)
		}
		return create
	default:
		create := discord.SlashCommandCreate{
			Name:        name,
			Description: cmd.Description,
			Options:     options,
		}
		if hasPerms {
			create.DefaultMemberPermissions = omit.NewPtr(perms)
		}
		if locs := descriptionLocalizations(pluginID, cmd.DescriptionID, locales, localize); len(locs) != 0 {
			create.DescriptionLocalizations = locs
		}
		return create
	}
}

func commandPermissions(names []string) (discord.Permissions, bool) {
	if len(names) == 0 {
		return 0, false
	}

	var (
		perms discord.Permissions
		ok    bool
	)
	for _, name := range names {
		perm, found := commandPermissionByName(name)
		if !found {
			continue
		}
		perms |= perm
		ok = true
	}
	return perms, ok
}

func commandPermissionByName(name string) (discord.Permissions, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "administrator":
		return discord.PermissionAdministrator, true
	case "manage_guild":
		return discord.PermissionManageGuild, true
	case "manage_roles":
		return discord.PermissionManageRoles, true
	case "manage_expressions":
		return discord.PermissionManageGuildExpressions, true
	case "create_expressions":
		return discord.PermissionCreateGuildExpressions, true
	case "manage_emojis_and_stickers":
		return discord.PermissionManageGuildExpressions, true
	case "manage_messages":
		return discord.PermissionManageMessages, true
	case "manage_nicknames":
		return discord.PermissionManageNicknames, true
	case "manage_channels":
		return discord.PermissionManageChannels, true
	case "kick_members":
		return discord.PermissionKickMembers, true
	case "ban_members":
		return discord.PermissionBanMembers, true
	case "moderate_members":
		return discord.PermissionModerateMembers, true
	default:
		return 0, false
	}
}

func descriptionLocalizations(
	pluginID string,
	descriptionID string,
	locales []string,
	localize CommandLocalizer,
) map[discord.Locale]string {
	descID := strings.TrimSpace(descriptionID)
	if descID == "" || len(locales) == 0 || localize == nil {
		return nil
	}

	locs := map[discord.Locale]string{}
	for _, locale := range locales {
		s, ok := localize(pluginID, locale, descID)
		if !ok {
			continue
		}
		locs[discord.Locale(locale)] = s
	}
	return locs
}

func buildSubcommands(
	pluginID string,
	cmds []Subcommand,
	locales []string,
	localize CommandLocalizer,
) []discord.ApplicationCommandOption {
	out := make([]discord.ApplicationCommandOption, 0, len(cmds))
	for _, sc := range cmds {
		opt := discord.ApplicationCommandOptionSubCommand{
			Name:        sc.Name,
			Description: sc.Description,
			Options:     buildOptions(pluginID, sc.Options, locales, localize),
		}
		if locs := descriptionLocalizations(pluginID, sc.DescriptionID, locales, localize); len(locs) != 0 {
			opt.DescriptionLocalizations = locs
		}
		out = append(out, opt)
	}
	return out
}

func buildGroups(
	pluginID string,
	groups []CommandGroup,
	locales []string,
	localize CommandLocalizer,
) []discord.ApplicationCommandOption {
	out := make([]discord.ApplicationCommandOption, 0, len(groups))
	for _, g := range groups {
		opt := discord.ApplicationCommandOptionSubCommandGroup{
			Name:        g.Name,
			Description: g.Description,
		}
		if locs := descriptionLocalizations(pluginID, g.DescriptionID, locales, localize); len(locs) != 0 {
			opt.DescriptionLocalizations = locs
		}

		subs := make([]discord.ApplicationCommandOptionSubCommand, 0, len(g.Subcommands))
		for _, sc := range g.Subcommands {
			sub := discord.ApplicationCommandOptionSubCommand{
				Name:        sc.Name,
				Description: sc.Description,
				Options:     buildOptions(pluginID, sc.Options, locales, localize),
			}
			if locs := descriptionLocalizations(pluginID, sc.DescriptionID, locales, localize); len(locs) != 0 {
				sub.DescriptionLocalizations = locs
			}
			subs = append(subs, sub)
		}
		opt.Options = subs

		out = append(out, opt)
	}
	return out
}

func buildOptions(
	pluginID string,
	opts []CommandOption,
	locales []string,
	localize CommandLocalizer,
) []discord.ApplicationCommandOption {
	// Discord requires required options to be listed before non-required options.
	// Plugin authors will naturally write "nice" human ordering; we normalize so
	// that one plugin cannot brick command registration.
	opts = normalizeRequiredOptionsFirst(opts)

	out := make([]discord.ApplicationCommandOption, 0, len(opts))
	for _, opt := range opts {
		if o, ok := buildOption(pluginID, opt, locales, localize); ok {
			out = append(out, o)
		}
	}
	return out
}

func normalizeRequiredOptionsFirst(opts []CommandOption) []CommandOption {
	if len(opts) < 2 {
		return opts
	}

	// Fast-path: already valid ordering.
	seenOptional := false
	needsFix := false
	for _, opt := range opts {
		if !opt.Required {
			seenOptional = true
			continue
		}
		if seenOptional {
			needsFix = true
			break
		}
	}
	if !needsFix {
		return opts
	}

	required := make([]CommandOption, 0, len(opts))
	optional := make([]CommandOption, 0, len(opts))
	for _, opt := range opts {
		if opt.Required {
			required = append(required, opt)
		} else {
			optional = append(optional, opt)
		}
	}
	return append(required, optional...)
}

func buildOption(
	pluginID string,
	opt CommandOption,
	locales []string,
	localize CommandLocalizer,
) (discord.ApplicationCommandOption, bool) {
	typ := strings.ToLower(strings.TrimSpace(opt.Type))
	descLocs := descriptionLocalizations(pluginID, opt.DescriptionID, locales, localize)
	switch typ {
	case "string":
		return buildStringOption(opt, descLocs), true
	case "bool":
		return buildBoolOption(opt, descLocs), true
	case "int":
		return buildIntOption(opt, descLocs), true
	case "float":
		return buildFloatOption(opt, descLocs), true
	case "user":
		return buildUserOption(opt, descLocs), true
	case "channel":
		return buildChannelOption(opt, descLocs), true
	case "role":
		return buildRoleOption(opt, descLocs), true
	case "mentionable":
		return buildMentionableOption(opt, descLocs), true
	case "attachment":
		return buildAttachmentOption(opt, descLocs), true
	default:
		return nil, false
	}
}

func buildStringOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionString {
	choices := buildStringChoices(opt.Choices)
	if strings.TrimSpace(opt.Autocomplete) != "" {
		choices = nil
	}
	o := discord.ApplicationCommandOptionString{
		Name:         opt.Name,
		Description:  opt.Description,
		Required:     opt.Required,
		MinLength:    opt.MinLength,
		MaxLength:    opt.MaxLength,
		Choices:      choices,
		Autocomplete: strings.TrimSpace(opt.Autocomplete) != "",
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildBoolOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionBool {
	o := discord.ApplicationCommandOptionBool{
		Name:        opt.Name,
		Description: opt.Description,
		Required:    opt.Required,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildIntOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionInt {
	choices := buildIntChoices(opt.Choices)
	if strings.TrimSpace(opt.Autocomplete) != "" {
		choices = nil
	}
	o := discord.ApplicationCommandOptionInt{
		Name:         opt.Name,
		Description:  opt.Description,
		Required:     opt.Required,
		Choices:      choices,
		Autocomplete: strings.TrimSpace(opt.Autocomplete) != "",
		MinValue:     floatToIntPtr(opt.MinValue),
		MaxValue:     floatToIntPtr(opt.MaxValue),
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildFloatOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionFloat {
	choices := buildFloatChoices(opt.Choices)
	if strings.TrimSpace(opt.Autocomplete) != "" {
		choices = nil
	}
	o := discord.ApplicationCommandOptionFloat{
		Name:         opt.Name,
		Description:  opt.Description,
		Required:     opt.Required,
		Choices:      choices,
		Autocomplete: strings.TrimSpace(opt.Autocomplete) != "",
		MinValue:     opt.MinValue,
		MaxValue:     opt.MaxValue,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildUserOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionUser {
	o := discord.ApplicationCommandOptionUser{
		Name:        opt.Name,
		Description: opt.Description,
		Required:    opt.Required,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildChannelOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionChannel {
	o := discord.ApplicationCommandOptionChannel{
		Name:         opt.Name,
		Description:  opt.Description,
		Required:     opt.Required,
		ChannelTypes: buildChannelTypes(opt.ChannelTypes),
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildRoleOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionRole {
	o := discord.ApplicationCommandOptionRole{
		Name:        opt.Name,
		Description: opt.Description,
		Required:    opt.Required,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildMentionableOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionMentionable {
	o := discord.ApplicationCommandOptionMentionable{
		Name:        opt.Name,
		Description: opt.Description,
		Required:    opt.Required,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildAttachmentOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionAttachment {
	o := discord.ApplicationCommandOptionAttachment{
		Name:        opt.Name,
		Description: opt.Description,
		Required:    opt.Required,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildStringChoices(in []OptionChoice) []discord.ApplicationCommandOptionChoiceString {
	out := make([]discord.ApplicationCommandOptionChoiceString, 0, len(in))
	for _, c := range in {
		v, ok := c.Value.(string)
		if !ok {
			continue
		}
		out = append(out, discord.ApplicationCommandOptionChoiceString{Name: c.Name, Value: v})
	}
	return out
}

func buildIntChoices(in []OptionChoice) []discord.ApplicationCommandOptionChoiceInt {
	out := make([]discord.ApplicationCommandOptionChoiceInt, 0, len(in))
	for _, c := range in {
		v, ok := floatToInt(c.Value)
		if !ok {
			continue
		}
		out = append(out, discord.ApplicationCommandOptionChoiceInt{Name: c.Name, Value: v})
	}
	return out
}

func buildFloatChoices(in []OptionChoice) []discord.ApplicationCommandOptionChoiceFloat {
	out := make([]discord.ApplicationCommandOptionChoiceFloat, 0, len(in))
	for _, c := range in {
		switch v := c.Value.(type) {
		case float64:
			out = append(out, discord.ApplicationCommandOptionChoiceFloat{Name: c.Name, Value: v})
		case int:
			out = append(out, discord.ApplicationCommandOptionChoiceFloat{Name: c.Name, Value: float64(v)})
		}
	}
	return out
}

func floatToIntPtr(v *float64) *int {
	if v == nil {
		return nil
	}
	if i, ok := floatToInt(*v); ok {
		return &i
	}
	return nil
}

func floatToInt(v any) (int, bool) {
	switch vv := v.(type) {
	case float64:
		if vv != float64(int(vv)) {
			return 0, false
		}
		return int(vv), true
	case int:
		return vv, true
	default:
		return 0, false
	}
}

func buildChannelTypes(in []int) []discord.ChannelType {
	if len(in) == 0 {
		return nil
	}
	out := make([]discord.ChannelType, 0, len(in))
	for _, v := range in {
		out = append(out, discord.ChannelType(v))
	}
	return out
}
