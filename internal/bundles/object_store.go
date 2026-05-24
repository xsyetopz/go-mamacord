package bundles

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ObjectStore interface {
	PutBundle(pluginID string, bundleRelativeDir string, srcDir string, hashB64 string) error
	MaterializeBundle(pluginID string, bundleRelativeDir string, dstDir string) error
	WriteBundleFile(pluginID string, bundleRelativeDir string, rel string, bytes []byte) error
	RemovePlugin(pluginID string) error
}

type DirObjectStore struct {
	root string
}

func NewDirObjectStore(root string) (DirObjectStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return DirObjectStore{}, errors.New("object store root is required")
	}
	return DirObjectStore{root: root}, nil
}

func (s DirObjectStore) PutBundle(pluginID string, bundleRelativeDir string, srcDir string, hashB64 string) error {
	pluginID = strings.TrimSpace(pluginID)
	bundleRelativeDir = strings.TrimSpace(bundleRelativeDir)
	if pluginID == "" {
		return errors.New("plugin id is required")
	}
	if bundleRelativeDir == "" {
		return errors.New("bundle relative dir is required")
	}
	target := filepath.Join(s.root, pluginID, bundleRelativeDir)
	if dirExists(target) {
		modified, err := DirModified(target, hashB64)
		if err != nil {
			return err
		}
		if modified {
			return fmt.Errorf("bundle %q already exists with different contents", bundleRelativeDir)
		}
		return nil
	}
	tmpRoot := filepath.Join(s.root, ".tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(tmpRoot, "bundle.")
	if err != nil {
		return err
	}
	if err := copyDirSafe(srcDir, tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	if err := os.Rename(tmpDir, target); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("activate object-store bundle: %w", err)
	}
	return nil
}

func (s DirObjectStore) MaterializeBundle(pluginID string, bundleRelativeDir string, dstDir string) error {
	pluginID = strings.TrimSpace(pluginID)
	bundleRelativeDir = strings.TrimSpace(bundleRelativeDir)
	if pluginID == "" {
		return errors.New("plugin id is required")
	}
	if bundleRelativeDir == "" {
		return errors.New("bundle relative dir is required")
	}
	source := filepath.Join(s.root, pluginID, bundleRelativeDir)
	if !dirExists(source) {
		return fmt.Errorf("bundle %q is not present in the object store", bundleRelativeDir)
	}
	return ensureCachedBundle(source, dstDir)
}

func (s DirObjectStore) WriteBundleFile(pluginID string, bundleRelativeDir string, rel string, bytes []byte) error {
	pluginID = strings.TrimSpace(pluginID)
	bundleRelativeDir = strings.TrimSpace(bundleRelativeDir)
	rel = strings.TrimSpace(rel)
	if pluginID == "" {
		return errors.New("plugin id is required")
	}
	if bundleRelativeDir == "" {
		return errors.New("bundle relative dir is required")
	}
	if rel == "" {
		return errors.New("bundle file path is required")
	}
	target := filepath.Join(s.root, pluginID, bundleRelativeDir, filepath.Clean(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, bytes, 0o644)
}

func (s DirObjectStore) RemovePlugin(pluginID string) error {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return errors.New("plugin id is required")
	}
	if err := os.RemoveAll(filepath.Join(s.root, pluginID)); err != nil {
		return fmt.Errorf("remove object-store plugin bundles: %w", err)
	}
	return nil
}

type ObjectStoreRepositoryOptions struct {
	Store    ObjectStore
	CacheDir string
}

type ObjectStoreRepository struct {
	store    ObjectStore
	cacheDir string
}

func NewObjectStoreRepository(opts ObjectStoreRepositoryOptions) (ObjectStoreRepository, error) {
	if opts.Store == nil {
		return ObjectStoreRepository{}, errors.New("object store is required")
	}
	cacheDir := strings.TrimSpace(opts.CacheDir)
	if cacheDir == "" {
		return ObjectStoreRepository{}, errors.New("bundle cache dir is required")
	}
	return ObjectStoreRepository{
		store:    opts.Store,
		cacheDir: cacheDir,
	}, nil
}

func (ObjectStoreRepository) ListPluginRoots(root string) ([]PluginRoot, error) {
	return LocalRepository{}.ListPluginRoots(root)
}

func (ObjectStoreRepository) ReadState(root string) (State, error) {
	return LocalRepository{}.ReadState(root)
}

func (r ObjectStoreRepository) WriteState(root string, state State) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return errors.New("plugin root is required")
	}
	if _, err := r.ResolveBundleRelativeDir(root, state.ActiveRelativeDir); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create plugin root: %w", err)
	}

	bytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bundle state: %w", err)
	}
	bytes = append(bytes, '\n')

	path := filepath.Join(root, StateFileName)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, bytes, 0o644); err != nil {
		return fmt.Errorf("write bundle state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("activate bundle state: %w", err)
	}
	return nil
}

func (r ObjectStoreRepository) ResolveBundleDir(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", errors.New("plugin root is required")
	}
	state, err := r.ReadState(root)
	if err != nil {
		return "", err
	}
	return r.ResolveBundleRelativeDir(root, state.ActiveRelativeDir)
}

func (r ObjectStoreRepository) ResolveBundleRelativeDir(root string, rel string) (string, error) {
	validatedDir, cleanedRel, err := validatedLocalBundlePath(root, rel)
	if err != nil {
		return "", err
	}
	if dirExists(validatedDir) {
		return validatedDir, nil
	}

	artifactDir := filepath.Join(r.cacheDir, "artifacts", filepath.Base(strings.TrimSpace(root)), cleanedRel)
	if err := r.store.MaterializeBundle(filepath.Base(strings.TrimSpace(root)), cleanedRel, artifactDir); err != nil {
		return "", err
	}
	return artifactDir, nil
}

func (r ObjectStoreRepository) ResolveActiveDir(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", errors.New("plugin root is required")
	}
	state, err := r.ReadState(root)
	if err != nil {
		return "", err
	}
	sourceDir, err := r.ResolveBundleRelativeDir(root, state.ActiveRelativeDir)
	if err != nil {
		return "", err
	}
	_, cleanedRel, err := validatedLocalBundlePath(root, state.ActiveRelativeDir)
	if err != nil {
		return "", err
	}
	activeDir := filepath.Join(r.cacheDir, "active", filepath.Base(root), cleanedRel)
	if err := ensureCachedBundle(sourceDir, activeDir); err != nil {
		return "", err
	}
	return activeDir, nil
}

func (r ObjectStoreRepository) resolveLoadDir(root string, cleanedRel string, sourceDir string) (string, error) {
	activeDir := filepath.Join(r.cacheDir, "active", filepath.Base(root), cleanedRel)
	if err := ensureCachedBundle(sourceDir, activeDir); err != nil {
		return "", err
	}
	return activeDir, nil
}

func (r ObjectStoreRepository) MaterializeBundle(srcDir string, root string, revision string) (MaterializedBundle, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return MaterializedBundle{}, errors.New("plugin root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return MaterializedBundle{}, fmt.Errorf("create plugin root: %w", err)
	}
	tmpRoot, err := os.MkdirTemp("", "mamacord-bundle-objectstore.")
	if err != nil {
		return MaterializedBundle{}, err
	}
	defer func() { _ = os.RemoveAll(tmpRoot) }()

	tmpDir := filepath.Join(tmpRoot, "bundle")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return MaterializedBundle{}, err
	}
	if err := copyDirSafe(srcDir, tmpDir); err != nil {
		return MaterializedBundle{}, err
	}
	hash, err := HashDir(tmpDir)
	if err != nil {
		return MaterializedBundle{}, err
	}
	hashB64 := base64.StdEncoding.EncodeToString(hash[:])
	bundleName := bundleDirName(revision, hash)
	bundleRelativeDir := filepath.Join("bundles", bundleName)
	if err := r.store.PutBundle(filepath.Base(root), bundleRelativeDir, tmpDir, hashB64); err != nil {
		return MaterializedBundle{}, err
	}
	if err := r.WriteState(root, State{
		ActiveRelativeDir: bundleRelativeDir,
		Revision:          strings.TrimSpace(revision),
		HashB64:           hashB64,
	}); err != nil {
		return MaterializedBundle{}, err
	}
	bundleDir, err := r.ResolveBundleDir(root)
	if err != nil {
		return MaterializedBundle{}, err
	}
	activeDir, err := r.ResolveActiveDir(root)
	if err != nil {
		return MaterializedBundle{}, err
	}
	return MaterializedBundle{
		RootDir:           root,
		BundleDir:         bundleDir,
		ActiveDir:         activeDir,
		BundleRelativeDir: bundleRelativeDir,
		HashB64:           hashB64,
	}, nil
}

func (r ObjectStoreRepository) WriteBundleSignature(root string, bytes []byte) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", errors.New("plugin root is required")
	}
	state, err := r.ReadState(root)
	if err != nil {
		return "", err
	}
	localBundleDir, cleanedRel, err := validatedLocalBundlePath(root, state.ActiveRelativeDir)
	if err != nil {
		return "", err
	}
	targetName := filepath.Join("signature.json")
	if dirExists(localBundleDir) {
		target := filepath.Join(localBundleDir, targetName)
		if err := os.WriteFile(target, bytes, 0o644); err != nil {
			return "", err
		}
		activeDir, activeErr := r.ResolveActiveDir(root)
		if activeErr == nil && activeDir != localBundleDir {
			if err := os.WriteFile(filepath.Join(activeDir, targetName), bytes, 0o644); err != nil {
				return "", err
			}
		}
		return target, nil
	}

	if err := r.store.WriteBundleFile(filepath.Base(root), cleanedRel, targetName, bytes); err != nil {
		return "", err
	}
	bundleDir, err := r.ResolveBundleDir(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(bundleDir, targetName)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, bytes, 0o644); err != nil {
		return "", err
	}
	activeDir, err := r.ResolveActiveDir(root)
	if err == nil && activeDir != bundleDir {
		if err := os.WriteFile(filepath.Join(activeDir, targetName), bytes, 0o644); err != nil {
			return "", err
		}
	}
	return target, nil
}

func (r ObjectStoreRepository) BundleModified(root string, installedHashB64 string) (bool, error) {
	bundleDir, err := r.ResolveBundleDir(root)
	if err != nil {
		return false, err
	}
	return DirModified(bundleDir, installedHashB64)
}

func (r ObjectStoreRepository) RemovePluginRoot(root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return errors.New("plugin root is required")
	}
	pluginID := filepath.Base(root)
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("remove plugin root: %w", err)
	}
	if err := r.store.RemovePlugin(pluginID); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(r.cacheDir, "artifacts", pluginID)); err != nil {
		return fmt.Errorf("remove artifact cache root: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(r.cacheDir, "active", pluginID)); err != nil {
		return fmt.Errorf("remove active cache root: %w", err)
	}
	return nil
}
