package bundles

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const StateFileName = ".mamacord-bundle.json"

type State struct {
	ActiveRelativeDir string `json:"active_relative_dir"`
	Revision          string `json:"revision,omitempty"`
	HashB64           string `json:"hash_b64,omitempty"`
}

type PluginRoot struct {
	Name string
	Dir  string
}

type MaterializedBundle struct {
	RootDir           string
	BundleDir         string
	ActiveDir         string
	BundleRelativeDir string
	HashB64           string
}

type Repository interface {
	ListPluginRoots(root string) ([]PluginRoot, error)
	ReadState(root string) (State, error)
	WriteState(root string, state State) error
	ResolveBundleDir(root string) (string, error)
	ResolveBundleRelativeDir(root string, rel string) (string, error)
	ResolveActiveDir(root string) (string, error)
	BundleModified(root string, installedHashB64 string) (bool, error)
	WriteBundleSignature(root string, bytes []byte) (string, error)
	MaterializeBundle(srcDir string, root string, revision string) (MaterializedBundle, error)
	RemovePluginRoot(root string) error
}

type LocalRepository struct{}

func NewLocalRepository() LocalRepository {
	return LocalRepository{}
}

func (LocalRepository) ListPluginRoots(root string) ([]PluginRoot, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("plugins root is required")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := make([]PluginRoot, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		out = append(out, PluginRoot{
			Name: entry.Name(),
			Dir:  filepath.Join(root, entry.Name()),
		})
	}
	return out, nil
}

func (LocalRepository) ReadState(root string) (State, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return State{}, errors.New("plugin root is required")
	}
	bytes, err := os.ReadFile(filepath.Join(root, StateFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, fmt.Errorf("plugin root %q is missing %s", root, StateFileName)
		}
		return State{}, fmt.Errorf("read bundle state: %w", err)
	}

	var state State
	if err := json.Unmarshal(bytes, &state); err != nil {
		return State{}, fmt.Errorf("parse bundle state: %w", err)
	}
	return state, nil
}

func (r LocalRepository) WriteState(root string, state State) error {
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

func (r LocalRepository) ResolveActiveDir(root string) (string, error) {
	return r.ResolveBundleDir(root)
}

func (r LocalRepository) ResolveBundleDir(root string) (string, error) {
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

func (LocalRepository) ResolveBundleRelativeDir(root string, rel string) (string, error) {
	root = strings.TrimSpace(root)
	rel = strings.TrimSpace(rel)
	if root == "" {
		return "", errors.New("plugin root is required")
	}
	if rel == "" {
		return "", errors.New("bundle active_relative_dir is required")
	}
	if filepath.IsAbs(rel) {
		return "", errors.New("bundle active_relative_dir must stay within the plugin root")
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." {
		return "", errors.New("bundle active_relative_dir must stay within the plugin root")
	}
	resolved := filepath.Join(root, clean)
	relative, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", fmt.Errorf("resolve bundle dir: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("bundle active_relative_dir must stay within the plugin root")
	}
	return resolved, nil
}

func (r LocalRepository) MaterializeBundle(srcDir string, root string, revision string) (MaterializedBundle, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return MaterializedBundle{}, errors.New("plugin root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return MaterializedBundle{}, err
	}
	tmpRoot := filepath.Join(root, ".tmp")
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
	bundleDir := filepath.Join(root, bundleRelativeDir)
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
	return MaterializedBundle{
		RootDir:           root,
		BundleDir:         bundleDir,
		ActiveDir:         bundleDir,
		BundleRelativeDir: bundleRelativeDir,
		HashB64:           hashB64,
	}, nil
}

func (r LocalRepository) WriteBundleSignature(root string, bytes []byte) (string, error) {
	bundleDir, err := r.ResolveBundleDir(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(bundleDir, "signature.json")
	if err := os.WriteFile(target, bytes, 0o644); err != nil {
		return "", err
	}
	return target, nil
}

func (r LocalRepository) BundleModified(root string, installedHashB64 string) (bool, error) {
	bundleDir, err := r.ResolveBundleDir(root)
	if err != nil {
		return false, err
	}
	return DirModified(bundleDir, installedHashB64)
}

func (LocalRepository) RemovePluginRoot(root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return errors.New("plugin root is required")
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("remove plugin root: %w", err)
	}
	return nil
}

func HashDir(dir string) ([32]byte, error) {
	paths, err := listFiles(dir)
	if err != nil {
		return [32]byte{}, err
	}

	h := sha256.New()
	for _, rel := range paths {
		full := filepath.Join(dir, rel)

		b, readErr := os.ReadFile(full)
		if readErr != nil {
			return [32]byte{}, fmt.Errorf("read %q: %w", rel, readErr)
		}

		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(b)
		_, _ = h.Write([]byte{0})
	}

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

func DirModified(dir string, installedHashB64 string) (bool, error) {
	hash, err := HashDir(dir)
	if err != nil {
		return false, err
	}
	return base64.StdEncoding.EncodeToString(hash[:]) != strings.TrimSpace(installedHashB64), nil
}

func listFiles(dir string) ([]string, error) {
	var out []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		rel = filepath.ToSlash(rel)
		if rel == "signature.json" {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %q: %w", dir, err)
	}

	sort.Strings(out)
	return out, nil
}

func bundleDirName(revision string, hash [32]byte) string {
	parts := make([]rune, 0, len(revision))
	for _, r := range strings.TrimSpace(revision) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			parts = append(parts, r)
			continue
		}
		parts = append(parts, '-')
	}
	cleanRevision := strings.Trim(string(parts), ".-_")
	if cleanRevision != "" {
		return "git-" + cleanRevision
	}
	return "sha256-" + fmt.Sprintf("%x", hash[:12])
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
		totalBytes += info.Size()
		if totalBytes > 32<<20 {
			return fmt.Errorf("plugin bundle exceeds size limit")
		}
		if info.Size() > 8<<20 {
			return fmt.Errorf("plugin file exceeds size limit: %s", rel)
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, bytes, 0o644)
	})
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
