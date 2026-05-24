package marketplace

import "time"

type SignatureState string

const (
	SignatureStateUnsigned  SignatureState = "unsigned"
	SignatureStateTrusted   SignatureState = "trusted"
	SignatureStateUntrusted SignatureState = "untrusted"
	SignatureStateInvalid   SignatureState = "invalid"
)

type ProvenanceKind string

const (
	ProvenanceKindBundled     ProvenanceKind = "bundled"
	ProvenanceKindManual      ProvenanceKind = "manual"
	ProvenanceKindMarketplace ProvenanceKind = "marketplace"
)

type Source struct {
	SourceID     string     `json:"source_id"`
	Kind         string     `json:"kind"`
	GitURL       string     `json:"git_url"`
	GitRef       string     `json:"git_ref,omitempty"`
	GitSubdir    string     `json:"git_subdir,omitempty"`
	TokenEnvVar  string     `json:"token_env_var,omitempty"`
	Enabled      bool       `json:"enabled"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	LastRevision string     `json:"last_revision,omitempty"`
	LastError    string     `json:"last_error,omitempty"`
}

type SourceUpsert struct {
	SourceID    string `json:"source_id"`
	Kind        string `json:"kind"`
	GitURL      string `json:"git_url"`
	GitRef      string `json:"git_ref,omitempty"`
	GitSubdir   string `json:"git_subdir,omitempty"`
	TokenEnvVar string `json:"token_env_var,omitempty"`
	Enabled     bool   `json:"enabled"`
}

type SyncResult struct {
	SourceID     string     `json:"source_id"`
	Revision     string     `json:"revision"`
	SyncedAt     time.Time  `json:"synced_at"`
	LastError    string     `json:"last_error,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

type SearchQuery struct {
	SourceID string `json:"source_id,omitempty"`
	Term     string `json:"term,omitempty"`
	Refresh  bool   `json:"refresh,omitempty"`
}

type PluginCandidate struct {
	SourceID       string         `json:"source_id"`
	PluginID       string         `json:"plugin_id"`
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	SourcePath     string         `json:"source_path"`
	GitRevision    string         `json:"git_revision"`
	Commands       []string       `json:"commands,omitempty"`
	SignatureState SignatureState `json:"signature_state"`
	SignerKeyID    string         `json:"signer_key_id,omitempty"`
	SyncError      string         `json:"sync_error,omitempty"`
}

type InstallRequest struct {
	SourceID string  `json:"source_id"`
	PluginID string  `json:"plugin_id"`
	ActorID  *uint64 `json:"actor_id,omitempty"`
	Force    bool    `json:"force,omitempty"`
}

type InstallResult struct {
	PluginID          string         `json:"plugin_id"`
	SourceID          string         `json:"source_id"`
	PluginRoot        string         `json:"plugin_root"`
	BundleRelativeDir string         `json:"bundle_relative_dir,omitempty"`
	GitRevision       string         `json:"git_revision"`
	SignatureState    SignatureState `json:"signature_state"`
	Enabled           bool           `json:"enabled"`
	LocalModified     bool           `json:"local_modified"`
}

type UpdateRequest struct {
	PluginID string  `json:"plugin_id"`
	ActorID  *uint64 `json:"actor_id,omitempty"`
	Force    bool    `json:"force,omitempty"`
}

type UpdateResult struct {
	PluginID          string         `json:"plugin_id"`
	SourceID          string         `json:"source_id"`
	PluginRoot        string         `json:"plugin_root"`
	BundleRelativeDir string         `json:"bundle_relative_dir,omitempty"`
	GitRevision       string         `json:"git_revision"`
	SignatureState    SignatureState `json:"signature_state"`
	Forced            bool           `json:"forced"`
}

type UninstallRequest struct {
	PluginID string `json:"plugin_id"`
}

type TrustSignerRequest struct {
	KeyID        string `json:"key_id"`
	PublicKeyB64 string `json:"public_key_b64"`
	VendorID     string `json:"vendor_id,omitempty"`
}

type TrustVendorRequest struct {
	VendorID        string `json:"vendor_id"`
	Name            string `json:"name"`
	WebsiteURL      string `json:"website_url,omitempty"`
	SupportURL      string `json:"support_url,omitempty"`
	TrustedKeysPath string `json:"trusted_keys_path,omitempty"`
	SourceID        string `json:"source_id,omitempty"`
}

type TrustVendorResult struct {
	VendorID string   `json:"vendor_id"`
	KeyIDs   []string `json:"key_ids"`
}
