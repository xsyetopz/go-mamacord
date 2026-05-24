package adminapi

import (
	"context"
	"errors"
)

func (s *Service) Modules() []ModuleResponse {
	if s.ModuleAdmin == nil {
		return nil
	}
	infos := s.ModuleAdmin.Infos()
	out := make([]ModuleResponse, 0, len(infos))
	for _, info := range infos {
		out = append(out, ModuleResponse{
			ID:             info.ID,
			Name:           info.Name,
			Kind:           string(info.Kind),
			Runtime:        string(info.Runtime),
			Enabled:        info.Enabled,
			DefaultEnabled: info.DefaultEnabled,
			Toggleable:     info.Toggleable,
			Signed:         info.Signed,
			Source:         info.Source,
			Commands:       append([]string(nil), info.Commands...),
		})
	}
	return out
}

func (s *Service) SetModuleEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error {
	if s.ModuleAdmin == nil {
		return errors.New("modules not configured")
	}
	return s.ModuleAdmin.SetEnabled(ctx, moduleID, enabled, actorID)
}

func (s *Service) ResetModule(ctx context.Context, moduleID string) error {
	if s.ModuleAdmin == nil {
		return errors.New("modules not configured")
	}
	return s.ModuleAdmin.Reset(ctx, moduleID)
}

func (s *Service) ReloadModules(ctx context.Context) error {
	if s.ModuleAdmin == nil {
		return errors.New("modules not configured")
	}
	return s.ModuleAdmin.Reload(ctx)
}
