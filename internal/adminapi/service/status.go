package service

import (
	"context"
	"net/url"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/config"
)

func (s *Service) Status(ctx context.Context) (StatusResponse, error) {
	var devGuildID *Snowflake
	if s.Config.Discord.DevGuildID != nil {
		v := Snowflake(*s.Config.Discord.DevGuildID)
		devGuildID = &v
	}
	resp := StatusResponse{
		Config: StatusConfig{
			StorageStatusConfig: StorageStatusConfig{
				StorageBackend: string(s.Config.Storage.Backend),
				StorageTarget:  storageTargetLabel(s.Config),
				MigrationsDir:  s.Config.Storage.Migrations,
			},
			FileStatusConfig: FileStatusConfig{
				LocalesDir:        s.Config.Files.LocalesDir,
				BundledPluginsDir: s.Config.Bundles.BundledPluginsDir,
				UserPluginsDir:    s.Config.Bundles.UserPluginsDir,
				PermissionsFile:   s.Config.Files.PermissionsFile,
				ModulesFile:       s.Config.Files.ModulesFile,
				TrustedKeysFile:   s.Config.Plugins.TrustedKeysFile,
			},
			EndpointStatusConfig: EndpointStatusConfig{
				OpsAddr:   s.Config.Runtime.OpsAddr,
				AdminAddr: s.Config.Runtime.AdminAddr,
			},
			RuntimeStatusConfig: RuntimeStatusConfig{
				RuntimeRoles:            s.Config.RuntimeRoleStrings(),
				DevGuildID:              devGuildID,
				CommandRegistrationMode: s.Config.Commands.RegistrationMode,
				ProdMode:                s.Config.Runtime.ProdMode,
				AllowUnsignedPlugins:    s.Config.Plugins.AllowUnsigned,
			},
		},
		Setup: s.setupResponse(false),
	}
	if s.BuildInfo != nil {
		resp.Build = buildResponse(s.BuildInfo())
	}
	if s.Snapshot != nil {
		resp.Snapshot = snapshotResponse(s.Snapshot())
	}
	keys, err := s.TrustedKeys(ctx)
	if err != nil {
		return StatusResponse{}, err
	}
	resp.Setup.TrustedKeysConfigured = len(keys.FileKeys) > 0 || len(keys.DBKeys) > 0
	return resp, nil
}

func storageTargetLabel(cfg config.Config) string {
	switch cfg.Storage.Backend {
	case config.StorageBackendPostgres:
		dsn := strings.TrimSpace(cfg.Storage.PostgresDSN)
		if dsn == "" {
			return ""
		}
		parsed, err := url.Parse(dsn)
		if err != nil {
			return "<invalid postgres dsn>"
		}
		if parsed.User != nil {
			username := parsed.User.Username()
			if username != "" {
				parsed.User = url.UserPassword(username, "***")
			} else {
				parsed.User = nil
			}
		}
		return parsed.String()
	default:
		return ""
	}
}

func (s *Service) Setup(ctx context.Context) (SetupResponse, error) {
	resp := s.setupResponse(true)
	keys, err := s.TrustedKeys(ctx)
	if err != nil {
		return SetupResponse{}, err
	}
	resp.TrustedKeysConfigured = len(keys.FileKeys) > 0 || len(keys.DBKeys) > 0
	return resp, nil
}
