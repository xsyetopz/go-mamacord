-- migrate:kind=normal
ALTER TABLE plugin_installs ADD COLUMN IF NOT EXISTS bundle_relative_dir TEXT;
