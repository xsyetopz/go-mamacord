package pluginhost

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"
)

func validateStarlarkBundleAuthority(bundleDir string, manifest StarlarkManifest) error {
	root, err := filepath.EvalSymlinks(bundleDir)
	if err != nil {
		return fmt.Errorf("resolve plugin bundle: %w", err)
	}
	entry := filepath.Join(root, StarlarkEntrypoint)
	if err := requireRegularContainedFile(root, entry); err != nil {
		return fmt.Errorf("entrypoint: %w", err)
	}
	actualLocales := []string{}
	localesDir := filepath.Join(root, "locales")
	entries, err := os.ReadDir(localesDir)
	if err != nil {
		return fmt.Errorf("read locales: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			actualLocales = append(actualLocales, entry.Name())
			if err := requireRegularContainedFile(root, filepath.Join(localesDir, entry.Name(), "messages.json")); err != nil {
				return fmt.Errorf("locale %q: %w", entry.Name(), err)
			}
		}
	}
	slices.Sort(actualLocales)
	expected := append([]string(nil), manifest.Locales.Supported...)
	slices.Sort(expected)
	if !slices.Equal(actualLocales, expected) {
		return fmt.Errorf("bundle locales %v do not match manifest %v", actualLocales, expected)
	}
	for _, asset := range manifest.Assets {
		if err := requireRegularContainedFile(root, filepath.Join(root, filepath.FromSlash(asset))); err != nil {
			return fmt.Errorf("asset %q: %w", asset, err)
		}
	}
	assetPaths := make(map[string]struct{}, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		assetPaths[filepath.FromSlash(asset)] = struct{}{}
	}
	fileCount, sourceBytes, assetBytes := 0, int64(0), int64(0)
	err = filepath.WalkDir(root, func(pathValue string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("bundle contains symlink %q", pathValue)
		}
		if entry.IsDir() {
			if pathValue != root {
				rel, err := filepath.Rel(root, pathValue)
				if err != nil {
					return err
				}
				for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
					if strings.HasPrefix(part, ".") {
						return fmt.Errorf("bundle directory %q is not canonical", filepath.ToSlash(rel))
					}
				}
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("bundle contains non-regular file %q", pathValue)
		}
		fileCount++
		if fileCount > 512 {
			return errors.New("bundle exceeds 512 files")
		}
		rel, err := filepath.Rel(root, pathValue)
		if err != nil {
			return err
		}
		if rel == "plugin.json" {
			if info.Size() > 64*1024 {
				return errors.New("plugin.json exceeds byte limit")
			}
			return nil
		}
		if rel == "signature.json" {
			if info.Size() > 16*1024 {
				return errors.New("signature.json exceeds byte limit")
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".lua") {
			return fmt.Errorf("bundle contains forbidden Lua source %q", pathValue)
		}
		if rel == StarlarkEntrypoint || strings.EqualFold(filepath.Ext(rel), ".star") {
			sourcePath := filepath.ToSlash(rel)
			if filepath.Ext(rel) != ".star" || strings.Contains(rel, "\\") || pathpkg.Clean(sourcePath) != sourcePath {
				return fmt.Errorf("bundle source path %q is not canonical", rel)
			}
			for _, part := range strings.Split(sourcePath, "/") {
				if strings.HasPrefix(part, ".") {
					return fmt.Errorf("bundle source path %q is not canonical", sourcePath)
				}
			}
			if info.Size() > 256*1024 {
				return fmt.Errorf("bundle source %q exceeds byte limit", rel)
			}
			sourceBytes += info.Size()
			if sourceBytes > 1024*1024 {
				return errors.New("bundle source exceeds aggregate byte limit")
			}
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 3 && parts[0] == "locales" && parts[2] == "messages.json" && slices.Contains(manifest.Locales.Supported, parts[1]) {
			if info.Size() > 512*1024 {
				return fmt.Errorf("locale %q exceeds byte limit", parts[1])
			}
			return nil
		}
		if _, ok := assetPaths[rel]; ok {
			if info.Size() > 1024*1024 {
				return fmt.Errorf("asset %q exceeds byte limit", filepath.ToSlash(rel))
			}
			assetBytes += info.Size()
			if assetBytes > 8*1024*1024 {
				return errors.New("bundle assets exceed aggregate byte limit")
			}
			return nil
		}
		return fmt.Errorf("bundle contains undeclared file %q", filepath.ToSlash(rel))
	})
	if err != nil {
		return err
	}
	return nil
}
func requireRegularContainedFile(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return errors.New("file escapes bundle")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("file is not regular")
	}
	return nil
}

func readBundleResources(bundleDir string, manifest StarlarkManifest) (map[string][]byte, error) {
	resources := make(map[string][]byte, len(manifest.Assets))
	root, err := filepath.EvalSymlinks(bundleDir)
	if err != nil {
		return nil, err
	}
	for _, asset := range manifest.Assets {
		full := filepath.Join(root, filepath.FromSlash(asset))
		if err := requireRegularContainedFile(root, full); err != nil {
			return nil, fmt.Errorf("resource %q: %w", asset, err)
		}
		file, err := os.Open(full)
		if err != nil {
			return nil, fmt.Errorf("open resource %q: %w", asset, err)
		}
		content, readErr := io.ReadAll(io.LimitReader(file, 1024*1024+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read resource %q: %w", asset, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close resource %q: %w", asset, closeErr)
		}
		if len(content) > 1024*1024 {
			return nil, fmt.Errorf("resource %q exceeds byte limit", asset)
		}
		resources[asset] = content
	}
	return resources, nil
}
