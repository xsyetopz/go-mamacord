package bundles

import (
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/config"
)

func Open(cfg config.Config) (Repository, error) {
	switch cfg.Bundles.Backend {
	case "", config.BundleBackendLocal:
		return NewLocalRepository(), nil
	case config.BundleBackendCached:
		repo, err := NewCachedRepository(CachedRepositoryOptions{
			StoreDir: cfg.Bundles.StoreDir,
			CacheDir: cfg.Bundles.CacheDir,
		})
		if err != nil {
			return nil, err
		}
		return repo, nil
	case config.BundleBackendObjectStore:
		store, err := NewDirObjectStore(cfg.Bundles.StoreDir)
		if err != nil {
			return nil, err
		}
		repo, err := NewObjectStoreRepository(ObjectStoreRepositoryOptions{
			Store:    store,
			CacheDir: cfg.Bundles.CacheDir,
		})
		if err != nil {
			return nil, err
		}
		return repo, nil
	default:
		return nil, fmt.Errorf("unsupported bundle backend %q", cfg.Bundles.Backend)
	}
}
