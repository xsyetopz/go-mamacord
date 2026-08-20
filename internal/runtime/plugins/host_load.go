package pluginhost

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkruntime "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark"
	"github.com/xsyetopz/go-mamacord/internal/scheduling"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type pluginLoadLocation struct {
	EntryDir  string
	BundleDir string
	Bundled   bool
}

func (m *Host) LoadAll(ctx context.Context) error {
	pluginDirs, err := m.readPluginDirEntries(ctx)
	if err != nil || pluginDirs == nil {
		return err
	}
	policy, err := permissions.LoadPolicyFile(m.permissionsFile)
	if err != nil {
		return err
	}
	var trustedSigners store.TrustedSignerStore
	if m.store != nil {
		trustedSigners = m.store.TrustedSigners()
	}
	keys, err := LoadTrustedKeys(ctx, m.trustedKeysFile, trustedSigners)
	if err != nil {
		return err
	}
	nextPlugins, nextCommands, err := m.loadPluginsFromEntries(ctx, pluginDirs, keys, policy)
	if err != nil {
		return err
	}
	nextEvents, nextJobs := buildSubscriptions(nextPlugins)
	oldPlugins := m.swapState(nextPlugins, nextCommands, nextEvents, nextJobs, policy)
	closePlugins(oldPlugins)
	return nil
}

func (m *Host) readPluginDirEntries(ctx context.Context) ([]pluginLoadLocation, error) {
	pluginDirs := []pluginLoadLocation{}
	for i, root := range m.dirs {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		entries, err := m.bundles.ListPluginRoots(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read plugins dir %q: %w", root, err)
		}
		for _, entry := range entries {
			entryDir := entry.Dir
			bundleDir, err := m.resolveDiscoveredBundleDir(ctx, entryDir, entry.Name)
			if err != nil {
				return nil, fmt.Errorf("resolve plugin root %q: %w", entryDir, err)
			}
			pluginDirs = append(pluginDirs, pluginLoadLocation{
				EntryDir:  entryDir,
				BundleDir: bundleDir,
				Bundled:   i == 0,
			})
		}
	}
	return pluginDirs, nil
}

func (m *Host) resolveDiscoveredBundleDir(
	ctx context.Context,
	entryDir string,
	entryName string,
) (string, error) {
	entryDir = strings.TrimSpace(entryDir)
	entryName = strings.TrimSpace(entryName)
	if entryDir == "" {
		return "", errors.New("plugin entry dir is required")
	}

	if m != nil && m.store != nil {
		install, ok, err := m.store.PluginInstalls().GetPluginInstall(ctx, entryName)
		if err != nil {
			m.logger.WarnContext(
				ctx,
				"failed to resolve plugin bundle install state, falling back to filesystem",
				slog.String("entry_dir", entryDir),
				slog.String("plugin", entryName),
				slog.String("err", err.Error()),
			)
		} else if ok && strings.TrimSpace(install.BundleRelativeDir) != "" {
			inspection, inspectErr := bundles.InspectBundle(m.bundles, entryDir, install.BundleRelativeDir)
			if inspectErr != nil {
				m.logger.WarnContext(
					ctx,
					"invalid stored plugin bundle, falling back to active bundle",
					slog.String("entry_dir", entryDir),
					slog.String("plugin", entryName),
					slog.String("err", inspectErr.Error()),
				)
			} else {
				return inspection.LoadDir, nil
			}
		}
	}

	inspection, err := bundles.InspectPreferredOrActiveBundle(m.bundles, entryDir, "")
	if err != nil {
		return "", err
	}
	return inspection.LoadDir, nil
}

func (m *Host) loadPluginsFromEntries(ctx context.Context, pluginDirs []pluginLoadLocation, keys map[string]ed25519.PublicKey, policy permissions.Policy) (map[string]*Plugin, map[string]PluginCommand, error) {
	nextPlugins := map[string]*Plugin{}
	nextCommands := map[string]PluginCommand{}
	for _, location := range pluginDirs {
		entryDir := strings.TrimSpace(location.EntryDir)
		if entryDir == "" {
			closePlugins(nextPlugins)
			return nil, nil, errors.New("plugin entry dir is empty")
		}
		plugin, commands, err := m.loadOne(ctx, location, keys, policy)
		if err != nil {
			closePlugins(nextPlugins)
			return nil, nil, fmt.Errorf("load plugin %q: %w", entryDir, err)
		}
		if plugin == nil {
			closePlugins(nextPlugins)
			return nil, nil, fmt.Errorf("load plugin %q returned no plugin", entryDir)
		}
		if _, exists := nextPlugins[plugin.ID]; exists {
			if plugin.Runtime != nil {
				closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				_ = plugin.Runtime.Close(closeCtx)
				cancel()
			}
			closePlugins(nextPlugins)
			return nil, nil, fmt.Errorf("duplicate plugin id %q", plugin.ID)
		}
		nextPlugins[plugin.ID] = plugin
		addCommands(ctx, m.logger, nextCommands, plugin.ID, commands)
	}
	return nextPlugins, nextCommands, nil
}

func (m *Host) loadOne(
	ctx context.Context,
	location pluginLoadLocation,
	keys map[string]ed25519.PublicKey,
	policy permissions.Policy,
) (*Plugin, []PluginCommand, error) {
	entryDir := strings.TrimSpace(location.EntryDir)
	if entryDir == "" {
		return nil, nil, errors.New("plugin entry dir is required")
	}
	bundleDir := strings.TrimSpace(location.BundleDir)
	if bundleDir == "" {
		bundleDir = entryDir
	}
	manifestBytes, err := os.ReadFile(filepath.Join(bundleDir, "plugin.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("read manifest: %w", err)
	}
	manifest, err := ParseStarlarkManifest(manifestBytes)
	if err != nil {
		return nil, nil, err
	}
	signaturePath := filepath.Join(bundleDir, "signature.json")
	var signature *Signature
	if value, readErr := ReadSignature(signaturePath); readErr == nil {
		signature = &value
	} else if !os.IsNotExist(readErr) {
		return nil, nil, readErr
	}
	if signature != nil {
		if err := VerifyDirSignature(bundleDir, *signature, keys); err != nil {
			return nil, nil, err
		}
	} else if m.prodMode && !m.allowUnsignedPlugins {
		return nil, nil, errors.New("missing signature.json")
	}
	if err := validateStarlarkBundleAuthority(bundleDir, manifest); err != nil {
		return nil, nil, err
	}
	bundleHashBefore, err := bundles.HashDir(bundleDir)
	if err != nil {
		return nil, nil, err
	}
	resources, err := readBundleResources(bundleDir, manifest)
	if err != nil {
		return nil, nil, err
	}
	pluginI18n, err := i18n.NewPluginSnapshot(m.i18n, manifest.ID, filepath.Join(bundleDir, "locales"))
	if err != nil {
		return nil, nil, fmt.Errorf("load plugin locales: %w", err)
	}
	requested := manifest.RequestedPermissions()
	granted := policy.Granted(manifest.ID)
	effective := permissions.Effective(requested, granted)
	source, err := starlarkruntime.OpenDirBundle(bundleDir)
	if err != nil {
		return nil, nil, err
	}
	generationID := contract.GenerationID(fmt.Sprintf("%s-%d", manifest.ID, m.generationCounter.Add(1)))
	printFn := func(message string) {
		m.logger.DebugContext(ctx, "plugin print", slog.String("plugin", manifest.ID), slog.String("message", message))
	}
	generation, err := (starlarkruntime.GenerationBuilder{Limits: starlarkruntime.DefaultLimits(), Print: printFn}).Build(ctx, source, generationID)
	if err != nil {
		return nil, nil, err
	}
	manager, err := starlarkruntime.NewGenerationManager(3*time.Second, func(retireErr error) {
		m.logger.Warn("plugin generation retirement", slog.String("plugin", manifest.ID), slog.String("err", retireErr.Error()))
	})
	if err != nil {
		return nil, nil, err
	}
	if err := manager.Activate(generation); err != nil {
		generation.BeginDrain()
		_ = generation.Release()
		return nil, nil, err
	}
	definition := generation.Definition()
	if err := validateAutomationDefinitions(manifest, definition); err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = manager.Close(closeCtx)
		cancel()
		return nil, nil, err
	}
	commands, err := commandsFromContract(definition)
	if err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = manager.Close(closeCtx)
		cancel()
		return nil, nil, err
	}
	bundleHashAfter, err := bundles.HashDir(bundleDir)
	if err != nil || !bytes.Equal(bundleHashBefore[:], bundleHashAfter[:]) {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = manager.Close(closeCtx)
		cancel()
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, errors.New("plugin bundle changed while loading")
	}
	if signature != nil {
		if err := VerifyDirSignature(bundleDir, *signature, keys); err != nil {
			closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = manager.Close(closeCtx)
			cancel()
			return nil, nil, err
		}
	}
	events, jobs := subscriptionsFromContract(manifest.ID, definition)
	plugin := &Plugin{ID: manifest.ID, Dir: entryDir, BundleDir: bundleDir, Bundled: location.Bundled, Manifest: manifest, Signature: signature, Effective: effective, Capabilities: manifest.Capabilities(effective), Resources: resources, Commands: commands, Events: events, Jobs: jobs, Definition: definition, I18n: pluginI18n, Runtime: manager}
	projected := make([]PluginCommand, 0, len(commands))
	for _, command := range commands {
		if command.Name != "" {
			projected = append(projected, PluginCommand{PluginID: manifest.ID, Command: command})
		}
	}
	return plugin, projected, nil
}

func validateAutomationDefinitions(manifest StarlarkManifest, definition contract.Definition) error {
	for _, cog := range definition.Cogs {
		for _, listener := range cog.Listeners {
			switch listener.Event {
			case "guild_member_join", "guild_member_leave":
				if !manifest.Permissions.Automation.Events.MemberJoinLeave {
					return fmt.Errorf("listener %q requires automation.events.member_join_leave", listener.ID)
				}
			case "guild_ban", "guild_unban":
				if !manifest.Permissions.Automation.Events.Moderation {
					return fmt.Errorf("listener %q requires automation.events.moderation", listener.ID)
				}
			default:
				return fmt.Errorf("listener %q uses unsupported event %q", listener.ID, listener.Event)
			}
		}
		for _, task := range cog.Tasks {
			if !manifest.Permissions.Automation.Jobs {
				return fmt.Errorf("task %q requires automation.jobs", task.ID)
			}
			if task.Schedule != strings.TrimSpace(task.Schedule) {
				return fmt.Errorf("task %q schedule is not canonical", task.ID)
			}
			if _, err := scheduling.ParseSchedule(task.Schedule); err != nil {
				return fmt.Errorf("task %q schedule: %w", task.ID, err)
			}
		}
	}
	return nil
}
