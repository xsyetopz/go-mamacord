package bundles

import (
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/config"
)

func Open(cfg config.Config) (Repository, error) {
	switch cfg.BundleBackend {
	case "", config.BundleBackendLocal:
		return NewLocalRepository(), nil
	case config.BundleBackendCached:
		repo, err := NewCachedRepository(CachedRepositoryOptions{
			StoreDir: cfg.BundleStoreDir,
			CacheDir: cfg.BundleCacheDir,
		})
		if err != nil {
			return nil, err
		}
		return repo, nil
	case config.BundleBackendObjectStore:
		store, err := NewDirObjectStore(cfg.BundleStoreDir)
		if err != nil {
			return nil, err
		}
		repo, err := NewObjectStoreRepository(ObjectStoreRepositoryOptions{
			Store:    store,
			CacheDir: cfg.BundleCacheDir,
		})
		if err != nil {
			return nil, err
		}
		return repo, nil
	default:
		return nil, fmt.Errorf("unsupported bundle backend %q", cfg.BundleBackend)
	}
}
