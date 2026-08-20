package pluginhost

import (
	"context"
	"encoding/json"
	pluginmanifest "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/manifest"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/projection"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/disgoorg/disgo/discord"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	accountstore "github.com/xsyetopz/go-mamacord/internal/storage/accounts"
	automationstore "github.com/xsyetopz/go-mamacord/internal/storage/automation"
	marketstore "github.com/xsyetopz/go-mamacord/internal/storage/marketplace"
	moderationstore "github.com/xsyetopz/go-mamacord/internal/storage/moderation"
	pluginstore "github.com/xsyetopz/go-mamacord/internal/storage/plugins"
)

func TestCommandPermissions_ExpressionsAliases(t *testing.T) {
	t.Parallel()

	got, ok := commandPermissions([]string{"manage_expressions", "create_expressions"})
	if !ok {
		t.Fatalf("expected expression permissions to map")
	}

	want := discord.PermissionManageGuildExpressions | discord.PermissionCreateGuildExpressions
	if got != want {
		t.Fatalf("unexpected permissions: got %v want %v", got, want)
	}
}

func TestCommandToCreate_ByType(t *testing.T) {
	t.Parallel()

	slash := commandToCreate("plugin", projection.Command{
		Type:        projection.CommandTypeSlash,
		Name:        "lookup",
		Description: "Lookup",
		Options: []projection.CommandOption{{
			OptionPresentation: projection.OptionPresentation{
				Name:         "query",
				Type:         "string",
				Description:  "Query",
				Autocomplete: "lookup_query",
				Choices: []projection.OptionChoice{
					{Name: "stale", Value: "stale"},
				},
			},
		}},
	}, nil, nil)

	slashCreate, ok := slash.(discord.SlashCommandCreate)
	if !ok {
		t.Fatalf("expected slash command create, got %T", slash)
	}
	if len(slashCreate.Options) != 1 {
		t.Fatalf("expected slash option to be present")
	}
	stringOpt, ok := slashCreate.Options[0].(discord.ApplicationCommandOptionString)
	if !ok {
		t.Fatalf("expected string option, got %T", slashCreate.Options[0])
	}
	if !stringOpt.Autocomplete {
		t.Fatalf("expected autocomplete to be enabled")
	}
	if len(stringOpt.Choices) != 0 {
		t.Fatalf("expected explicit choices to be cleared when autocomplete is enabled")
	}

	user := commandToCreate("plugin", projection.Command{
		Type: projection.CommandTypeUser,
		Name: "Inspect User",
	}, nil, nil)
	if _, ok := user.(discord.UserCommandCreate); !ok {
		t.Fatalf("expected user command create, got %T", user)
	}

	message := commandToCreate("plugin", projection.Command{
		Type: projection.CommandTypeMessage,
		Name: "Inspect Message",
	}, nil, nil)
	if _, ok := message.(discord.MessageCommandCreate); !ok {
		t.Fatalf("expected message command create, got %T", message)
	}
}

func TestCommandToCreate_NormalizesRequiredOptionsFirst(t *testing.T) {
	t.Parallel()

	createAny := commandToCreate("plugin", projection.Command{
		Type:        projection.CommandTypeSlash,
		Name:        "stickers",
		Description: "Manage stickers",
		Subcommands: []projection.Subcommand{{
			Name:        "create",
			Description: "Create",
			Options: []projection.CommandOption{
				// Bad order on purpose: optional first, required later.
				{OptionPresentation: projection.OptionPresentation{Name: "description", Type: "string", Description: "Sticker description", Required: false}},
				{OptionPresentation: projection.OptionPresentation{Name: "name", Type: "string", Description: "Sticker name", Required: true}},
				{OptionPresentation: projection.OptionPresentation{Name: "emoji_tag", Type: "string", Description: "Emoji tag", Required: true}},
			},
		}},
	}, nil, nil)

	create, ok := createAny.(discord.SlashCommandCreate)
	if !ok {
		t.Fatalf("expected slash create, got %T", createAny)
	}
	if len(create.Options) != 1 {
		t.Fatalf("expected 1 top-level option, got %d", len(create.Options))
	}

	sub, ok := create.Options[0].(discord.ApplicationCommandOptionSubCommand)
	if !ok {
		t.Fatalf("expected subcommand option, got %T", create.Options[0])
	}
	if len(sub.Options) != 3 {
		t.Fatalf("expected 3 sub options, got %d", len(sub.Options))
	}

	first, ok := sub.Options[0].(discord.ApplicationCommandOptionString)
	if !ok {
		t.Fatalf("expected first option to be string, got %T", sub.Options[0])
	}
	if !first.Required || first.Name != "name" {
		t.Fatalf("expected first option to be required name, got name=%q required=%v", first.Name, first.Required)
	}

	second, ok := sub.Options[1].(discord.ApplicationCommandOptionString)
	if !ok {
		t.Fatalf("expected second option to be string, got %T", sub.Options[1])
	}
	if !second.Required || second.Name != "emoji_tag" {
		t.Fatalf("expected second option to be required emoji_tag, got name=%q required=%v", second.Name, second.Required)
	}

	third, ok := sub.Options[2].(discord.ApplicationCommandOptionString)
	if !ok {
		t.Fatalf("expected third option to be string, got %T", sub.Options[2])
	}
	if third.Required || third.Name != "description" {
		t.Fatalf("expected third option to be optional description, got name=%q required=%v", third.Name, third.Required)
	}
}

func TestLoadOneReadsBundleDirButExportsEntryDir(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	entryDir := filepath.Join(tmp, "entry")
	bundleDir := filepath.Join(tmp, "bundles", "rev-1")
	if err := os.MkdirAll(entryDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(entry): %v", err)
	}
	writeTestPluginBundle(t, bundleDir, "separate")

	host, err := NewHost(Options{
		BundleOptions: BundleOptions{
			Dirs: []string{tmp},
		},
		RuntimeOptions: RuntimeOptions{
			Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		},
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}

	pl, _, err := host.loadOne(context.Background(), pluginLoadLocation{
		EntryDir:  entryDir,
		BundleDir: bundleDir,
		Bundled:   false,
	}, nil, permissions.Policy{})
	if err != nil {
		t.Fatalf("loadOne: %v", err)
	}
	if pl.Dir != entryDir {
		t.Fatalf("expected exported plugin dir to stay on entry dir, got %q want %q", pl.Dir, entryDir)
	}
	if pl.BundleDir != bundleDir {
		t.Fatalf("expected bundle dir to be tracked separately, got %q want %q", pl.BundleDir, bundleDir)
	}
}

func TestLoadAllUsesBundleStateToLoadBundleDirButExportsEntryDir(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "user")
	entryDir := filepath.Join(root, "managed")
	bundleDir := filepath.Join(entryDir, "bundles", "git-abc123")
	writeTestPluginBundle(t, bundleDir, "managed")
	if err := bundles.NewLocalRepository().WriteState(entryDir, bundles.State{
		ActiveRelativeDir: filepath.Join("bundles", "git-abc123"),
		Revision:          "abc123",
	}); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	host, err := NewHost(Options{
		BundleOptions: BundleOptions{
			Dirs: []string{root},
		},
		RuntimeOptions: RuntimeOptions{
			Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		},
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	if err := host.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	info, ok := host.plugins["managed"]
	if !ok {
		t.Fatal("expected managed plugin to load through bundle state")
	}
	if info.Dir != entryDir {
		t.Fatalf("expected plugin entry dir to remain the root entry, got %q want %q", info.Dir, entryDir)
	}
	if info.BundleDir != bundleDir {
		t.Fatalf("expected plugin bundle dir to resolve from bundle state, got %q want %q", info.BundleDir, bundleDir)
	}
}

func TestLoadAllPrefersStoredBundleRelativeDirForManagedUserPlugin(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "user")
	entryDir := filepath.Join(root, "managed")
	oldBundleDir := filepath.Join(entryDir, "bundles", "git-old")
	newBundleDir := filepath.Join(entryDir, "bundles", "git-new")
	writeTestPluginBundleVersion(t, oldBundleDir, "managed", "0.1.0")
	writeTestPluginBundleVersion(t, newBundleDir, "managed", "0.2.0")
	if err := bundles.NewLocalRepository().WriteState(entryDir, bundles.State{
		ActiveRelativeDir: filepath.Join("bundles", "git-old"),
		Revision:          "old",
	}); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	host, err := NewHost(Options{
		BundleOptions: BundleOptions{
			Dirs: []string{root},
		},
		RuntimeOptions: RuntimeOptions{
			Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
			Store: hostStoreStub{
				installs: map[string]marketstore.PluginInstall{
					"managed": {
						PluginID: "managed",
						PluginInstallSource: marketstore.PluginInstallSource{
							InstallKind: "git",
							GitURL:      "https://example.invalid/demo.git",
							GitRevision: "new",
							SourcePath:  "managed",
						},
						PluginInstallArtifact: marketstore.PluginInstallArtifact{
							BundleRelativeDir: filepath.Join("bundles", "git-new"),
							InstalledHashB64:  "hash",
						},
						InstalledAt: time.Unix(1_700_000_000, 0).UTC(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	if err := host.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	info, ok := host.plugins["managed"]
	if !ok {
		t.Fatal("expected managed plugin to load through stored bundle registry")
	}
	if info.BundleDir != newBundleDir {
		t.Fatalf("expected stored bundle dir to win over pointer file, got %q want %q", info.BundleDir, newBundleDir)
	}
	if info.Manifest.Version != "0.2.0" {
		t.Fatalf("expected manifest version from stored bundle dir, got %q", info.Manifest.Version)
	}
}

func TestLoadAllFallsBackWhenStoredBundleRelativeDirIsInvalid(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "user")
	entryDir := filepath.Join(root, "managed")
	bundleDir := filepath.Join(entryDir, "bundles", "git-old")
	writeTestPluginBundleVersion(t, bundleDir, "managed", "0.1.0")
	if err := bundles.NewLocalRepository().WriteState(entryDir, bundles.State{
		ActiveRelativeDir: filepath.Join("bundles", "git-old"),
		Revision:          "old",
	}); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	host, err := NewHost(Options{
		BundleOptions: BundleOptions{
			Dirs: []string{root},
		},
		RuntimeOptions: RuntimeOptions{
			Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
			Store: hostStoreStub{
				installs: map[string]marketstore.PluginInstall{
					"managed": {
						PluginID: "managed",
						PluginInstallSource: marketstore.PluginInstallSource{
							InstallKind: "git",
							GitURL:      "https://example.invalid/demo.git",
							GitRevision: "bad",
							SourcePath:  "managed",
						},
						PluginInstallArtifact: marketstore.PluginInstallArtifact{
							BundleRelativeDir: filepath.Join("..", "escape"),
							InstalledHashB64:  "hash",
						},
						InstalledAt: time.Unix(1_700_000_000, 0).UTC(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	if err := host.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	info, ok := host.plugins["managed"]
	if !ok {
		t.Fatal("expected managed plugin to load through fallback bundle state")
	}
	if info.BundleDir != bundleDir {
		t.Fatalf("expected invalid stored bundle path to fall back to bundle state, got %q want %q", info.BundleDir, bundleDir)
	}
}

func TestLoadAllRejectsFlatPluginRootsWithoutBundleState(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	root := filepath.Join(tmp, "user")
	entryDir := filepath.Join(root, "flat")
	writeTestPluginBundle(t, entryDir, "flat")

	host, err := NewHost(Options{
		BundleOptions: BundleOptions{
			Dirs: []string{root},
		},
		RuntimeOptions: RuntimeOptions{
			Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		},
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	if err := host.LoadAll(context.Background()); err == nil {
		t.Fatal("expected missing bundle-state rejection")
	}
	if len(host.plugins) != 0 {
		t.Fatalf("unexpected loaded plugins: %d", len(host.plugins))
	}
}

func TestInfosMarksBundledPluginsFromBundledRoot(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	bundledRoot := filepath.Join(tmp, "bundled")
	userRoot := filepath.Join(tmp, "user")
	writeTestPluginRoot(t, filepath.Join(bundledRoot, "bundled-one"), "bundled-one")
	writeTestPluginRoot(t, filepath.Join(userRoot, "user-one"), "user-one")

	host, err := NewHost(Options{
		BundleOptions: BundleOptions{
			Dirs: []string{bundledRoot, userRoot},
		},
		RuntimeOptions: RuntimeOptions{
			Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		},
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	if err := host.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	infos := host.Infos()
	if len(infos) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(infos))
	}

	bundledByID := map[string]bool{}
	for _, info := range infos {
		bundledByID[info.ID] = info.Bundled
	}
	if !bundledByID["bundled-one"] {
		t.Fatal("expected plugin discovered under the bundled root to be marked bundled")
	}
	if bundledByID["user-one"] {
		t.Fatal("expected plugin discovered under the user root to not be marked bundled")
	}
}

func TestLoadAllUsesCachedRepositoryActiveDir(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	repo, err := bundles.NewCachedRepository(bundles.CachedRepositoryOptions{
		StoreDir: filepath.Join(tmp, "bundle-store"),
		CacheDir: filepath.Join(tmp, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewCachedRepository: %v", err)
	}

	srcDir := filepath.Join(tmp, "src")
	writeTestPluginBundle(t, srcDir, "managed")

	rootDir := filepath.Join(tmp, "plugins", "managed")
	bundle, err := repo.MaterializeBundle(srcDir, rootDir, "git-abc123")
	if err != nil {
		t.Fatalf("MaterializeBundle: %v", err)
	}

	host, err := NewHost(Options{
		BundleOptions: BundleOptions{
			Dirs:       []string{filepath.Join(tmp, "plugins")},
			Repository: repo,
		},
		RuntimeOptions: RuntimeOptions{
			Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		},
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	if err := host.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	pl, ok := host.plugins["managed"]
	if !ok {
		t.Fatal("expected managed plugin to load through cached repository")
	}
	if pl.Dir != rootDir {
		t.Fatalf("expected entry dir to stay at plugin root, got %q want %q", pl.Dir, rootDir)
	}
	if pl.BundleDir != bundle.ActiveDir {
		t.Fatalf("expected host bundle dir to resolve to active cache dir, got %q want %q", pl.BundleDir, bundle.ActiveDir)
	}
	if strings.Contains(pl.BundleDir, string(filepath.Separator)+"bundle-store"+string(filepath.Separator)) {
		t.Fatalf("expected host bundle dir to use active cache, not store artifact dir: %q", pl.BundleDir)
	}
}

func TestLoadAllUsesObjectStoreRepositoryActiveDir(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	storeBackend, err := bundles.NewDirObjectStore(filepath.Join(tmp, "object-store"))
	if err != nil {
		t.Fatalf("NewDirObjectStore: %v", err)
	}
	repo, err := bundles.NewObjectStoreRepository(bundles.ObjectStoreRepositoryOptions{
		Store:    storeBackend,
		CacheDir: filepath.Join(tmp, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewObjectStoreRepository: %v", err)
	}

	srcDir := filepath.Join(tmp, "src")
	writeTestPluginBundle(t, srcDir, "managed")

	rootDir := filepath.Join(tmp, "plugins", "managed")
	bundle, err := repo.MaterializeBundle(srcDir, rootDir, "git-abc123")
	if err != nil {
		t.Fatalf("MaterializeBundle: %v", err)
	}

	host, err := NewHost(Options{
		BundleOptions: BundleOptions{
			Dirs:       []string{filepath.Join(tmp, "plugins")},
			Repository: repo,
		},
		RuntimeOptions: RuntimeOptions{
			Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		},
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	if err := host.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	pl, ok := host.plugins["managed"]
	if !ok {
		t.Fatal("expected managed plugin to load through object-store repository")
	}
	if pl.Dir != rootDir {
		t.Fatalf("expected entry dir to stay at plugin root, got %q want %q", pl.Dir, rootDir)
	}
	if pl.BundleDir != bundle.ActiveDir {
		t.Fatalf("expected host bundle dir to resolve to active cache dir, got %q want %q", pl.BundleDir, bundle.ActiveDir)
	}
	if strings.Contains(pl.BundleDir, string(filepath.Separator)+"object-store"+string(filepath.Separator)) {
		t.Fatalf("expected host bundle dir to use active cache, not object-store contents: %q", pl.BundleDir)
	}
}

func TestLoadAllPrefersStoredBundleRelativeDirForManagedObjectStorePluginAndUsesActiveCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmp := t.TempDir()
	storeBackend, err := bundles.NewDirObjectStore(filepath.Join(tmp, "object-store"))
	if err != nil {
		t.Fatalf("NewDirObjectStore: %v", err)
	}
	repo, err := bundles.NewObjectStoreRepository(bundles.ObjectStoreRepositoryOptions{
		Store:    storeBackend,
		CacheDir: filepath.Join(tmp, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewObjectStoreRepository: %v", err)
	}

	rootDir := filepath.Join(tmp, "plugins", "managed")
	oldSrcDir := filepath.Join(tmp, "old-src")
	newSrcDir := filepath.Join(tmp, "new-src")
	writeTestPluginBundleVersion(t, oldSrcDir, "managed", "0.1.0")
	writeTestPluginBundleVersion(t, newSrcDir, "managed", "0.2.0")

	oldBundle, err := repo.MaterializeBundle(oldSrcDir, rootDir, "git-old")
	if err != nil {
		t.Fatalf("MaterializeBundle(old): %v", err)
	}
	newBundle, err := repo.MaterializeBundle(newSrcDir, rootDir, "git-new")
	if err != nil {
		t.Fatalf("MaterializeBundle(new): %v", err)
	}
	if err := repo.WriteState(rootDir, bundles.State{
		ActiveRelativeDir: oldBundle.BundleRelativeDir,
		Revision:          "old",
		HashB64:           oldBundle.HashB64,
	}); err != nil {
		t.Fatalf("WriteState(old): %v", err)
	}

	host, err := NewHost(Options{
		BundleOptions: BundleOptions{
			Dirs:       []string{filepath.Join(tmp, "plugins")},
			Repository: repo,
		},
		RuntimeOptions: RuntimeOptions{
			Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
			Store: hostStoreStub{
				installs: map[string]marketstore.PluginInstall{
					"managed": {
						PluginID: "managed",
						PluginInstallSource: marketstore.PluginInstallSource{
							InstallKind: "git",
							GitURL:      "https://example.invalid/demo.git",
							GitRevision: "new",
							SourcePath:  "managed",
						},
						PluginInstallArtifact: marketstore.PluginInstallArtifact{
							BundleRelativeDir: newBundle.BundleRelativeDir,
							InstalledHashB64:  newBundle.HashB64,
						},
						InstalledAt: time.Unix(1_700_000_000, 0).UTC(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	if err := host.LoadAll(ctx); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	pl, ok := host.plugins["managed"]
	if !ok {
		t.Fatal("expected managed plugin to load through stored bundle registry")
	}
	if pl.Manifest.Version != "0.2.0" {
		t.Fatalf("expected stored bundle manifest version %q, got %q", "0.2.0", pl.Manifest.Version)
	}
	if pl.BundleDir == newBundle.BundleDir {
		t.Fatalf("expected runtime bundle dir to materialize into active cache, got artifact dir %q", pl.BundleDir)
	}
	if pl.BundleDir != newBundle.ActiveDir {
		t.Fatalf("expected runtime bundle dir to use preferred active cache %q, got %q", newBundle.ActiveDir, pl.BundleDir)
	}
	if strings.Contains(pl.BundleDir, string(filepath.Separator)+"object-store"+string(filepath.Separator)) {
		t.Fatalf("expected runtime bundle dir to stay off object-store contents, got %q", pl.BundleDir)
	}
}

func writeTestPluginBundle(t *testing.T, dir, pluginID string) {
	t.Helper()

	writeTestPluginBundleVersion(t, dir, pluginID, "0.1.0")
}

func writeTestPluginRoot(t *testing.T, root, pluginID string) {
	t.Helper()

	bundleRel := filepath.Join("bundles", "test-v0.1.0")
	writeTestPluginBundleVersion(t, filepath.Join(root, bundleRel), pluginID, "0.1.0")
	if err := bundles.NewLocalRepository().WriteState(root, bundles.State{
		ActiveRelativeDir: bundleRel,
		Revision:          "0.1.0",
	}); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
}

func writeTestPluginBundleVersion(t *testing.T, dir, pluginID, version string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	manifest, err := json.Marshal(pluginmanifest.StarlarkManifest{Schema: pluginmanifest.StarlarkManifestSchema, ID: pluginID, Name: pluginID, Version: version, Entrypoint: pluginmanifest.StarlarkEntrypoint, Permissions: pluginmanifest.StarlarkManifestPermissions{Network: pluginmanifest.StarlarkManifestNetworkPermissions{Hosts: []string{}}}, Locales: pluginmanifest.StarlarkManifestLocales{Default: "en-US", Supported: []string{"en-US"}}, StateKeys: []string{}, Assets: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), manifest, 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.json): %v", err)
	}
	source := `load("@mamacord//api.star", "cog", "plugin", "reply", "slash_command")
def run(ctx): return [reply(content="ok")]
def setup(bot): bot.add_cog(cog(name="Test", commands=[slash_command(name="test", description="Test command", handler=run)]))
PLUGIN = plugin(setup=setup)
`
	if err := os.WriteFile(filepath.Join(dir, "plugin.star"), []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.star): %v", err)
	}
	localeDir := filepath.Join(dir, "locales", "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localeDir, "messages.json"), []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

type hostStoreStub struct {
	installs map[string]marketstore.PluginInstall
	kv       pluginstore.PluginKVStore
}

func (s hostStoreStub) TrustedSigners() marketstore.TrustedSignerStore {
	return trustedSignerListStub{}
}
func (s hostStoreStub) PluginInstalls() marketstore.PluginInstallStore {
	return pluginInstallLookupStub{installs: s.installs}
}
func (s hostStoreStub) PluginKV() pluginstore.PluginKVStore          { return s.kv }
func (s hostStoreStub) UserSettings() accountstore.UserSettingsStore { return nil }
func (s hostStoreStub) Reminders() automationstore.ReminderStore     { return nil }
func (s hostStoreStub) CheckIns() automationstore.CheckInStore       { return nil }
func (s hostStoreStub) Warnings() moderationstore.WarningStore       { return nil }
func (s hostStoreStub) Audit() moderationstore.AuditStore            { return nil }

type trustedSignerListStub struct{}

func (trustedSignerListStub) ListTrustedSigners(context.Context) ([]marketstore.TrustedSigner, error) {
	return nil, nil
}
func (trustedSignerListStub) PutTrustedSigner(context.Context, marketstore.TrustedSigner) error {
	return nil
}
func (trustedSignerListStub) DeleteTrustedSigner(context.Context, string) error { return nil }

type pluginInstallLookupStub struct {
	installs map[string]marketstore.PluginInstall
}

func (s pluginInstallLookupStub) GetPluginInstall(_ context.Context, pluginID string) (marketstore.PluginInstall, bool, error) {
	item, ok := s.installs[pluginID]
	return item, ok, nil
}
func (s pluginInstallLookupStub) ListPluginInstalls(context.Context) ([]marketstore.PluginInstall, error) {
	return nil, nil
}
func (s pluginInstallLookupStub) PutPluginInstall(context.Context, marketstore.PluginInstall) error {
	return nil
}
func (s pluginInstallLookupStub) DeletePluginInstall(context.Context, string) error { return nil }

func TestLoadAllPreservesActiveGenerationWhenReplacementFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pluginRoot := filepath.Join(root, "stable")
	writeTestPluginRoot(t, pluginRoot, "stable")
	host, err := NewHost(Options{BundleOptions: BundleOptions{Dirs: []string{root}}, AuthorityOptions: AuthorityOptions{AllowUnsignedPlugin: true}, RuntimeOptions: RuntimeOptions{Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil))}})
	if err != nil {
		t.Fatal(err)
	}
	if err = host.LoadAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	original := host.plugins["stable"]
	if original == nil {
		t.Fatal("plugin not active")
	}
	bundleDir := filepath.Join(pluginRoot, "bundles", "test-v0.1.0")
	if err = os.WriteFile(filepath.Join(bundleDir, "plugin.star"), []byte("this is not valid Starlark !!!\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = host.LoadAll(context.Background()); err == nil {
		t.Fatal("expected replacement failure")
	}
	if host.plugins["stable"] != original {
		t.Fatal("failed reload replaced active plugin")
	}
	plan, err := host.PlanCommand("slash", "test", []string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := host.Run(context.Background(), "stable", contract.Invocation{InvocationIdentity: contract.InvocationIdentity{Route: plan.Route, Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Author: &contract.UserRef{ID: "175928847299117063"}}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"test"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if message, ok := terminal.(*contract.MessageOperation); !ok || message.Message.Content != "ok" {
		t.Fatalf("terminal=%#v", terminal)
	}
}

func TestFindCommandDefinitionMatchesCommandKind(t *testing.T) {
	definition := contract.Definition{Cogs: []contract.CogDefinition{{Name: "Test", Commands: []contract.CommandDefinition{{Kind: contract.CommandSlash, Name: "same", Route: "command:slash:same"}, {Kind: contract.CommandUser, Name: "same", Route: "command:user:same"}}}}}
	found, ok := findCommandDefinition(definition, contract.CommandUser, []string{"same"})
	if !ok || found.Route != "command:user:same" {
		t.Fatalf("found=%#v ok=%v", found, ok)
	}
}

func TestPlanEventsRetainsEveryListenerAndDeduplicatesPluginSubscription(t *testing.T) {
	definition := contract.Definition{Cogs: []contract.CogDefinition{{Name: "Events", Listeners: []contract.ListenerDefinition{{ID: "one", Event: "guild_member_join", Route: "listener:one"}, {ID: "two", Event: "guild_member_join", Route: "listener:two"}}}}}
	host := &Host{hostRegistry: hostRegistry{plugins: map[string]*Plugin{"events": {PluginSource: PluginSource{ID: "events"}, PluginCatalog: PluginCatalog{Definition: definition}}}}}
	plans, err := host.PlanEvents("events", "guild_member_join")
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[0].Route != "listener:one" || plans[1].Route != "listener:two" {
		t.Fatalf("plans=%#v", plans)
	}
	subscriptions, _ := buildSubscriptions(map[string]*Plugin{"events": {PluginSource: PluginSource{ID: "events"}, PluginCatalog: PluginCatalog{Events: []string{"guild_member_join", "guild_member_join"}}}})
	if got := subscriptions["guild_member_join"]; len(got) != 1 || got[0] != "events" {
		t.Fatalf("subscriptions=%v", got)
	}
}
