package commandapi

import (
	"context"
	"log/slog"
	"maps"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	"github.com/xsyetopz/go-mamacord/internal/persona"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Store interface {
	Restrictions() store.RestrictionStore
	Warnings() store.WarningStore
	Audit() store.AuditStore
	TrustedSigners() store.TrustedSignerStore
	MarketplaceSources() store.MarketplaceSourceStore
	MarketplaceSourceSyncs() store.MarketplaceSourceSyncStore
	PluginInstalls() store.PluginInstallStore
	TrustedVendors() store.TrustedVendorStore
	TrustedVendorKeys() store.TrustedVendorKeyStore
	AdminSessions() store.AdminSessionStore
	PluginKV() store.PluginKVStore
	ModuleStates() store.ModuleStateStore
	Users() store.UserStore
	Guilds() store.GuildStore
	GuildMembers() store.GuildMemberStore
	UserSettings() store.UserSettingsStore
	Reminders() store.ReminderStore
	CheckIns() store.CheckInStore
}

type Translator struct {
	Registry i18n.Registry
	Locale   discord.Locale
	PluginID string
	UserID   uint64
}

func (t Translator) S(messageID string, data map[string]any) string {
	if t.UserID != 0 {
		data = withPersonaTemplateData(data, t.Locale, t.UserID, messageID)
	}
	return t.Registry.MustLocalize(i18n.Config{
		Locale:       t.Locale.Code(),
		PluginID:     strings.TrimSpace(t.PluginID),
		MessageID:    messageID,
		TemplateData: data,
	})
}

func withPersonaTemplateData(
	data map[string]any,
	locale discord.Locale,
	userID uint64,
	messageID string,
) map[string]any {
	if data == nil {
		data = map[string]any{}
	} else {
		clone := make(map[string]any, len(data)+1)
		maps.Copy(clone, data)
		data = clone
	}

	if _, ok := data["Pet"]; !ok {
		data["Pet"] = personaPet(locale, userID, messageID)
	}
	if _, ok := data["Mommy"]; !ok {
		data["Mommy"] = personaMommy(locale)
	}
	return data
}

func personaPet(locale discord.Locale, userID uint64, messageID string) string {
	return persona.PetName(locale, userID, messageID)
}

func personaMommy(locale discord.Locale) string {
	return persona.Mommy(locale)
}

func LocalizeMap(locales []string, fn func(locale string) string) map[discord.Locale]string {
	const baseLocale = "en-US"
	base := strings.TrimSpace(fn(baseLocale))

	out := map[discord.Locale]string{}
	for _, locale := range locales {
		locale = strings.TrimSpace(locale)
		if locale == "" || strings.EqualFold(locale, baseLocale) {
			continue
		}

		translated := fn(locale)
		translated = strings.TrimSpace(translated)
		if translated == "" {
			continue
		}
		if base != "" && translated == base {
			continue
		}
		out[discord.Locale(locale)] = translated
	}
	return out
}

type PluginAdmin interface {
	Configured() bool
	Infos() []pluginhost.PluginInfo
	Reload(ctx context.Context) error
}

type MarketplaceAdmin interface {
	Configured() bool
	ListSources(ctx context.Context) ([]marketplace.Source, error)
	UpsertSource(ctx context.Context, req marketplace.SourceUpsert) (marketplace.Source, error)
	DeleteSource(ctx context.Context, sourceID string) error
	SyncSource(ctx context.Context, sourceID string) (marketplace.SyncResult, error)
	Search(ctx context.Context, query marketplace.SearchQuery) ([]marketplace.PluginCandidate, error)
	Install(ctx context.Context, req marketplace.InstallRequest) (marketplace.InstallResult, error)
	Update(ctx context.Context, req marketplace.UpdateRequest) (marketplace.UpdateResult, error)
	Uninstall(ctx context.Context, req marketplace.UninstallRequest) error
	TrustSigner(ctx context.Context, req marketplace.TrustSignerRequest) error
	TrustVendor(ctx context.Context, req marketplace.TrustVendorRequest) (marketplace.TrustVendorResult, error)
}

type ModuleKind string

const (
	ModuleKindCoreBuiltin ModuleKind = "core_builtin"
	ModuleKindPlugin      ModuleKind = "plugin"
)

type ModuleRuntime string

const (
	ModuleRuntimeGo  ModuleRuntime = "go"
	ModuleRuntimeLua ModuleRuntime = "lua"
)

type ModuleInfo struct {
	ID             string
	Name           string
	Kind           ModuleKind
	Runtime        ModuleRuntime
	Enabled        bool
	DefaultEnabled bool
	Toggleable     bool
	Signed         bool
	Source         string
	Commands       []string
}

type ModuleAdmin interface {
	Configured() bool
	Infos() []ModuleInfo
	Reload(ctx context.Context) error
	SetEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error
	Reset(ctx context.Context, moduleID string) error
}

type Services struct {
	Logger   *slog.Logger
	Store    Store
	ProdMode bool

	IsOwner func(userID uint64) bool

	Plugins     PluginAdmin
	Marketplace MarketplaceAdmin
	Modules     ModuleAdmin

	// HelpNames returns the localized slash command names for help output.
	HelpNames func(locale discord.Locale) []string
}

type SlashCommand struct {
	Name          string
	NameID        string
	DescID        string
	CreateCommand func(locales []string, t Translator) discord.ApplicationCommandCreate
	Handle        func(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t Translator, s Services) (interactions.SlashAction, error)
}
