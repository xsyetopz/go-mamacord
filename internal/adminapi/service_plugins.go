package adminapi

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

func (s *Service) Plugins() ([]PluginSummary, error) {
	infosByID := map[string]pluginhost.PluginInfo{}
	if s.PluginAdmin != nil {
		for _, info := range s.PluginAdmin.Infos() {
			infosByID[info.ID] = info
		}
	}

	roots := []struct {
		dir     string
		bundled bool
	}{
		{dir: strings.TrimSpace(s.Config.BundledPluginsDir), bundled: true},
		{dir: strings.TrimSpace(s.Config.UserPluginsDir), bundled: false},
	}
	installsByID := map[string]store.PluginInstall{}
	if s.Store != nil {
		installs, err := s.Store.PluginInstalls().ListPluginInstalls(context.Background())
		if err != nil {
			return nil, err
		}
		for _, install := range installs {
			installsByID[install.PluginID] = install
		}
	}

	outByID := map[string]PluginSummary{}
	bundleDirsByID := map[string]string{}
	bundleRepo := s.bundleRepo()
	for _, root := range roots {
		if root.dir == "" {
			continue
		}
		entries, err := bundleRepo.ListPluginRoots(root.dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			id := entry.Name
			entryDir := entry.Dir
			preferredBundleRel := ""
			if install, ok := installsByID[id]; ok && strings.TrimSpace(install.BundleRelativeDir) != "" {
				preferredBundleRel = install.BundleRelativeDir
			}
			inspection, err := bundles.InspectPreferredOrActiveBundle(bundleRepo, entryDir, preferredBundleRel)
			if err != nil {
				continue
			}

			summary := PluginSummary{
				ID:                id,
				PluginRoot:        entryDir,
				Bundled:           root.bundled,
				HasSignatureFile:  inspection.HasSignatureFile,
				BundleRelativeDir: inspection.BundleRelativeDir,
			}
			if root.bundled {
				summary.ProvenanceKind = string(marketplace.ProvenanceKindBundled)
			} else {
				summary.ProvenanceKind = string(marketplace.ProvenanceKindManual)
			}
			if manifest, err := pluginhost.ParseManifest(inspection.ManifestBytes); err == nil {
				summary.ID = manifest.ID
				summary.Name = manifest.Name
				summary.Version = manifest.Version
			}
			if existing, ok := outByID[summary.ID]; ok && !existing.Bundled {
				continue
			}
			outByID[summary.ID] = summary
			bundleDirsByID[summary.ID] = inspection.BundleDir
		}
	}

	out := make([]PluginSummary, 0, len(outByID))
	for id, summary := range outByID {
		if install, ok := installsByID[id]; ok {
			summary.ProvenanceKind = string(marketplace.ProvenanceKindMarketplace)
			summary.SourceID = install.SourceID
			summary.GitRevision = install.GitRevision
			if modified, err := bundleRepo.BundleModified(summary.PluginRoot, install.InstalledHashB64); err == nil {
				summary.LocalModified = modified
			}
		}
		if bundleDir := bundleDirsByID[id]; bundleDir != "" {
			state, _ := marketplace.SignatureStateForDir(context.Background(), bundleDir, s.Config.TrustedKeysFile, s.Store)
			summary.SignatureState = string(state)
		}
		if info, ok := infosByID[id]; ok {
			summary.Name = fallbackString(summary.Name, info.Name)
			summary.Version = fallbackString(summary.Version, info.Version)
			summary.PluginRoot = fallbackString(summary.PluginRoot, info.Dir)
			summary.Commands = make([]string, 0, len(info.Commands))
			for _, cmd := range info.Commands {
				if strings.TrimSpace(cmd.Name) != "" {
					summary.Commands = append(summary.Commands, cmd.Name)
				}
			}
			summary.Loaded = true
			summary.Signed = info.Signed
		}
		out = append(out, summary)
	}

	slices.SortFunc(out, func(a, b PluginSummary) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

func (s *Service) ReloadPlugins(ctx context.Context) error {
	if s.PluginAdmin == nil {
		return errors.New("plugins not configured")
	}
	return s.PluginAdmin.Reload(ctx)
}

func (s *Service) MarketplaceSources(ctx context.Context) ([]marketplace.Source, error) {
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		return nil, errors.New("marketplace not configured")
	}
	return s.Marketplace.ListSources(ctx)
}

func (s *Service) UpsertMarketplaceSource(ctx context.Context, req marketplace.SourceUpsert) (marketplace.Source, error) {
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		return marketplace.Source{}, errors.New("marketplace not configured")
	}
	return s.Marketplace.UpsertSource(ctx, req)
}

func (s *Service) DeleteMarketplaceSource(ctx context.Context, sourceID string) error {
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		return errors.New("marketplace not configured")
	}
	return s.Marketplace.DeleteSource(ctx, sourceID)
}

func (s *Service) SyncMarketplaceSource(ctx context.Context, sourceID string) (marketplace.SyncResult, error) {
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		return marketplace.SyncResult{}, errors.New("marketplace not configured")
	}
	return s.Marketplace.SyncSource(ctx, sourceID)
}

func (s *Service) SearchMarketplace(ctx context.Context, query marketplace.SearchQuery) ([]marketplace.PluginCandidate, error) {
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		return nil, errors.New("marketplace not configured")
	}
	return s.Marketplace.Search(ctx, query)
}

func (s *Service) InstallMarketplacePlugin(ctx context.Context, actorID uint64, req MarketplaceInstallRequest) (marketplace.InstallResult, error) {
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		return marketplace.InstallResult{}, errors.New("marketplace not configured")
	}
	actor := actorID
	return s.Marketplace.Install(ctx, marketplace.InstallRequest{
		SourceID: req.SourceID,
		PluginID: req.PluginID,
		Force:    req.Force,
		ActorID:  &actor,
	})
}

func (s *Service) UpdateMarketplacePlugin(ctx context.Context, actorID uint64, req MarketplaceUpdateRequest) (marketplace.UpdateResult, error) {
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		return marketplace.UpdateResult{}, errors.New("marketplace not configured")
	}
	actor := actorID
	return s.Marketplace.Update(ctx, marketplace.UpdateRequest{
		PluginID: req.PluginID,
		Force:    req.Force,
		ActorID:  &actor,
	})
}

func (s *Service) UninstallMarketplacePlugin(ctx context.Context, req MarketplaceUninstallRequest) error {
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		return errors.New("marketplace not configured")
	}
	return s.Marketplace.Uninstall(ctx, marketplace.UninstallRequest{PluginID: req.PluginID})
}

func (s *Service) TrustMarketplaceSigner(ctx context.Context, req MarketplaceTrustSignerRequest) error {
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		return errors.New("marketplace not configured")
	}
	return s.Marketplace.TrustSigner(ctx, marketplace.TrustSignerRequest{
		KeyID:        req.KeyID,
		PublicKeyB64: req.PublicKeyB64,
		VendorID:     req.VendorID,
	})
}

func (s *Service) TrustMarketplaceVendor(ctx context.Context, req MarketplaceTrustVendorRequest) (marketplace.TrustVendorResult, error) {
	if s.Marketplace == nil || !s.Marketplace.Configured() {
		return marketplace.TrustVendorResult{}, errors.New("marketplace not configured")
	}
	return s.Marketplace.TrustVendor(ctx, marketplace.TrustVendorRequest{
		VendorID:        req.VendorID,
		Name:            req.Name,
		WebsiteURL:      req.WebsiteURL,
		SupportURL:      req.SupportURL,
		TrustedKeysPath: req.TrustedKeysPath,
		SourceID:        req.SourceID,
	})
}
