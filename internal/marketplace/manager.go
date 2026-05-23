package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

const (
	sourceKindGit        = "git"
	installKindGit       = "git"
	maxPluginBytes int64 = 32 << 20
	maxFileBytes   int64 = 8 << 20
)

type Store interface {
	TrustedSigners() store.TrustedSignerStore
	MarketplaceSources() store.MarketplaceSourceStore
	MarketplaceSourceSyncs() store.MarketplaceSourceSyncStore
	PluginInstalls() store.PluginInstallStore
	TrustedVendors() store.TrustedVendorStore
	TrustedVendorKeys() store.TrustedVendorKeyStore
	ModuleStates() store.ModuleStateStore
}

type Options struct {
	Logger            *slog.Logger
	Store             Store
	BundledPluginsDir string
	UserPluginsDir    string
	TrustedKeysFile   string
	CacheDir          string
	ProdMode          bool
	AllowUnsigned     bool
	GitBin            string
	Now               func() time.Time
}

type Manager struct {
	logger            *slog.Logger
	store             Store
	bundledPluginsDir string
	userPluginsDir    string
	trustedKeysFile   string
	cacheDir          string
	prodMode          bool
	allowUnsigned     bool
	gitBin            string
	now               func() time.Time

	lockMu sync.Mutex
	locks  map[string]*sync.Mutex
}

func New(opts Options) (*Manager, error) {
	if opts.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if opts.Store == nil {
		return nil, errors.New("store is required")
	}
	userDir := strings.TrimSpace(opts.UserPluginsDir)
	if userDir == "" {
		return nil, errors.New("user plugins dir is required")
	}
	cacheDir := strings.TrimSpace(opts.CacheDir)
	if cacheDir == "" {
		cacheDir = "./data/marketplace_cache"
	}
	gitBin := strings.TrimSpace(opts.GitBin)
	if gitBin == "" {
		gitBin = "git"
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Manager{
		logger:            opts.Logger.With(slog.String("component", "marketplace")),
		store:             opts.Store,
		bundledPluginsDir: strings.TrimSpace(opts.BundledPluginsDir),
		userPluginsDir:    userDir,
		trustedKeysFile:   strings.TrimSpace(opts.TrustedKeysFile),
		cacheDir:          cacheDir,
		prodMode:          opts.ProdMode,
		allowUnsigned:     opts.AllowUnsigned,
		gitBin:            gitBin,
		now:               now,
		locks:             map[string]*sync.Mutex{},
	}, nil
}

func (m *Manager) lock(sourceID string) func() {
	sourceID = strings.TrimSpace(sourceID)
	m.lockMu.Lock()
	mu, ok := m.locks[sourceID]
	if !ok {
		mu = &sync.Mutex{}
		m.locks[sourceID] = mu
	}
	m.lockMu.Unlock()
	mu.Lock()
	return mu.Unlock
}

func (m *Manager) Configured() bool {
	return m != nil
}

func (m *Manager) ListSources(ctx context.Context) ([]Source, error) {
	rows, err := m.store.MarketplaceSources().ListMarketplaceSources(ctx)
	if err != nil {
		return nil, err
	}
	syncs, err := m.store.MarketplaceSourceSyncs().ListMarketplaceSourceSyncs(ctx)
	if err != nil {
		return nil, err
	}
	syncByID := make(map[string]store.MarketplaceSourceSync, len(syncs))
	for _, item := range syncs {
		syncByID[item.SourceID] = item
	}
	out := make([]Source, 0, len(rows))
	for _, row := range rows {
		out = append(out, sourceFromStore(row, syncByID[row.SourceID]))
	}
	return out, nil
}

func (m *Manager) UpsertSource(ctx context.Context, req SourceUpsert) (Source, error) {
	source, err := normalizeSource(req)
	if err != nil {
		return Source{}, err
	}
	existing, ok, err := m.store.MarketplaceSources().GetMarketplaceSource(ctx, source.SourceID)
	if err != nil {
		return Source{}, err
	}
	if ok {
		source.CreatedAt = existing.CreatedAt
	} else {
		source.CreatedAt = m.now().UTC()
	}
	source.UpdatedAt = m.now().UTC()
	if err := m.store.MarketplaceSources().PutMarketplaceSource(ctx, source); err != nil {
		return Source{}, err
	}
	return sourceFromStore(source, store.MarketplaceSourceSync{}), nil
}

func (m *Manager) DeleteSource(ctx context.Context, sourceID string) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return errors.New("source_id is required")
	}
	if err := m.store.MarketplaceSources().DeleteMarketplaceSource(ctx, sourceID); err != nil {
		return err
	}
	if err := m.store.MarketplaceSourceSyncs().DeleteMarketplaceSourceSync(ctx, sourceID); err != nil {
		return err
	}
	return nil
}

func (m *Manager) SyncSource(ctx context.Context, sourceID string) (SyncResult, error) {
	source, ok, err := m.store.MarketplaceSources().GetMarketplaceSource(ctx, sourceID)
	if err != nil {
		return SyncResult{}, err
	}
	if !ok {
		return SyncResult{}, fmt.Errorf("unknown source %q", sourceID)
	}
	if !source.Enabled {
		return SyncResult{}, fmt.Errorf("source %q is disabled", sourceID)
	}
	return m.syncSourceRecord(ctx, source)
}

func (m *Manager) Search(ctx context.Context, query SearchQuery) ([]PluginCandidate, error) {
	sources, err := m.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	term := strings.ToLower(strings.TrimSpace(query.Term))
	out := []PluginCandidate{}
	for _, source := range sources {
		if query.SourceID != "" && source.SourceID != strings.TrimSpace(query.SourceID) {
			continue
		}
		if !source.Enabled {
			continue
		}
		if query.Refresh {
			if _, err := m.SyncSource(ctx, source.SourceID); err != nil {
				source.LastError = err.Error()
			} else {
				refreshed, ok, getErr := m.store.MarketplaceSources().GetMarketplaceSource(ctx, source.SourceID)
				if getErr == nil && ok {
					if sync, syncOK, syncErr := m.store.MarketplaceSourceSyncs().GetMarketplaceSourceSync(ctx, source.SourceID); syncErr == nil && syncOK {
						source = sourceFromStore(refreshed, sync)
					}
				}
			}
		}
		candidates, err := m.scanSourceCandidates(ctx, source)
		if err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			if term == "" || strings.Contains(strings.ToLower(candidate.PluginID), term) || strings.Contains(strings.ToLower(candidate.Name), term) {
				out = append(out, candidate)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PluginID == out[j].PluginID {
			return out[i].SourceID < out[j].SourceID
		}
		return out[i].PluginID < out[j].PluginID
	})
	return out, nil
}

func (m *Manager) Install(ctx context.Context, req InstallRequest) (InstallResult, error) {
	req.SourceID = strings.TrimSpace(req.SourceID)
	req.PluginID = strings.TrimSpace(req.PluginID)
	if req.SourceID == "" || req.PluginID == "" {
		return InstallResult{}, errors.New("source_id and plugin_id are required")
	}

	source, ok, err := m.store.MarketplaceSources().GetMarketplaceSource(ctx, req.SourceID)
	if err != nil {
		return InstallResult{}, err
	}
	if !ok {
		return InstallResult{}, fmt.Errorf("unknown source %q", req.SourceID)
	}
	if !source.Enabled {
		return InstallResult{}, fmt.Errorf("source %q is disabled", req.SourceID)
	}

	candidate, err := m.resolveCandidate(ctx, source, req.PluginID)
	if err != nil {
		return InstallResult{}, err
	}
	targetDir := filepath.Join(m.userPluginsDir, candidate.PluginID)
	install, exists, err := m.store.PluginInstalls().GetPluginInstall(ctx, candidate.PluginID)
	if err != nil {
		return InstallResult{}, err
	}
	if exists && install.SourceID != source.SourceID {
		return InstallResult{}, fmt.Errorf("plugin %q is already managed by source %q", candidate.PluginID, install.SourceID)
	}
	if exists && !req.Force {
		return InstallResult{}, fmt.Errorf("plugin %q is already installed; use update", candidate.PluginID)
	}
	if err := m.checkInstallCollision(ctx, candidate.PluginID, exists); err != nil {
		return InstallResult{}, err
	}
	if candidate.SignatureState != SignatureStateTrusted && m.prodMode && !m.allowUnsigned {
		return InstallResult{}, fmt.Errorf("plugin %q is not signed by a trusted signer", candidate.PluginID)
	}

	hashB64, err := m.copyPluginIntoPlace(candidate.SourcePath, targetDir)
	if err != nil {
		return InstallResult{}, err
	}
	now := m.now().UTC()
	record := store.PluginInstall{
		PluginID:         candidate.PluginID,
		InstallKind:      installKindGit,
		SourceID:         source.SourceID,
		GitURL:           source.GitURL,
		GitRef:           source.GitRef,
		GitRevision:      candidate.GitRevision,
		SourcePath:       strings.TrimPrefix(strings.TrimPrefix(candidate.SourcePath, m.sourceCheckoutRoot(source.SourceID)), string(filepath.Separator)),
		InstalledAt:      now,
		InstalledHashB64: hashB64,
	}
	if req.ActorID != nil {
		record.InstalledBy = req.ActorID
	}
	if err := m.store.PluginInstalls().PutPluginInstall(ctx, record); err != nil {
		return InstallResult{}, err
	}
	if err := m.store.ModuleStates().PutModuleState(ctx, store.ModuleState{
		ModuleID:  candidate.PluginID,
		Enabled:   false,
		UpdatedAt: now,
		UpdatedBy: req.ActorID,
	}); err != nil {
		return InstallResult{}, err
	}

	return InstallResult{
		PluginID:       candidate.PluginID,
		SourceID:       source.SourceID,
		TargetDir:      targetDir,
		GitRevision:    candidate.GitRevision,
		SignatureState: candidate.SignatureState,
		Enabled:        false,
	}, nil
}

func (m *Manager) Update(ctx context.Context, req UpdateRequest) (UpdateResult, error) {
	req.PluginID = strings.TrimSpace(req.PluginID)
	if req.PluginID == "" {
		return UpdateResult{}, errors.New("plugin_id is required")
	}
	install, ok, err := m.store.PluginInstalls().GetPluginInstall(ctx, req.PluginID)
	if err != nil {
		return UpdateResult{}, err
	}
	if !ok {
		return UpdateResult{}, fmt.Errorf("plugin %q is not marketplace-managed", req.PluginID)
	}
	source, ok, err := m.store.MarketplaceSources().GetMarketplaceSource(ctx, install.SourceID)
	if err != nil {
		return UpdateResult{}, err
	}
	if !ok {
		return UpdateResult{}, fmt.Errorf("source %q not found", install.SourceID)
	}
	targetDir := filepath.Join(m.userPluginsDir, req.PluginID)
	localModified := false
	if dirExists(targetDir) {
		localModified, err = DirModified(targetDir, install.InstalledHashB64)
		if err != nil {
			return UpdateResult{}, err
		}
	}
	if localModified && !req.Force {
		return UpdateResult{}, fmt.Errorf("plugin %q has local modifications; use force to replace it", req.PluginID)
	}
	candidate, err := m.resolveCandidate(ctx, source, install.PluginID)
	if err != nil {
		return UpdateResult{}, err
	}
	if candidate.SignatureState != SignatureStateTrusted && m.prodMode && !m.allowUnsigned {
		return UpdateResult{}, fmt.Errorf("plugin %q is not signed by a trusted signer", candidate.PluginID)
	}
	hashB64, err := m.copyPluginIntoPlace(candidate.SourcePath, targetDir)
	if err != nil {
		return UpdateResult{}, err
	}
	record := install
	record.GitURL = source.GitURL
	record.GitRef = source.GitRef
	record.GitRevision = candidate.GitRevision
	record.SourcePath = strings.TrimPrefix(strings.TrimPrefix(candidate.SourcePath, m.sourceCheckoutRoot(source.SourceID)), string(filepath.Separator))
	record.InstalledAt = m.now().UTC()
	record.InstalledHashB64 = hashB64
	record.InstalledBy = req.ActorID
	if err := m.store.PluginInstalls().PutPluginInstall(ctx, record); err != nil {
		return UpdateResult{}, err
	}
	if err := m.store.ModuleStates().PutModuleState(ctx, store.ModuleState{
		ModuleID:  candidate.PluginID,
		Enabled:   false,
		UpdatedAt: m.now().UTC(),
		UpdatedBy: req.ActorID,
	}); err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{
		PluginID:       candidate.PluginID,
		SourceID:       source.SourceID,
		TargetDir:      targetDir,
		GitRevision:    candidate.GitRevision,
		SignatureState: candidate.SignatureState,
		Forced:         req.Force,
	}, nil
}

func (m *Manager) Uninstall(ctx context.Context, req UninstallRequest) error {
	req.PluginID = strings.TrimSpace(req.PluginID)
	if req.PluginID == "" {
		return errors.New("plugin_id is required")
	}
	if _, ok, err := m.store.PluginInstalls().GetPluginInstall(ctx, req.PluginID); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("plugin %q is not marketplace-managed", req.PluginID)
	}
	targetDir := filepath.Join(m.userPluginsDir, req.PluginID)
	if dirExists(targetDir) {
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("remove plugin dir: %w", err)
		}
	}
	if err := m.store.PluginInstalls().DeletePluginInstall(ctx, req.PluginID); err != nil {
		return err
	}
	if err := m.store.ModuleStates().DeleteModuleState(ctx, req.PluginID); err != nil {
		return err
	}
	return nil
}

func (m *Manager) TrustSigner(ctx context.Context, req TrustSignerRequest) error {
	if strings.TrimSpace(req.KeyID) == "" || strings.TrimSpace(req.PublicKeyB64) == "" {
		return errors.New("key_id and public_key_b64 are required")
	}
	return m.store.TrustedSigners().PutTrustedSigner(ctx, store.TrustedSigner{
		KeyID:        strings.TrimSpace(req.KeyID),
		PublicKeyB64: strings.TrimSpace(req.PublicKeyB64),
		AddedAt:      m.now().UTC(),
	})
}

func (m *Manager) TrustVendor(ctx context.Context, req TrustVendorRequest) (TrustVendorResult, error) {
	vendorID := strings.TrimSpace(req.VendorID)
	name := strings.TrimSpace(req.Name)
	if vendorID == "" || name == "" {
		return TrustVendorResult{}, errors.New("vendor_id and name are required")
	}
	path, err := m.resolveTrustedKeysImport(ctx, req)
	if err != nil {
		return TrustVendorResult{}, err
	}
	keys, err := pluginhost.ReadTrustedKeysFile(path)
	if err != nil {
		return TrustVendorResult{}, err
	}
	keyIDs := make([]string, 0, len(keys))
	for keyID, publicKey := range keys {
		req := TrustSignerRequest{
			KeyID:        keyID,
			PublicKeyB64: base64.StdEncoding.EncodeToString(publicKey),
			VendorID:     vendorID,
		}
		if err := m.TrustSigner(ctx, req); err != nil {
			return TrustVendorResult{}, err
		}
		keyIDs = append(keyIDs, keyID)
	}
	sort.Strings(keyIDs)
	now := m.now().UTC()
	if err := m.store.TrustedVendors().PutTrustedVendor(ctx, store.TrustedVendor{
		VendorID:   vendorID,
		Name:       name,
		WebsiteURL: strings.TrimSpace(req.WebsiteURL),
		SupportURL: strings.TrimSpace(req.SupportURL),
		AddedAt:    now,
		UpdatedAt:  now,
	}); err != nil {
		return TrustVendorResult{}, err
	}
	vendorKeys := make([]store.TrustedVendorKey, 0, len(keyIDs))
	for _, keyID := range keyIDs {
		vendorKeys = append(vendorKeys, store.TrustedVendorKey{VendorID: vendorID, KeyID: keyID})
	}
	if err := m.store.TrustedVendorKeys().ReplaceTrustedVendorKeys(ctx, vendorID, vendorKeys); err != nil {
		return TrustVendorResult{}, err
	}
	return TrustVendorResult{VendorID: vendorID, KeyIDs: keyIDs}, nil
}

func (m *Manager) resolveTrustedKeysImport(ctx context.Context, req TrustVendorRequest) (string, error) {
	if path := strings.TrimSpace(req.TrustedKeysPath); path != "" && req.SourceID == "" {
		return path, nil
	}
	if strings.TrimSpace(req.SourceID) == "" {
		return "", errors.New("trusted_keys_path or source_id is required")
	}
	source, ok, err := m.store.MarketplaceSources().GetMarketplaceSource(ctx, req.SourceID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("unknown source %q", req.SourceID)
	}
	if _, err := m.syncSourceRecord(ctx, source); err != nil {
		return "", err
	}
	rel := strings.TrimSpace(req.TrustedKeysPath)
	if rel == "" {
		rel = "trusted_keys.json"
	}
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return "", errors.New("trusted_keys_path must stay within the source checkout")
	}
	return filepath.Join(m.sourceCheckoutRoot(source.SourceID), clean), nil
}

func (m *Manager) resolveCandidate(ctx context.Context, source store.MarketplaceSource, pluginID string) (PluginCandidate, error) {
	if _, err := m.syncSourceRecord(ctx, source); err != nil {
		return PluginCandidate{}, err
	}
	candidates, err := m.scanSourceCandidates(ctx, sourceFromStore(source, store.MarketplaceSourceSync{}))
	if err != nil {
		return PluginCandidate{}, err
	}
	for _, candidate := range candidates {
		if candidate.PluginID == pluginID {
			return candidate, nil
		}
	}
	return PluginCandidate{}, fmt.Errorf("plugin %q not found in source %q", pluginID, source.SourceID)
}

func (m *Manager) scanSourceCandidates(ctx context.Context, source Source) ([]PluginCandidate, error) {
	root := filepath.Join(m.sourceCheckoutRoot(source.SourceID), filepath.Clean(strings.TrimPrefix(source.GitSubdir, string(filepath.Separator))))
	if strings.TrimSpace(source.GitSubdir) == "" {
		root = m.sourceCheckoutRoot(source.SourceID)
	}
	if !dirExists(root) {
		if source.LastError != "" {
			return []PluginCandidate{{
				SourceID:    source.SourceID,
				SyncError:   source.LastError,
				GitRevision: source.LastRevision,
			}}, nil
		}
		return nil, nil
	}
	revision, _ := m.gitRevision(ctx, m.sourceCheckoutRoot(source.SourceID))
	seen := map[string]string{}
	out := []PluginCandidate{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		manifestPath := filepath.Join(path, "plugin.json")
		luaPath := filepath.Join(path, "plugin.lua")
		if !fileExists(manifestPath) || !fileExists(luaPath) {
			return nil
		}
		manifest, err := pluginhost.ReadManifest(manifestPath)
		if err != nil {
			return nil
		}
		if previous, ok := seen[manifest.ID]; ok {
			return fmt.Errorf("source %q contains duplicate plugin id %q in %s and %s", source.SourceID, manifest.ID, previous, path)
		}
		seen[manifest.ID] = path
		state, signerKeyID := SignatureStateForDir(ctx, path, m.trustedKeysFile, m.store)
		out = append(out, PluginCandidate{
			SourceID:       source.SourceID,
			PluginID:       manifest.ID,
			Name:           manifest.Name,
			Version:        manifest.Version,
			SourcePath:     path,
			GitRevision:    revision,
			Commands:       manifestCommandNames(manifest.Commands),
			SignatureState: state,
			SignerKeyID:    signerKeyID,
			SyncError:      source.LastError,
		})
		return filepath.SkipDir
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PluginID < out[j].PluginID })
	return out, nil
}

func (m *Manager) syncSourceRecord(ctx context.Context, source store.MarketplaceSource) (SyncResult, error) {
	unlock := m.lock(source.SourceID)
	defer unlock()

	root := m.sourceCheckoutRoot(source.SourceID)
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		return SyncResult{}, err
	}
	var syncErr error
	if !dirExists(root) {
		syncErr = m.git(ctx, "", source, "clone", "--no-checkout", source.GitURL, root)
	} else {
		syncErr = m.git(ctx, root, source, "remote", "set-url", "origin", source.GitURL)
	}
	if syncErr == nil {
		args := []string{"fetch", "--depth", "1", "origin"}
		if ref := strings.TrimSpace(source.GitRef); ref != "" {
			args = append(args, ref)
		}
		syncErr = m.git(ctx, root, source, args...)
	}
	if syncErr == nil {
		syncErr = m.git(ctx, root, source, "checkout", "-f", "FETCH_HEAD")
	}
	revision := ""
	if syncErr == nil {
		revision, syncErr = m.gitRevision(ctx, root)
	}
	now := m.now().UTC()
	syncRow := store.MarketplaceSourceSync{
		SourceID:     source.SourceID,
		LastRevision: revision,
	}
	if syncErr == nil {
		syncRow.LastSyncedAt = &now
	} else {
		syncRow.LastError = syncErr.Error()
	}
	if err := m.store.MarketplaceSourceSyncs().PutMarketplaceSourceSync(ctx, syncRow); err != nil {
		return SyncResult{}, err
	}
	if syncErr != nil {
		return SyncResult{}, syncErr
	}
	return SyncResult{
		SourceID:     source.SourceID,
		Revision:     revision,
		SyncedAt:     now,
		LastSyncedAt: &now,
	}, nil
}

func (m *Manager) git(ctx context.Context, dir string, source store.MarketplaceSource, args ...string) error {
	cmd := exec.CommandContext(ctx, m.gitBin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	askPassPath := ""
	if tokenEnvVar := strings.TrimSpace(source.TokenEnvVar); tokenEnvVar != "" {
		token := strings.TrimSpace(os.Getenv(tokenEnvVar))
		if token == "" {
			return fmt.Errorf("token env var %q is empty", tokenEnvVar)
		}
		path, err := writeAskPassScript(token)
		if err != nil {
			return err
		}
		askPassPath = path
		defer os.Remove(path)
		cmd.Env = append(cmd.Env,
			"GIT_ASKPASS="+askPassPath,
			"MAMACORD_GIT_TOKEN="+token,
			"MAMACORD_GIT_USERNAME=x-access-token",
		)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), redactToken(strings.TrimSpace(message), source))
	}
	return nil
}

func writeAskPassScript(token string) (string, error) {
	tmp, err := os.CreateTemp("", "mamacord-git-askpass-*.sh")
	if err != nil {
		return "", err
	}
	defer tmp.Close()
	content := "#!/bin/sh\ncase \"$1\" in\n*Username*) printf '%s' \"$MAMACORD_GIT_USERNAME\" ;;\n*) printf '%s' \"$MAMACORD_GIT_TOKEN\" ;;\nesac\n"
	if _, err := tmp.WriteString(content); err != nil {
		return "", err
	}
	if err := tmp.Chmod(0o700); err != nil {
		return "", err
	}
	_ = token
	return tmp.Name(), nil
}

func (m *Manager) gitRevision(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, m.gitBin, "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (m *Manager) checkInstallCollision(ctx context.Context, pluginID string, hasInstall bool) error {
	if m.bundledPluginsDir != "" && fileExists(filepath.Join(m.bundledPluginsDir, pluginID, "plugin.json")) {
		return fmt.Errorf("plugin %q conflicts with a bundled plugin", pluginID)
	}
	userDir := filepath.Join(m.userPluginsDir, pluginID)
	if fileExists(filepath.Join(userDir, "plugin.json")) && !hasInstall {
		return fmt.Errorf("plugin %q already exists in the user plugin root", pluginID)
	}
	if _, ok, err := m.store.PluginInstalls().GetPluginInstall(ctx, pluginID); err != nil {
		return err
	} else if ok && !hasInstall {
		return fmt.Errorf("plugin %q is already marketplace-managed", pluginID)
	}
	return nil
}

func (m *Manager) copyPluginIntoPlace(srcDir, targetDir string) (string, error) {
	if err := os.MkdirAll(m.userPluginsDir, 0o755); err != nil {
		return "", err
	}
	parent := filepath.Dir(targetDir)
	tmpRoot := filepath.Join(parent, ".tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return "", err
	}
	tmpDir, err := os.MkdirTemp(tmpRoot, filepath.Base(targetDir)+".")
	if err != nil {
		return "", err
	}
	if err := copyDirSafe(srcDir, tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", err
	}
	hash, err := pluginhost.HashDir(tmpDir)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", err
	}
	backupDir := ""
	if dirExists(targetDir) {
		backupDir = filepath.Join(tmpRoot, filepath.Base(targetDir)+".old."+m.now().UTC().Format("20060102150405"))
		if err := os.Rename(targetDir, backupDir); err != nil {
			_ = os.RemoveAll(tmpDir)
			return "", fmt.Errorf("move existing plugin: %w", err)
		}
	}
	if err := os.Rename(tmpDir, targetDir); err != nil {
		if backupDir != "" {
			_ = os.Rename(backupDir, targetDir)
		}
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("activate plugin: %w", err)
	}
	if backupDir != "" {
		_ = os.RemoveAll(backupDir)
	}
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

func copyDirSafe(srcDir, dstDir string) error {
	var totalBytes int64
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in marketplace plugins: %s", path)
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxFileBytes {
			return fmt.Errorf("file %s exceeds max file size", rel)
		}
		totalBytes += info.Size()
		if totalBytes > maxPluginBytes {
			return errors.New("plugin exceeds maximum size")
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func sourceFromStore(source store.MarketplaceSource, sync store.MarketplaceSourceSync) Source {
	return Source{
		SourceID:     source.SourceID,
		Kind:         source.Kind,
		GitURL:       source.GitURL,
		GitRef:       source.GitRef,
		GitSubdir:    source.GitSubdir,
		TokenEnvVar:  source.TokenEnvVar,
		Enabled:      source.Enabled,
		CreatedAt:    source.CreatedAt,
		UpdatedAt:    source.UpdatedAt,
		LastSyncedAt: sync.LastSyncedAt,
		LastRevision: sync.LastRevision,
		LastError:    sync.LastError,
	}
}

func normalizeSource(req SourceUpsert) (store.MarketplaceSource, error) {
	sourceID := strings.TrimSpace(req.SourceID)
	if sourceID == "" {
		return store.MarketplaceSource{}, errors.New("source_id is required")
	}
	if !isSimpleID(sourceID) {
		return store.MarketplaceSource{}, fmt.Errorf("source_id %q is invalid", sourceID)
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = sourceKindGit
	}
	if kind != sourceKindGit {
		return store.MarketplaceSource{}, fmt.Errorf("unsupported source kind %q", kind)
	}
	gitURL := strings.TrimSpace(req.GitURL)
	if gitURL == "" {
		return store.MarketplaceSource{}, errors.New("git_url is required")
	}
	tokenEnvVar := strings.TrimSpace(req.TokenEnvVar)
	if tokenEnvVar != "" && strings.Contains(tokenEnvVar, "=") {
		return store.MarketplaceSource{}, errors.New("token_env_var must be an env var name, not a value")
	}
	return store.MarketplaceSource{
		SourceID:    sourceID,
		Kind:        kind,
		GitURL:      gitURL,
		GitRef:      strings.TrimSpace(req.GitRef),
		GitSubdir:   strings.TrimSpace(req.GitSubdir),
		TokenEnvVar: tokenEnvVar,
		Enabled:     req.Enabled,
	}, nil
}

func isSimpleID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func manifestCommandNames(cmds []pluginhost.Command) []string {
	out := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		if name := strings.TrimSpace(cmd.Name); name != "" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func SignatureStateForDir(ctx context.Context, dir, trustedKeysFile string, src pluginhost.TrustedSignerSource) (SignatureState, string) {
	sigPath := filepath.Join(dir, "signature.json")
	if !fileExists(sigPath) {
		return SignatureStateUnsigned, ""
	}
	sig, err := pluginhost.ReadSignature(sigPath)
	if err != nil {
		return SignatureStateInvalid, ""
	}
	keys, err := pluginhost.LoadTrustedKeys(ctx, trustedKeysFile, src)
	if err != nil {
		return SignatureStateInvalid, sig.KeyID
	}
	if err := pluginhost.VerifyDirSignature(dir, sig, keys); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unknown signer") {
			return SignatureStateUntrusted, sig.KeyID
		}
		return SignatureStateInvalid, sig.KeyID
	}
	return SignatureStateTrusted, sig.KeyID
}

func DirModified(dir, installedHashB64 string) (bool, error) {
	hash, err := pluginhost.HashDir(dir)
	if err != nil {
		return false, err
	}
	return base64.StdEncoding.EncodeToString(hash[:]) != strings.TrimSpace(installedHashB64), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func redactToken(message string, source store.MarketplaceSource) string {
	tokenEnvVar := strings.TrimSpace(source.TokenEnvVar)
	if tokenEnvVar == "" {
		return message
	}
	token := strings.TrimSpace(os.Getenv(tokenEnvVar))
	if token == "" {
		return message
	}
	return strings.ReplaceAll(message, token, "[redacted]")
}

func (m *Manager) sourceCheckoutRoot(sourceID string) string {
	return filepath.Join(m.cacheDir, "sources", strings.TrimSpace(sourceID))
}

func installHashForDir(dir string) string {
	hash, err := pluginhost.HashDir(dir)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(hash[:])
}

func SourceIDForURL(url string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(url)))
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}
