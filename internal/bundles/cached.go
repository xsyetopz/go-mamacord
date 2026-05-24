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

type CachedRepositoryOptions struct {
	StoreDir string
	CacheDir string
}

type CachedRepository struct {
	storeDir string
	cacheDir string
}

func NewCachedRepository(opts CachedRepositoryOptions) (CachedRepository, error) {
	storeDir := strings.TrimSpace(opts.StoreDir)
	cacheDir := strings.TrimSpace(opts.CacheDir)
	if storeDir == "" {
		return CachedRepository{}, errors.New("bundle store dir is required")
	}
	if cacheDir == "" {
		return CachedRepository{}, errors.New("bundle cache dir is required")
	}
	return CachedRepository{
		storeDir: storeDir,
		cacheDir: cacheDir,
	}, nil
}

func (CachedRepository) ListPluginRoots(root string) ([]PluginRoot, error) {
	return LocalRepository{}.ListPluginRoots(root)
}

func (CachedRepository) ReadState(root string) (State, error) {
	return LocalRepository{}.ReadState(root)
}

func (r CachedRepository) WriteState(root string, state State) error {
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

func (r CachedRepository) ResolveBundleDir(root string) (string, error) {
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

func (r CachedRepository) ResolveBundleRelativeDir(root string, rel string) (string, error) {
	validatedDir, cleanedRel, err := validatedLocalBundlePath(root, rel)
	if err != nil {
		return "", err
	}
	if dirExists(validatedDir) {
		return validatedDir, nil
	}

	storeDir := filepath.Join(r.storeDir, filepath.Base(strings.TrimSpace(root)), cleanedRel)
	if !dirExists(storeDir) {
		return "", fmt.Errorf("bundle %q is not present under %q or %q", cleanedRel, root, r.storeDir)
	}
	return storeDir, nil
}

func (r CachedRepository) ResolveActiveDir(root string) (string, error) {
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
	activeDir := filepath.Join(r.cacheDir, filepath.Base(root), cleanedRel)
	if err := ensureCachedBundle(sourceDir, activeDir); err != nil {
		return "", err
	}
	return activeDir, nil
}

func (r CachedRepository) resolveLoadDir(root string, cleanedRel string, sourceDir string) (string, error) {
	activeDir := filepath.Join(r.cacheDir, filepath.Base(root), cleanedRel)
	if err := ensureCachedBundle(sourceDir, activeDir); err != nil {
		return "", err
	}
	return activeDir, nil
}

func (r CachedRepository) MaterializeBundle(srcDir string, root string, revision string) (MaterializedBundle, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return MaterializedBundle{}, errors.New("plugin root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return MaterializedBundle{}, fmt.Errorf("create plugin root: %w", err)
	}
	tmpRoot := filepath.Join(r.storeDir, ".tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return MaterializedBundle{}, err
	}
	tmpDir, err := os.MkdirTemp(tmpRoot, "bundle.")
	if err != nil {
		return MaterializedBundle{}, err
	}
	if err := copyDirSafe(srcDir, tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return MaterializedBundle{}, err
	}
	hash, err := HashDir(tmpDir)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return MaterializedBundle{}, err
	}
	hashB64 := base64.StdEncoding.EncodeToString(hash[:])
	bundleName := bundleDirName(revision, hash)
	bundleRelativeDir := filepath.Join("bundles", bundleName)
	bundleDir := filepath.Join(r.storeDir, filepath.Base(root), bundleRelativeDir)
	if dirExists(bundleDir) {
		modified, err := DirModified(bundleDir, hashB64)
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return MaterializedBundle{}, err
		}
		if modified {
			_ = os.RemoveAll(tmpDir)
			return MaterializedBundle{}, fmt.Errorf("bundle %q already exists with different contents", bundleName)
		}
		_ = os.RemoveAll(tmpDir)
	} else {
		if err := os.MkdirAll(filepath.Dir(bundleDir), 0o755); err != nil {
			_ = os.RemoveAll(tmpDir)
			return MaterializedBundle{}, err
		}
		if err := os.Rename(tmpDir, bundleDir); err != nil {
			_ = os.RemoveAll(tmpDir)
			return MaterializedBundle{}, fmt.Errorf("activate bundle: %w", err)
		}
	}
	if err := r.WriteState(root, State{
		ActiveRelativeDir: bundleRelativeDir,
		Revision:          strings.TrimSpace(revision),
		HashB64:           hashB64,
	}); err != nil {
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

func (r CachedRepository) WriteBundleSignature(root string, bytes []byte) (string, error) {
	bundleDir, err := r.ResolveBundleDir(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(bundleDir, "signature.json")
	if err := os.WriteFile(target, bytes, 0o644); err != nil {
		return "", err
	}
	activeDir, err := r.ResolveActiveDir(root)
	if err == nil && activeDir != bundleDir {
		if err := os.WriteFile(filepath.Join(activeDir, "signature.json"), bytes, 0o644); err != nil {
			return "", err
		}
	}
	return target, nil
}

func (r CachedRepository) BundleModified(root string, installedHashB64 string) (bool, error) {
	bundleDir, err := r.ResolveBundleDir(root)
	if err != nil {
		return false, err
	}
	return DirModified(bundleDir, installedHashB64)
}

func (r CachedRepository) RemovePluginRoot(root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return errors.New("plugin root is required")
	}
	pluginID := filepath.Base(root)
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("remove plugin root: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(r.storeDir, pluginID)); err != nil {
		return fmt.Errorf("remove stored bundle root: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(r.cacheDir, pluginID)); err != nil {
		return fmt.Errorf("remove bundle cache root: %w", err)
	}
	return nil
}

func validatedLocalBundlePath(root string, rel string) (string, string, error) {
	resolved, err := LocalRepository{}.ResolveBundleRelativeDir(root, rel)
	if err != nil {
		return "", "", err
	}
	cleanedRel, err := filepath.Rel(strings.TrimSpace(root), resolved)
	if err != nil {
		return "", "", fmt.Errorf("resolve bundle dir: %w", err)
	}
	return resolved, cleanedRel, nil
}

func ensureCachedBundle(sourceDir, activeDir string) error {
	if dirExists(activeDir) {
		sourceHash, err := HashDir(sourceDir)
		if err != nil {
			return err
		}
		activeHash, err := HashDir(activeDir)
		if err == nil && activeHash == sourceHash {
			return nil
		}
		if removeErr := os.RemoveAll(activeDir); removeErr != nil {
			return fmt.Errorf("reset active bundle cache: %w", removeErr)
		}
	}

	tmpRoot := filepath.Join(filepath.Dir(activeDir), ".tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(tmpRoot, "bundle.")
	if err != nil {
		return err
	}
	if err := copyDirSafe(sourceDir, tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(activeDir), 0o755); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	if err := os.Rename(tmpDir, activeDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("activate cached bundle: %w", err)
	}
	return nil
}
