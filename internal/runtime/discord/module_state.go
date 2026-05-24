package discordruntime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

func (b *Bot) loadModuleStates(ctx context.Context) (map[string]store.ModuleState, error) {
	if b.store == nil {
		return nil, errors.New("store not configured")
	}

	states, err := b.store.ModuleStates().ListModuleStates(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[string]store.ModuleState, len(states))
	for _, state := range states {
		if strings.TrimSpace(state.ModuleID) == "" {
			continue
		}
		out[state.ModuleID] = state
	}
	return out, nil
}

func (b *Bot) moduleInfos() []moduleapi.Info {
	out := make([]moduleapi.Info, 0, len(b.modules))
	for _, info := range b.modules {
		info.Commands = append([]string(nil), info.Commands...)
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (b *Bot) moduleInfo(moduleID string) (moduleapi.Info, bool) {
	info, ok := b.modules[strings.TrimSpace(moduleID)]
	return info, ok
}

func (b *Bot) moduleEnabled(moduleID string) bool {
	info, ok := b.moduleInfo(moduleID)
	return ok && info.Enabled
}

func (b *Bot) setModuleEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return errors.New("module id is required")
	}

	info, ok := b.moduleInfo(moduleID)
	if !ok {
		return fmt.Errorf("unknown module %q", moduleID)
	}
	if !info.Toggleable {
		return fmt.Errorf("module %q is required and cannot be disabled", moduleID)
	}

	state := store.ModuleState{
		ModuleID:  moduleID,
		Enabled:   enabled,
		UpdatedAt: time.Now().UTC(),
	}
	if actorID != 0 {
		state.UpdatedBy = &actorID
	}
	if err := b.store.ModuleStates().PutModuleState(ctx, state); err != nil {
		return err
	}
	return b.reloadModules(ctx)
}

func (b *Bot) resetModule(ctx context.Context, moduleID string) error {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return errors.New("module id is required")
	}

	info, ok := b.moduleInfo(moduleID)
	if !ok {
		return fmt.Errorf("unknown module %q", moduleID)
	}
	if !info.Toggleable {
		return fmt.Errorf("module %q is required and cannot be reset", moduleID)
	}

	if err := b.store.ModuleStates().DeleteModuleState(ctx, moduleID); err != nil {
		return err
	}
	return b.reloadModules(ctx)
}

func (b *Bot) reloadModules(ctx context.Context) error {
	if b.pluginHost != nil {
		if err := b.pluginHost.LoadAll(ctx); err != nil {
			return err
		}
	}
	if err := b.refreshRuntimeCatalog(ctx); err != nil {
		return err
	}
	if err := syncGatewayCommands(
		ctx,
		b.enableGateway,
		b.commandRegisterAllGuilds,
		b.devGuildID,
		b.registerCommands,
		func(ctx context.Context) error {
			return b.commandRegistrar().RegisterInCachedGuilds(ctx)
		},
	); err != nil {
		return err
	}
	if b.enableScheduler && b.scheduler != nil && b.ready.Load() {
		b.scheduler.Restart(ctx)
	}
	return nil
}

func syncGatewayCommands(
	ctx context.Context,
	enableGateway bool,
	registerAllGuilds bool,
	devGuildID *uint64,
	registerCommands func(context.Context) error,
	registerCachedGuilds func(context.Context) error,
) error {
	if !enableGateway {
		return nil
	}
	if registerCommands != nil {
		if err := registerCommands(ctx); err != nil {
			return err
		}
	}
	if registerAllGuilds && devGuildID == nil && registerCachedGuilds != nil {
		if err := registerCachedGuilds(ctx); err != nil {
			return err
		}
	}
	return nil
}
