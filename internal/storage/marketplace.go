package storage

import (
	"context"
	"time"
)

type TrustedSigner struct {
	KeyID        string
	PublicKeyB64 string
	AddedAt      time.Time
}

type TrustedSignerStore interface {
	ListTrustedSigners(ctx context.Context) ([]TrustedSigner, error)
	PutTrustedSigner(ctx context.Context, signer TrustedSigner) error
	DeleteTrustedSigner(ctx context.Context, keyID string) error
}

type MarketplaceSource struct {
	SourceID    string
	Kind        string
	GitURL      string
	GitRef      string
	GitSubdir   string
	TokenEnvVar string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MarketplaceSourceStore interface {
	GetMarketplaceSource(ctx context.Context, sourceID string) (MarketplaceSource, bool, error)
	ListMarketplaceSources(ctx context.Context) ([]MarketplaceSource, error)
	PutMarketplaceSource(ctx context.Context, source MarketplaceSource) error
	DeleteMarketplaceSource(ctx context.Context, sourceID string) error
}

type MarketplaceSourceSync struct {
	SourceID     string
	LastSyncedAt *time.Time
	LastRevision string
	LastError    string
}

type MarketplaceSourceSyncStore interface {
	GetMarketplaceSourceSync(ctx context.Context, sourceID string) (MarketplaceSourceSync, bool, error)
	ListMarketplaceSourceSyncs(ctx context.Context) ([]MarketplaceSourceSync, error)
	PutMarketplaceSourceSync(ctx context.Context, sync MarketplaceSourceSync) error
	DeleteMarketplaceSourceSync(ctx context.Context, sourceID string) error
}

type PluginInstall struct {
	PluginID string
	PluginInstallSource
	PluginInstallArtifact
	InstalledAt time.Time
	InstalledBy *uint64
}

type PluginInstallSource struct {
	InstallKind string
	SourceID    string
	GitURL      string
	GitRef      string
	GitRevision string
	SourcePath  string
}

type PluginInstallArtifact struct {
	BundleRelativeDir string
	InstalledHashB64  string
}

type PluginInstallStore interface {
	GetPluginInstall(ctx context.Context, pluginID string) (PluginInstall, bool, error)
	ListPluginInstalls(ctx context.Context) ([]PluginInstall, error)
	PutPluginInstall(ctx context.Context, install PluginInstall) error
	DeletePluginInstall(ctx context.Context, pluginID string) error
}

type TrustedVendor struct {
	VendorID   string
	Name       string
	WebsiteURL string
	SupportURL string
	AddedAt    time.Time
	UpdatedAt  time.Time
}

type TrustedVendorStore interface {
	GetTrustedVendor(ctx context.Context, vendorID string) (TrustedVendor, bool, error)
	ListTrustedVendors(ctx context.Context) ([]TrustedVendor, error)
	PutTrustedVendor(ctx context.Context, vendor TrustedVendor) error
	DeleteTrustedVendor(ctx context.Context, vendorID string) error
}

type TrustedVendorKey struct {
	VendorID string
	KeyID    string
}

type TrustedVendorKeyStore interface {
	ListTrustedVendorKeys(ctx context.Context, vendorID string) ([]TrustedVendorKey, error)
	ReplaceTrustedVendorKeys(ctx context.Context, vendorID string, keys []TrustedVendorKey) error
}
