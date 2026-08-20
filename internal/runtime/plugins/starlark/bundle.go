package starlark

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type SourceBundle interface {
	ReadSource(label string, maxBytes int64) ([]byte, error)
}

type DirBundle struct{ root string }

func OpenDirBundle(root string) (*DirBundle, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("bundle root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve bundle root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve bundle root symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat bundle root: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("bundle root is not a directory")
	}
	return &DirBundle{root: filepath.Clean(resolved)}, nil
}

func (bundle *DirBundle) ReadSource(label string, maxBytes int64) ([]byte, error) {
	if bundle == nil || bundle.root == "" {
		return nil, errors.New("bundle is not initialized")
	}
	if maxBytes <= 0 {
		return nil, errors.New("source byte limit must be positive")
	}
	canonical, err := CanonicalBundleLabel(label)
	if err != nil {
		return nil, err
	}
	relative, err := labelRelativePath(canonical)
	if err != nil {
		return nil, err
	}
	full := filepath.Join(bundle.root, filepath.FromSlash(relative))
	contained, err := filepath.Rel(bundle.root, full)
	if err != nil {
		return nil, fmt.Errorf("resolve bundle source: %w", err)
	}
	if contained == ".." || strings.HasPrefix(contained, ".."+string(filepath.Separator)) || filepath.IsAbs(contained) {
		return nil, errors.New("bundle source escapes root")
	}
	if err := rejectSymlinkPath(bundle.root, contained); err != nil {
		return nil, err
	}

	file, err := os.Open(full)
	if err != nil {
		return nil, fmt.Errorf("open bundle source %q: %w", canonical, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat bundle source %q: %w", canonical, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("bundle source %q is not a regular file", canonical)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read bundle source %q: %w", canonical, err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("bundle source %q exceeds %d bytes", canonical, maxBytes)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("bundle source %q is not valid UTF-8", canonical)
	}
	return data, nil
}

func CanonicalBundleLabel(raw string) (string, error) {
	if raw == APIModuleLabel {
		return "", errors.New("host API label is not a bundle source")
	}
	if strings.ContainsRune(raw, '\x00') || strings.Contains(raw, "\\") {
		return "", errors.New("bundle label contains an invalid character")
	}
	if !strings.HasPrefix(raw, "//") {
		return "", errors.New("bundle label must start with //")
	}
	body := strings.TrimPrefix(raw, "//")
	if strings.Count(body, ":") != 1 {
		return "", errors.New("bundle label must contain exactly one colon")
	}
	directory, filename, ok := strings.Cut(body, ":")
	if !ok || filename == "" {
		return "", errors.New("bundle label filename is required")
	}
	if strings.Contains(filename, "/") || filename == "." || filename == ".." || strings.HasPrefix(filename, ".") || path.Ext(filename) != ".star" {
		return "", errors.New("bundle label must name a visible .star file")
	}
	if directory != "" {
		if path.IsAbs(directory) || path.Clean(directory) != directory {
			return "", errors.New("bundle label directory is not canonical")
		}
		for _, component := range strings.Split(directory, "/") {
			if component == "" || component == "." || component == ".." || strings.HasPrefix(component, ".") {
				return "", errors.New("bundle label contains an invalid directory component")
			}
		}
	}
	return "//" + directory + ":" + filename, nil
}

func labelRelativePath(label string) (string, error) {
	canonical, err := CanonicalBundleLabel(label)
	if err != nil {
		return "", err
	}
	body := strings.TrimPrefix(canonical, "//")
	directory, filename, _ := strings.Cut(body, ":")
	if directory == "" {
		return filename, nil
	}
	return path.Join(directory, filename), nil
}

func rejectSymlinkPath(root, relative string) error {
	current := root
	for _, component := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("inspect bundle source path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("bundle source path contains symlink %q", component)
		}
	}
	return nil
}
