package adminapi

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	"github.com/xsyetopz/go-mamacord/internal/storagebootstrap"
)

func (s *Service) LoadModulesConfig() (config.ModulesFile, error) {
	return config.LoadModulesFile(s.Config.ModulesFile)
}

func (s *Service) SaveModulesConfig(file config.ModulesFile) error {
	return config.WriteModulesFile(s.Config.ModulesFile, file)
}

func (s *Service) LoadPermissionsConfig() (permissions.Policy, error) {
	return permissions.LoadPolicyFile(s.Config.PermissionsFile)
}

func (s *Service) SavePermissionsConfig(policy permissions.Policy) error {
	return permissions.WritePolicyFile(s.Config.PermissionsFile, policy)
}

func (s *Service) TrustedKeys(ctx context.Context) (TrustedKeysResponse, error) {
	resp := TrustedKeysResponse{}
	path := strings.TrimSpace(s.Config.TrustedKeysFile)
	if path != "" && fileExists(path) {
		bytes, err := os.ReadFile(path)
		if err != nil {
			return TrustedKeysResponse{}, err
		}
		var file pluginhost.TrustedKeys
		if err := json.Unmarshal(bytes, &file); err != nil {
			return TrustedKeysResponse{}, err
		}
		resp.FileKeys = make([]TrustedKeyResponse, 0, len(file.Keys))
		for _, key := range file.Keys {
			resp.FileKeys = append(resp.FileKeys, TrustedKeyResponse{
				KeyID:        key.KeyID,
				PublicKeyB64: key.PublicKeyB64,
			})
		}
	}
	if s.Store != nil {
		keys, err := s.Store.TrustedSigners().ListTrustedSigners(ctx)
		if err != nil {
			return TrustedKeysResponse{}, err
		}
		resp.DBKeys = make([]TrustedSignerResponse, 0, len(keys))
		for _, key := range keys {
			resp.DBKeys = append(resp.DBKeys, TrustedSignerResponse{
				KeyID:        key.KeyID,
				PublicKeyB64: key.PublicKeyB64,
				AddedAt:      formatTime(key.AddedAt),
			})
		}
	}
	return resp, nil
}

func (s *Service) MigrationStatus(ctx context.Context) (MigrationStatusResponse, error) {
	status, err := storagebootstrap.MigrationStatus(ctx, s.Config)
	if err != nil {
		return MigrationStatusResponse{}, err
	}
	return migrationStatusResponse(status), nil
}

func (s *Service) MigrateUp(ctx context.Context) (MigrationStatusResponse, error) {
	status, err := storagebootstrap.MigrateUp(ctx, s.Config)
	if err != nil {
		return MigrationStatusResponse{}, err
	}
	return migrationStatusResponse(status), nil
}
