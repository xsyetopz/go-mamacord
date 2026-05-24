package pluginhost

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	luaplugin "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
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
				m.logger.Warn(
					"invalid plugin bundle state, skipping plugin root",
					slog.String("entry_dir", entryDir),
					slog.String("err", err.Error()),
				)
				continue
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

func (m *Host) resetPluginLocales() {
	if m.i18n != nil {
		m.i18n.ResetPluginLocales()
	}
}

func (m *Host) loadPluginsFromEntries(
	ctx context.Context,
	pluginDirs []pluginLoadLocation,
	keys map[string]ed25519.PublicKey,
	policy permissions.Policy,
) (map[string]*Plugin, map[string]PluginCommand) {
	nextPlugins := map[string]*Plugin{}
	nextCommands := map[string]PluginCommand{}

	for _, location := range pluginDirs {
		entryDir := strings.TrimSpace(location.EntryDir)
		if entryDir == "" {
			continue
		}
		pl, cmds, err := m.loadOne(ctx, location, keys, policy)
		if err != nil {
			m.logger.WarnContext(
				ctx,
				"failed to load plugin",
				slog.String("dir", entryDir),
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
		m.loadPluginLocales(ctx, pl.ID, pl.BundleDir)
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
func (m *Host) loadOne(
	_ context.Context,
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

	manifestPath := filepath.Join(bundleDir, "plugin.json")
	manifest, err := ReadManifest(manifestPath)
	if err != nil {
		return nil, nil, err
	}
	if permErr := manifest.Permissions.Validate(); permErr != nil {
		return nil, nil, fmt.Errorf("permissions: %w", permErr)
	}

	signaturePath := filepath.Join(bundleDir, "signature.json")
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

		if verifyErr := VerifyDirSignature(bundleDir, *sig, keys); verifyErr != nil {
			return nil, nil, verifyErr
		}
	}

	script := filepath.Join(bundleDir, "plugin.lua")
	granted := policy.Granted(manifest.ID)
	effective := permissions.Effective(manifest.Permissions, granted)

	vm, err := luaplugin.NewFromFile(script, luaplugin.Options{
		Logger:      m.logger,
		PluginID:    manifest.ID,
		PluginDir:   bundleDir,
		Permissions: effective,
		Bridge:      luaplugin.Bridge{Discord: m.bridge.Discord},
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
		Dir:       entryDir,
		BundleDir: bundleDir,
		Bundled:   location.Bundled,
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
