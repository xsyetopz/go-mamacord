package marketplacepg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
	"github.com/xsyetopz/go-mamacord/internal/storage/postgres/internal/sqlvalue"
	"strings"
	"time"
)

type marketplaceSourceStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s marketplaceSourceStore) GetMarketplaceSource(ctx context.Context, sourceID string) (storage.MarketplaceSource, bool, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return storage.MarketplaceSource{}, false, nil
	}

	const query = `
SELECT source_id, kind, git_url, git_ref, git_subdir, token_env_var, enabled, created_at, updated_at
FROM plugin_sources
WHERE source_id = $1`

	var (
		source               storage.MarketplaceSource
		gitRef, gitSubdir    sql.NullString
		tokenEnvVar          sql.NullString
		createdAt, updatedAt int64
	)

	err := s.db.QueryRowContext(ctx, query, sourceID).Scan(
		&source.SourceID,
		&source.Kind,
		&source.GitURL,
		&gitRef,
		&gitSubdir,
		&tokenEnvVar,
		&source.Enabled,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storage.MarketplaceSource{}, false, nil
		}
		return storage.MarketplaceSource{}, false, fmt.Errorf("get marketplace source: %w", err)
	}

	source.SourceID = strings.TrimSpace(source.SourceID)
	source.Kind = strings.TrimSpace(source.Kind)
	source.GitURL = strings.TrimSpace(source.GitURL)
	source.GitRef = strings.TrimSpace(gitRef.String)
	source.GitSubdir = strings.TrimSpace(gitSubdir.String)
	source.TokenEnvVar = strings.TrimSpace(tokenEnvVar.String)
	source.CreatedAt = time.Unix(createdAt, 0).UTC()
	source.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return source, true, nil
}

func (s marketplaceSourceStore) ListMarketplaceSources(ctx context.Context) ([]storage.MarketplaceSource, error) {
	const query = `
SELECT source_id, kind, git_url, git_ref, git_subdir, token_env_var, enabled, created_at, updated_at
FROM plugin_sources
ORDER BY source_id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list marketplace sources: %w", err)
	}
	defer rows.Close()

	var out []storage.MarketplaceSource
	for rows.Next() {
		var (
			source               storage.MarketplaceSource
			gitRef, gitSubdir    sql.NullString
			tokenEnvVar          sql.NullString
			createdAt, updatedAt int64
		)
		if err := rows.Scan(
			&source.SourceID,
			&source.Kind,
			&source.GitURL,
			&gitRef,
			&gitSubdir,
			&tokenEnvVar,
			&source.Enabled,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan marketplace source: %w", err)
		}
		source.SourceID = strings.TrimSpace(source.SourceID)
		source.Kind = strings.TrimSpace(source.Kind)
		source.GitURL = strings.TrimSpace(source.GitURL)
		source.GitRef = strings.TrimSpace(gitRef.String)
		source.GitSubdir = strings.TrimSpace(gitSubdir.String)
		source.TokenEnvVar = strings.TrimSpace(tokenEnvVar.String)
		source.CreatedAt = time.Unix(createdAt, 0).UTC()
		source.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		out = append(out, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate marketplace sources: %w", err)
	}
	return out, nil
}

func (s marketplaceSourceStore) PutMarketplaceSource(ctx context.Context, source storage.MarketplaceSource) error {
	sourceID := strings.TrimSpace(source.SourceID)
	if sourceID == "" {
		return errors.New("source_id is required")
	}
	kind := strings.TrimSpace(source.Kind)
	if kind == "" {
		return errors.New("kind is required")
	}
	gitURL := strings.TrimSpace(source.GitURL)
	if gitURL == "" {
		return errors.New("git_url is required")
	}
	createdAt := source.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.now().UTC()
	}
	updatedAt := source.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = s.now().UTC()
	}

	const query = `
INSERT INTO plugin_sources(source_id, kind, git_url, git_ref, git_subdir, token_env_var, enabled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT(source_id) DO UPDATE SET
	kind = excluded.kind,
	git_url = excluded.git_url,
	git_ref = excluded.git_ref,
	git_subdir = excluded.git_subdir,
	token_env_var = excluded.token_env_var,
	enabled = excluded.enabled,
	updated_at = excluded.updated_at`

	if _, err := s.db.ExecContext(
		ctx,
		query,
		sourceID,
		kind,
		gitURL,
		sqlvalue.NullIfBlank(source.GitRef),
		sqlvalue.NullIfBlank(source.GitSubdir),
		sqlvalue.NullIfBlank(source.TokenEnvVar),
		source.Enabled,
		createdAt.Unix(),
		updatedAt.Unix(),
	); err != nil {
		return fmt.Errorf("put marketplace source: %w", err)
	}
	return nil
}

func (s marketplaceSourceStore) DeleteMarketplaceSource(ctx context.Context, sourceID string) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return errors.New("source_id is required")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM plugin_sources WHERE source_id = $1`, sourceID); err != nil {
		return fmt.Errorf("delete marketplace source: %w", err)
	}
	return nil
}

type marketplaceSourceSyncStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s marketplaceSourceSyncStore) GetMarketplaceSourceSync(ctx context.Context, sourceID string) (storage.MarketplaceSourceSync, bool, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return storage.MarketplaceSourceSync{}, false, nil
	}

	const query = `
SELECT source_id, last_synced_at, last_revision, last_error
FROM plugin_source_sync
WHERE source_id = $1`

	var (
		out          storage.MarketplaceSourceSync
		lastSyncedAt sql.NullInt64
		lastRevision sql.NullString
		lastError    sql.NullString
	)

	if err := s.db.QueryRowContext(ctx, query, sourceID).Scan(&out.SourceID, &lastSyncedAt, &lastRevision, &lastError); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storage.MarketplaceSourceSync{}, false, nil
		}
		return storage.MarketplaceSourceSync{}, false, fmt.Errorf("get marketplace source sync: %w", err)
	}
	if lastSyncedAt.Valid {
		ts := time.Unix(lastSyncedAt.Int64, 0).UTC()
		out.LastSyncedAt = &ts
	}
	out.SourceID = strings.TrimSpace(out.SourceID)
	out.LastRevision = strings.TrimSpace(lastRevision.String)
	out.LastError = strings.TrimSpace(lastError.String)
	return out, true, nil
}

func (s marketplaceSourceSyncStore) ListMarketplaceSourceSyncs(ctx context.Context) ([]storage.MarketplaceSourceSync, error) {
	const query = `
SELECT source_id, last_synced_at, last_revision, last_error
FROM plugin_source_sync
ORDER BY source_id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list marketplace source syncs: %w", err)
	}
	defer rows.Close()

	var out []storage.MarketplaceSourceSync
	for rows.Next() {
		var (
			item         storage.MarketplaceSourceSync
			lastSyncedAt sql.NullInt64
			lastRevision sql.NullString
			lastError    sql.NullString
		)
		if err := rows.Scan(&item.SourceID, &lastSyncedAt, &lastRevision, &lastError); err != nil {
			return nil, fmt.Errorf("scan marketplace source sync: %w", err)
		}
		if lastSyncedAt.Valid {
			ts := time.Unix(lastSyncedAt.Int64, 0).UTC()
			item.LastSyncedAt = &ts
		}
		item.SourceID = strings.TrimSpace(item.SourceID)
		item.LastRevision = strings.TrimSpace(lastRevision.String)
		item.LastError = strings.TrimSpace(lastError.String)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate marketplace source syncs: %w", err)
	}
	return out, nil
}

func (s marketplaceSourceSyncStore) PutMarketplaceSourceSync(ctx context.Context, sync storage.MarketplaceSourceSync) error {
	sourceID := strings.TrimSpace(sync.SourceID)
	if sourceID == "" {
		return errors.New("source_id is required")
	}

	var syncedAt any
	if sync.LastSyncedAt != nil && !sync.LastSyncedAt.IsZero() {
		syncedAt = sync.LastSyncedAt.UTC().Unix()
	}

	const query = `
INSERT INTO plugin_source_sync(source_id, last_synced_at, last_revision, last_error)
VALUES ($1, $2, $3, $4)
ON CONFLICT(source_id) DO UPDATE SET
	last_synced_at = excluded.last_synced_at,
	last_revision = excluded.last_revision,
	last_error = excluded.last_error`

	if _, err := s.db.ExecContext(ctx, query, sourceID, syncedAt, sqlvalue.NullIfBlank(sync.LastRevision), sqlvalue.NullIfBlank(sync.LastError)); err != nil {
		return fmt.Errorf("put marketplace source sync: %w", err)
	}
	return nil
}

func (s marketplaceSourceSyncStore) DeleteMarketplaceSourceSync(ctx context.Context, sourceID string) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return errors.New("source_id is required")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM plugin_source_sync WHERE source_id = $1`, sourceID); err != nil {
		return fmt.Errorf("delete marketplace source sync: %w", err)
	}
	return nil
}

type pluginInstallStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s pluginInstallStore) GetPluginInstall(ctx context.Context, pluginID string) (storage.PluginInstall, bool, error) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return storage.PluginInstall{}, false, nil
	}

	const query = `
SELECT plugin_id, install_kind, source_id, git_url, git_ref, git_revision, source_path, bundle_relative_dir, installed_at, installed_by, installed_hash_b64
FROM plugin_installs
WHERE plugin_id = $1`

	var (
		item        storage.PluginInstall
		sourceID    sql.NullString
		gitRef      sql.NullString
		bundleRel   sql.NullString
		installedBy sql.NullInt64
		installedAt int64
	)

	if err := s.db.QueryRowContext(ctx, query, pluginID).Scan(
		&item.PluginID,
		&item.InstallKind,
		&sourceID,
		&item.GitURL,
		&gitRef,
		&item.GitRevision,
		&item.SourcePath,
		&bundleRel,
		&installedAt,
		&installedBy,
		&item.InstalledHashB64,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storage.PluginInstall{}, false, nil
		}
		return storage.PluginInstall{}, false, fmt.Errorf("get plugin install: %w", err)
	}
	item.PluginID = strings.TrimSpace(item.PluginID)
	item.InstallKind = strings.TrimSpace(item.InstallKind)
	item.SourceID = strings.TrimSpace(sourceID.String)
	item.GitURL = strings.TrimSpace(item.GitURL)
	item.GitRef = strings.TrimSpace(gitRef.String)
	item.GitRevision = strings.TrimSpace(item.GitRevision)
	item.SourcePath = strings.TrimSpace(item.SourcePath)
	item.BundleRelativeDir = strings.TrimSpace(bundleRel.String)
	item.InstalledHashB64 = strings.TrimSpace(item.InstalledHashB64)
	item.InstalledAt = time.Unix(installedAt, 0).UTC()
	if installedBy.Valid {
		v, err := sqlvalue.Int64ToUint64(installedBy.Int64, "installed_by")
		if err != nil {
			return storage.PluginInstall{}, false, err
		}
		item.InstalledBy = &v
	}
	return item, true, nil
}

func (s pluginInstallStore) ListPluginInstalls(ctx context.Context) ([]storage.PluginInstall, error) {
	const query = `
SELECT plugin_id, install_kind, source_id, git_url, git_ref, git_revision, source_path, bundle_relative_dir, installed_at, installed_by, installed_hash_b64
FROM plugin_installs
ORDER BY plugin_id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list plugin installs: %w", err)
	}
	defer rows.Close()

	var out []storage.PluginInstall
	for rows.Next() {
		var (
			item        storage.PluginInstall
			sourceID    sql.NullString
			gitRef      sql.NullString
			bundleRel   sql.NullString
			installedBy sql.NullInt64
			installedAt int64
		)
		if err := rows.Scan(
			&item.PluginID,
			&item.InstallKind,
			&sourceID,
			&item.GitURL,
			&gitRef,
			&item.GitRevision,
			&item.SourcePath,
			&bundleRel,
			&installedAt,
			&installedBy,
			&item.InstalledHashB64,
		); err != nil {
			return nil, fmt.Errorf("scan plugin install: %w", err)
		}
		item.PluginID = strings.TrimSpace(item.PluginID)
		item.InstallKind = strings.TrimSpace(item.InstallKind)
		item.SourceID = strings.TrimSpace(sourceID.String)
		item.GitURL = strings.TrimSpace(item.GitURL)
		item.GitRef = strings.TrimSpace(gitRef.String)
		item.GitRevision = strings.TrimSpace(item.GitRevision)
		item.SourcePath = strings.TrimSpace(item.SourcePath)
		item.BundleRelativeDir = strings.TrimSpace(bundleRel.String)
		item.InstalledHashB64 = strings.TrimSpace(item.InstalledHashB64)
		item.InstalledAt = time.Unix(installedAt, 0).UTC()
		if installedBy.Valid {
			v, err := sqlvalue.Int64ToUint64(installedBy.Int64, "installed_by")
			if err != nil {
				return nil, err
			}
			item.InstalledBy = &v
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugin installs: %w", err)
	}
	return out, nil
}

func (s pluginInstallStore) PutPluginInstall(ctx context.Context, install storage.PluginInstall) error {
	pluginID := strings.TrimSpace(install.PluginID)
	if pluginID == "" {
		return errors.New("plugin_id is required")
	}
	installKind := strings.TrimSpace(install.InstallKind)
	if installKind == "" {
		return errors.New("install_kind is required")
	}
	gitURL := strings.TrimSpace(install.GitURL)
	if gitURL == "" {
		return errors.New("git_url is required")
	}
	gitRevision := strings.TrimSpace(install.GitRevision)
	if gitRevision == "" {
		return errors.New("git_revision is required")
	}
	sourcePath := strings.TrimSpace(install.SourcePath)
	if sourcePath == "" {
		return errors.New("source_path is required")
	}
	bundleRelativeDir := strings.TrimSpace(install.BundleRelativeDir)
	if bundleRelativeDir == "" {
		return errors.New("bundle_relative_dir is required")
	}
	installedHashB64 := strings.TrimSpace(install.InstalledHashB64)
	if installedHashB64 == "" {
		return errors.New("installed_hash_b64 is required")
	}
	installedAt := install.InstalledAt
	if installedAt.IsZero() {
		installedAt = s.now().UTC()
	}

	var installedBy any
	if install.InstalledBy != nil {
		v, err := sqlvalue.Uint64ToInt64(*install.InstalledBy, "installed_by")
		if err != nil {
			return err
		}
		installedBy = v
	}

	const query = `
INSERT INTO plugin_installs(plugin_id, install_kind, source_id, git_url, git_ref, git_revision, source_path, bundle_relative_dir, installed_at, installed_by, installed_hash_b64)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT(plugin_id) DO UPDATE SET
	install_kind = excluded.install_kind,
	source_id = excluded.source_id,
	git_url = excluded.git_url,
	git_ref = excluded.git_ref,
	git_revision = excluded.git_revision,
	source_path = excluded.source_path,
	bundle_relative_dir = excluded.bundle_relative_dir,
	installed_at = excluded.installed_at,
	installed_by = excluded.installed_by,
	installed_hash_b64 = excluded.installed_hash_b64`

	if _, err := s.db.ExecContext(
		ctx,
		query,
		pluginID,
		installKind,
		sqlvalue.NullIfBlank(install.SourceID),
		gitURL,
		sqlvalue.NullIfBlank(install.GitRef),
		gitRevision,
		sourcePath,
		bundleRelativeDir,
		installedAt.Unix(),
		installedBy,
		installedHashB64,
	); err != nil {
		return fmt.Errorf("put plugin install: %w", err)
	}
	return nil
}

func (s pluginInstallStore) DeletePluginInstall(ctx context.Context, pluginID string) error {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return errors.New("plugin_id is required")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM plugin_installs WHERE plugin_id = $1`, pluginID); err != nil {
		return fmt.Errorf("delete plugin install: %w", err)
	}
	return nil
}

type trustedVendorStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s trustedVendorStore) GetTrustedVendor(ctx context.Context, vendorID string) (storage.TrustedVendor, bool, error) {
	vendorID = strings.TrimSpace(vendorID)
	if vendorID == "" {
		return storage.TrustedVendor{}, false, nil
	}

	const query = `
SELECT vendor_id, name, website_url, support_url, added_at, updated_at
FROM trusted_vendors
WHERE vendor_id = $1`

	var (
		item                   storage.TrustedVendor
		websiteURL, supportURL sql.NullString
		addedAt, updatedAt     int64
	)
	if err := s.db.QueryRowContext(ctx, query, vendorID).Scan(
		&item.VendorID,
		&item.Name,
		&websiteURL,
		&supportURL,
		&addedAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storage.TrustedVendor{}, false, nil
		}
		return storage.TrustedVendor{}, false, fmt.Errorf("get trusted vendor: %w", err)
	}
	item.VendorID = strings.TrimSpace(item.VendorID)
	item.Name = strings.TrimSpace(item.Name)
	item.WebsiteURL = strings.TrimSpace(websiteURL.String)
	item.SupportURL = strings.TrimSpace(supportURL.String)
	item.AddedAt = time.Unix(addedAt, 0).UTC()
	item.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return item, true, nil
}

func (s trustedVendorStore) ListTrustedVendors(ctx context.Context) ([]storage.TrustedVendor, error) {
	const query = `
SELECT vendor_id, name, website_url, support_url, added_at, updated_at
FROM trusted_vendors
ORDER BY vendor_id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list trusted vendors: %w", err)
	}
	defer rows.Close()

	var out []storage.TrustedVendor
	for rows.Next() {
		var (
			item                   storage.TrustedVendor
			websiteURL, supportURL sql.NullString
			addedAt, updatedAt     int64
		)
		if err := rows.Scan(&item.VendorID, &item.Name, &websiteURL, &supportURL, &addedAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan trusted vendor: %w", err)
		}
		item.VendorID = strings.TrimSpace(item.VendorID)
		item.Name = strings.TrimSpace(item.Name)
		item.WebsiteURL = strings.TrimSpace(websiteURL.String)
		item.SupportURL = strings.TrimSpace(supportURL.String)
		item.AddedAt = time.Unix(addedAt, 0).UTC()
		item.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trusted vendors: %w", err)
	}
	return out, nil
}

func (s trustedVendorStore) PutTrustedVendor(ctx context.Context, vendor storage.TrustedVendor) error {
	vendorID := strings.TrimSpace(vendor.VendorID)
	if vendorID == "" {
		return errors.New("vendor_id is required")
	}
	name := strings.TrimSpace(vendor.Name)
	if name == "" {
		return errors.New("name is required")
	}
	addedAt := vendor.AddedAt
	if addedAt.IsZero() {
		addedAt = s.now().UTC()
	}
	updatedAt := vendor.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = s.now().UTC()
	}

	const query = `
INSERT INTO trusted_vendors(vendor_id, name, website_url, support_url, added_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT(vendor_id) DO UPDATE SET
	name = excluded.name,
	website_url = excluded.website_url,
	support_url = excluded.support_url,
	updated_at = excluded.updated_at`

	if _, err := s.db.ExecContext(
		ctx,
		query,
		vendorID,
		name,
		sqlvalue.NullIfBlank(vendor.WebsiteURL),
		sqlvalue.NullIfBlank(vendor.SupportURL),
		addedAt.Unix(),
		updatedAt.Unix(),
	); err != nil {
		return fmt.Errorf("put trusted vendor: %w", err)
	}
	return nil
}

func (s trustedVendorStore) DeleteTrustedVendor(ctx context.Context, vendorID string) error {
	vendorID = strings.TrimSpace(vendorID)
	if vendorID == "" {
		return errors.New("vendor_id is required")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM trusted_vendors WHERE vendor_id = $1`, vendorID); err != nil {
		return fmt.Errorf("delete trusted vendor: %w", err)
	}
	return nil
}

type trustedVendorKeyStore struct {
	db *sql.DB
}

func (s trustedVendorKeyStore) ListTrustedVendorKeys(ctx context.Context, vendorID string) ([]storage.TrustedVendorKey, error) {
	vendorID = strings.TrimSpace(vendorID)
	if vendorID == "" {
		return nil, errors.New("vendor_id is required")
	}

	rows, err := s.db.QueryContext(ctx, `SELECT vendor_id, key_id FROM trusted_vendor_keys WHERE vendor_id = $1 ORDER BY key_id`, vendorID)
	if err != nil {
		return nil, fmt.Errorf("list trusted vendor keys: %w", err)
	}
	defer rows.Close()

	var out []storage.TrustedVendorKey
	for rows.Next() {
		var item storage.TrustedVendorKey
		if err := rows.Scan(&item.VendorID, &item.KeyID); err != nil {
			return nil, fmt.Errorf("scan trusted vendor key: %w", err)
		}
		item.VendorID = strings.TrimSpace(item.VendorID)
		item.KeyID = strings.TrimSpace(item.KeyID)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trusted vendor keys: %w", err)
	}
	return out, nil
}

func (s trustedVendorKeyStore) ReplaceTrustedVendorKeys(ctx context.Context, vendorID string, keys []storage.TrustedVendorKey) error {
	vendorID = strings.TrimSpace(vendorID)
	if vendorID == "" {
		return errors.New("vendor_id is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin trusted vendor key tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM trusted_vendor_keys WHERE vendor_id = $1`, vendorID); err != nil {
		return fmt.Errorf("delete trusted vendor keys: %w", err)
	}
	for _, key := range keys {
		keyID := strings.TrimSpace(key.KeyID)
		if keyID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO trusted_vendor_keys(vendor_id, key_id) VALUES ($1, $2)`, vendorID, keyID); err != nil {
			return fmt.Errorf("insert trusted vendor key: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit trusted vendor key tx: %w", err)
	}
	return nil
}
