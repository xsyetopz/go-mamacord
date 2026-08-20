package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/permissions"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

func validStarlarkManifestJSON(t *testing.T) []byte {
	t.Helper()
	manifest := StarlarkManifest{Schema: StarlarkManifestSchema, ID: "fun", Name: "Fun", Version: "1.2.3", Entrypoint: StarlarkEntrypoint, Permissions: StarlarkManifestPermissions{Network: StarlarkManifestNetworkPermissions{HTTP: true, Hosts: []string{"kawaii.red"}}}, Locales: StarlarkManifestLocales{Default: "en-US", Supported: []string{"en-US", "pl"}}, StateKeys: []string{}, Assets: []string{}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestParseStarlarkManifestStrictAuthority(t *testing.T) {
	t.Parallel()
	manifest, err := ParseStarlarkManifest(validStarlarkManifestJSON(t))
	if err != nil {
		t.Fatal(err)
	}
	requested := manifest.RequestedPermissions()
	if !requested.Network.HTTP {
		t.Fatal("HTTP request was not projected")
	}
	effective := permissions.Effective(requested, requested)
	capabilities := manifest.Capabilities(effective)
	if len(capabilities) != 1 || capabilities[0] != contract.CapabilityNetworkHTTP {
		t.Fatalf("capabilities=%v", capabilities)
	}
}
func TestParseStarlarkManifestRejectsMissingNullUnknownAndDuplicateFields(t *testing.T) {
	t.Parallel()
	valid := string(validStarlarkManifestJSON(t))
	cases := map[string]string{"missing": strings.Replace(valid, `"assets":[]`, `"removed":[]`, 1), "null": strings.Replace(valid, `"assets":[]`, `"assets":null`, 1), "unknown": strings.Replace(valid, `"assets":[]`, `"assets":[],"unknown":true`, 1), "duplicate": strings.Replace(valid, `"id":"fun"`, `"id":"fun","id":"other"`, 1), "wrong-type": strings.Replace(valid, `"entrypoint":"plugin.star"`, `"entrypoint":12`, 1)}
	for name, data := range cases {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseStarlarkManifest([]byte(data)); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}
func TestStarlarkManifestRejectsUndeclaredOrUnsupportedAuthority(t *testing.T) {
	t.Parallel()
	base := string(validStarlarkManifestJSON(t))
	cases := []string{strings.Replace(base, `"hosts":["kawaii.red"]`, `"hosts":[]`, 1), strings.Replace(base, `"reactions":false`, `"reactions":true`, 1), strings.Replace(base, `"default":"en-US"`, `"default":"de"`, 1), strings.Replace(base, `"assets":[]`, `"assets":["../secret"]`, 1)}
	for _, data := range cases {
		if _, err := ParseStarlarkManifest([]byte(data)); err == nil {
			t.Fatalf("invalid authority accepted: %s", data)
		}
	}
}

func TestPortedBundleManifestsUseStrictStarlarkAuthority(t *testing.T) {
	t.Parallel()
	paths := []string{
		filepath.Join("..", "..", "..", "..", "examples", "plugins", "example", "bundles", "example-v0.1.0", "plugin.json"),
		filepath.Join("..", "..", "..", "..", "plugins", "fun", "bundles", "release-v0.1.0", "plugin.json"),
		filepath.Join("..", "..", "..", "..", "plugins", "info", "bundles", "release-v0.1.0", "plugin.json"),
		filepath.Join("..", "..", "..", "..", "plugins", "manager", "bundles", "release-v0.1.0", "plugin.json"),
		filepath.Join("..", "..", "..", "..", "plugins", "moderation", "bundles", "release-v0.1.0", "plugin.json"),
		filepath.Join("..", "..", "..", "..", "plugins", "wellness", "bundles", "release-v0.1.0", "plugin.json"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseStarlarkManifest(data); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
}

func TestBundleAuthorityRejectsUndeclaredAndNoncanonicalFiles(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"extra.txt", ".hidden.star"} {
		name := name
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeTestPluginBundle(t, dir, "strict")
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			manifest, err := ReadStarlarkManifest(filepath.Join(dir, "plugin.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err = ValidateBundleAuthority(dir, manifest); err == nil {
				t.Fatalf("accepted %q", name)
			}
		})
	}
}
func TestManifestRejectsReservedAssetPaths(t *testing.T) {
	t.Parallel()
	manifest, err := ParseStarlarkManifest(validStarlarkManifestJSON(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range []string{"plugin.star", "signature.json", "locales/en-US/messages.json", "lib/module.star"} {
		manifest.Assets = []string{asset}
		if err := manifest.Validate(); err == nil {
			t.Errorf("accepted %q", asset)
		}
	}
}

func TestManifestAssetsRequireExplicitResourceRead(t *testing.T) {
	manifest, err := ParseStarlarkManifest(validStarlarkManifestJSON(t))
	if err != nil {
		t.Fatal(err)
	}
	manifest.Assets = []string{"assets/message.txt"}
	if err := manifest.Validate(); err == nil {
		t.Fatal("asset accepted without resources.read")
	}
	manifest.Permissions.Resources.Read = true
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	manifest.Assets = []string{}
	if err := manifest.Validate(); err == nil {
		t.Fatal("resources.read accepted without asset")
	}
}

func TestBundleResourcesAreImmutableGenerationOwnedSnapshots(t *testing.T) {
	dir := t.TempDir()
	manifest, err := ParseStarlarkManifest(validStarlarkManifestJSON(t))
	if err != nil {
		t.Fatal(err)
	}
	manifest.Permissions.Network = StarlarkManifestNetworkPermissions{}
	manifest.Permissions.Resources.Read = true
	manifest.Assets = []string{"assets/message.bin"}
	for name, content := range map[string]string{"plugin.json": "{}", "plugin.star": "PLUGIN = None\n", "locales/en-US/messages.json": "[]\n", "locales/pl/messages.json": "[]\n", "assets/message.bin": "before"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateBundleAuthority(dir, manifest); err != nil {
		t.Fatal(err)
	}
	resources, err := ReadBundleResources(dir, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "message.bin"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if string(resources["assets/message.bin"]) != "before" {
		t.Fatalf("resource=%q", resources["assets/message.bin"])
	}
	resources["assets/message.bin"][0] = 'X'
	second, err := ReadBundleResources(dir, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if string(second["assets/message.bin"]) != "after" {
		t.Fatalf("second=%q", second["assets/message.bin"])
	}
}

func writeTestPluginBundle(t *testing.T, dir, pluginID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "locales", "en-US"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := StarlarkManifest{
		Schema: StarlarkManifestSchema, ID: pluginID, Name: pluginID, Version: "0.1.0", Entrypoint: StarlarkEntrypoint,
		Permissions: StarlarkManifestPermissions{Network: StarlarkManifestNetworkPermissions{Hosts: []string{}}},
		Locales:     StarlarkManifestLocales{Default: "en-US", Supported: []string{"en-US"}}, StateKeys: []string{}, Assets: []string{},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string][]byte{
		"plugin.json": data,
		"plugin.star": []byte("PLUGIN = None\n"),
		filepath.Join("locales", "en-US", "messages.json"): []byte("{}\n"),
	} {
		if err := os.WriteFile(filepath.Join(dir, path), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
