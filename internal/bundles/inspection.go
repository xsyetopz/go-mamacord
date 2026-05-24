package bundles

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Inspection struct {
	RootDir           string
	BundleRelativeDir string
	BundleDir         string
	LoadDir           string
	ManifestBytes     []byte
	HasSignatureFile  bool
}

func InspectPreferredOrActiveBundle(repo Repository, root string, preferredRelativeDir string) (Inspection, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Inspection{}, errors.New("plugin root is required")
	}
	if repo == nil {
		repo = NewLocalRepository()
	}

	if rel := strings.TrimSpace(preferredRelativeDir); rel != "" {
		if inspection, err := InspectBundle(repo, root, rel); err == nil {
			return inspection, nil
		}
	}

	state, err := repo.ReadState(root)
	if err != nil {
		return Inspection{}, err
	}
	return InspectBundle(repo, root, state.ActiveRelativeDir)
}

func InspectBundle(repo Repository, root string, bundleRelativeDir string) (Inspection, error) {
	root = strings.TrimSpace(root)
	bundleRelativeDir = strings.TrimSpace(bundleRelativeDir)
	if root == "" {
		return Inspection{}, errors.New("plugin root is required")
	}
	if bundleRelativeDir == "" {
		return Inspection{}, errors.New("bundle relative dir is required")
	}
	if repo == nil {
		repo = NewLocalRepository()
	}

	bundleDir, err := repo.ResolveBundleRelativeDir(root, bundleRelativeDir)
	if err != nil {
		return Inspection{}, err
	}
	_, cleanedRel, err := validatedLocalBundlePath(root, bundleRelativeDir)
	if err != nil {
		return Inspection{}, err
	}
	loadDir, err := resolveLoadDir(repo, root, cleanedRel, bundleDir)
	if err != nil {
		return Inspection{}, err
	}
	manifestBytes, err := os.ReadFile(filepath.Join(loadDir, "plugin.json"))
	if err != nil {
		return Inspection{}, fmt.Errorf("read manifest: %w", err)
	}

	return Inspection{
		RootDir:           root,
		BundleRelativeDir: cleanedRel,
		BundleDir:         bundleDir,
		LoadDir:           loadDir,
		ManifestBytes:     manifestBytes,
		HasSignatureFile:  fileExists(filepath.Join(bundleDir, "signature.json")) || fileExists(filepath.Join(loadDir, "signature.json")),
	}, nil
}

func resolveLoadDir(repo Repository, root string, cleanedRel string, sourceDir string) (string, error) {
	root = strings.TrimSpace(root)
	switch typed := repo.(type) {
	case LocalRepository:
		return sourceDir, nil
	case *LocalRepository:
		return sourceDir, nil
	case CachedRepository:
		return typed.resolveLoadDir(root, cleanedRel, sourceDir)
	case *CachedRepository:
		return typed.resolveLoadDir(root, cleanedRel, sourceDir)
	case ObjectStoreRepository:
		return typed.resolveLoadDir(root, cleanedRel, sourceDir)
	case *ObjectStoreRepository:
		return typed.resolveLoadDir(root, cleanedRel, sourceDir)
	default:
		state, err := repo.ReadState(root)
		if err != nil {
			return sourceDir, nil
		}
		if filepath.Clean(strings.TrimSpace(state.ActiveRelativeDir)) != filepath.Clean(cleanedRel) {
			return sourceDir, nil
		}
		activeDir, err := repo.ResolveActiveDir(root)
		if err != nil {
			return "", err
		}
		return activeDir, nil
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
