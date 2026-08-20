package pluginhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xsyetopz/go-mamacord/internal/permissions"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

const (
	StarlarkManifestSchema = "https://raw.githubusercontent.com/xsyetopz/go-mamacord/refs/heads/main/schemas/plugin.schema.json"
	StarlarkEntrypoint     = "plugin.star"
)

var (
	manifestIDPattern      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	manifestVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	localePattern          = regexp.MustCompile(`^[a-z]{2,3}(?:-[A-Z][a-z]{3})?(?:-(?:[A-Z]{2}|[0-9]{3}))?$`)
	hostPattern            = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
)

type StarlarkManifest struct {
	Schema      string                      `json:"$schema"`
	ID          string                      `json:"id"`
	Name        string                      `json:"name"`
	Version     string                      `json:"version"`
	Entrypoint  string                      `json:"entrypoint"`
	Permissions StarlarkManifestPermissions `json:"permissions"`
	Locales     StarlarkManifestLocales     `json:"locales"`
	StateKeys   []string                    `json:"state_keys"`
	Assets      []string                    `json:"assets"`
}

type StarlarkManifestPermissions struct {
	Storage    permissions.StoragePermissions     `json:"storage"`
	Discord    permissions.DiscordPermissions     `json:"discord"`
	Network    StarlarkManifestNetworkPermissions `json:"network"`
	Resources  permissions.ResourcePermissions    `json:"resources"`
	Automation permissions.AutomationPermissions  `json:"automation"`
}
type StarlarkManifestNetworkPermissions struct {
	HTTP  bool     `json:"http"`
	Hosts []string `json:"hosts"`
}
type StarlarkManifestLocales struct {
	Default   string   `json:"default"`
	Supported []string `json:"supported"`
}

func ParseStarlarkManifest(data []byte) (StarlarkManifest, error) {
	if len(data) == 0 {
		return StarlarkManifest{}, errors.New("manifest is empty")
	}
	if len(data) > 64*1024 {
		return StarlarkManifest{}, errors.New("manifest exceeds 65536 bytes")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return StarlarkManifest{}, err
	}
	if err := validateRequiredManifestFields(data); err != nil {
		return StarlarkManifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest StarlarkManifest
	if err := decoder.Decode(&manifest); err != nil {
		return StarlarkManifest{}, fmt.Errorf("parse Starlark manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return StarlarkManifest{}, errors.New("manifest contains multiple JSON values")
		}
		return StarlarkManifest{}, fmt.Errorf("parse Starlark manifest trailing data: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return StarlarkManifest{}, err
	}
	return manifest, nil
}

func (manifest StarlarkManifest) Validate() error {
	if manifest.Schema != StarlarkManifestSchema {
		return fmt.Errorf("manifest $schema must be %q", StarlarkManifestSchema)
	}
	if !manifestIDPattern.MatchString(manifest.ID) {
		return errors.New("manifest id is invalid")
	}
	if manifest.Name != strings.TrimSpace(manifest.Name) || manifest.Name == "" || !utf8.ValidString(manifest.Name) || utf8.RuneCountInString(manifest.Name) > 100 {
		return errors.New("manifest name is invalid")
	}
	for _, character := range manifest.Name {
		if unicode.IsControl(character) {
			return errors.New("manifest name is invalid")
		}
	}
	if !manifestVersionPattern.MatchString(manifest.Version) {
		return errors.New("manifest version must be SemVer")
	}
	if manifest.Entrypoint != StarlarkEntrypoint {
		return fmt.Errorf("manifest entrypoint must be %q", StarlarkEntrypoint)
	}
	if manifest.Permissions.Discord.Reactions || manifest.Permissions.Discord.Threads || manifest.Permissions.Discord.Invites || manifest.Permissions.Discord.Webhooks {
		return errors.New("manifest requests a Discord capability not supported by the Starlark API")
	}
	if err := validateManifestNetwork(manifest.Permissions.Network); err != nil {
		return err
	}
	if err := validateManifestLocales(manifest.Locales); err != nil {
		return err
	}
	if err := validateManifestStateKeys(manifest.StateKeys, manifest.Permissions.Storage.KV); err != nil {
		return err
	}
	if err := validateManifestAssets(manifest.Assets); err != nil {
		return err
	}
	if len(manifest.Assets) > 0 && !manifest.Permissions.Resources.Read {
		return errors.New("manifest assets require resources.read permission")
	}
	if manifest.Permissions.Resources.Read && len(manifest.Assets) == 0 {
		return errors.New("resources.read permission requires at least one asset")
	}
	return nil
}
func (manifest StarlarkManifest) RequestedPermissions() permissions.Permissions {
	return permissions.Permissions{Storage: manifest.Permissions.Storage, Discord: manifest.Permissions.Discord, Network: permissions.NetworkPermissions{HTTP: manifest.Permissions.Network.HTTP}, Resources: manifest.Permissions.Resources, Automation: manifest.Permissions.Automation}
}
func (manifest StarlarkManifest) Capabilities(effective permissions.Permissions) []contract.Capability {
	var values []contract.Capability
	add := func(enabled bool, capability contract.Capability) {
		if enabled {
			values = append(values, capability)
		}
	}
	add(effective.Storage.KV, contract.CapabilityStorageKV)
	add(effective.Storage.UserSettings, contract.CapabilityStorageUserSettings)
	add(effective.Storage.CheckIns, contract.CapabilityStorageCheckIns)
	add(effective.Storage.Reminders, contract.CapabilityStorageReminders)
	add(effective.Storage.Warnings, contract.CapabilityStorageWarnings)
	add(effective.Storage.Audit, contract.CapabilityStorageAudit)
	add(effective.Discord.Users, contract.CapabilityDiscordUsers)
	add(effective.Discord.Guilds, contract.CapabilityDiscordGuilds)
	add(effective.Discord.Channels, contract.CapabilityDiscordChannels)
	add(effective.Discord.Messages, contract.CapabilityDiscordMessages)
	add(effective.Discord.Members, contract.CapabilityDiscordMembers)
	add(effective.Discord.Roles, contract.CapabilityDiscordRoles)
	add(effective.Discord.Emojis, contract.CapabilityDiscordEmojis)
	add(effective.Discord.Stickers, contract.CapabilityDiscordStickers)
	add(effective.Network.HTTP, contract.CapabilityNetworkHTTP)
	add(effective.Resources.Read, contract.CapabilityResourcesRead)
	return values
}
func validateManifestNetwork(network StarlarkManifestNetworkPermissions) error {
	if network.HTTP && len(network.Hosts) == 0 {
		return errors.New("network HTTP permission requires at least one host")
	}
	if !network.HTTP && len(network.Hosts) != 0 {
		return errors.New("network hosts require HTTP permission")
	}
	if len(network.Hosts) > 20 {
		return errors.New("network host list exceeds 20 entries")
	}
	if !slices.IsSorted(network.Hosts) {
		return errors.New("network hosts must be sorted")
	}
	seen := map[string]struct{}{}
	for _, host := range network.Hosts {
		if host != strings.ToLower(host) || !hostPattern.MatchString(host) {
			return fmt.Errorf("network host %q is invalid", host)
		}
		if _, exists := seen[host]; exists {
			return fmt.Errorf("duplicate network host %q", host)
		}
		seen[host] = struct{}{}
	}
	return nil
}
func validateManifestLocales(locales StarlarkManifestLocales) error {
	if !localePattern.MatchString(locales.Default) {
		return errors.New("default locale is invalid")
	}
	if len(locales.Supported) == 0 || len(locales.Supported) > 50 {
		return errors.New("supported locales must contain 1 to 50 entries")
	}
	seen := map[string]struct{}{}
	for _, locale := range locales.Supported {
		if !localePattern.MatchString(locale) {
			return fmt.Errorf("locale %q is invalid", locale)
		}
		if _, exists := seen[locale]; exists {
			return fmt.Errorf("duplicate locale %q", locale)
		}
		seen[locale] = struct{}{}
	}
	if _, exists := seen[locales.Default]; !exists {
		return errors.New("supported locales omit default locale")
	}
	return nil
}
func validateManifestStateKeys(keys []string, storageKV bool) error {
	if keys == nil {
		return errors.New("manifest state_keys must be an array")
	}
	if len(keys) > 25 {
		return errors.New("manifest state_keys exceed 25 entries")
	}
	if len(keys) != 0 && !storageKV {
		return errors.New("manifest state_keys require storage.kv permission")
	}
	seen := map[string]struct{}{}
	for _, key := range keys {
		if key != strings.TrimSpace(key) || !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`).MatchString(key) {
			return fmt.Errorf("state key %q is invalid", key)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate state key %q", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateManifestAssets(assets []string) error {
	if assets == nil {
		return errors.New("manifest assets must be an array")
	}
	if len(assets) > 100 {
		return errors.New("manifest assets exceed 100 entries")
	}
	if !slices.IsSorted(assets) {
		return errors.New("manifest assets must be sorted")
	}
	seen := map[string]struct{}{}
	for _, asset := range assets {
		if !utf8.ValidString(asset) || len(asset) > 240 {
			return fmt.Errorf("asset path %q is invalid", asset)
		}
		if asset == "plugin.json" || asset == "plugin.star" || asset == "signature.json" || strings.HasPrefix(asset, "locales/") || strings.HasSuffix(strings.ToLower(asset), ".star") {
			return fmt.Errorf("asset path %q is reserved", asset)
		}
		if asset == "" || strings.Contains(asset, "\\") || strings.HasPrefix(asset, "/") || path.Clean(asset) != asset {
			return fmt.Errorf("asset path %q is not canonical", asset)
		}
		for _, component := range strings.Split(asset, "/") {
			if component == "" || component == "." || component == ".." || strings.HasPrefix(component, ".") {
				return fmt.Errorf("asset path %q contains invalid component", asset)
			}
		}
		if _, exists := seen[asset]; exists {
			return fmt.Errorf("duplicate asset path %q", asset)
		}
		seen[asset] = struct{}{}
	}
	return nil
}

func validateRequiredManifestFields(data []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse Starlark manifest: %w", err)
	}
	required := []string{"$schema", "id", "name", "version", "entrypoint", "permissions", "locales", "state_keys", "assets"}
	if err := requireObjectFields(root, "manifest", required); err != nil {
		return err
	}
	var permissionsObject map[string]json.RawMessage
	if err := json.Unmarshal(root["permissions"], &permissionsObject); err != nil {
		return errors.New("manifest permissions must be an object")
	}
	if err := requireObjectFields(permissionsObject, "permissions", []string{"storage", "discord", "network", "resources", "automation"}); err != nil {
		return err
	}
	groups := []struct {
		name   string
		fields []string
	}{{"storage", []string{"kv", "user_settings", "checkins", "reminders", "warnings", "audit"}}, {"discord", []string{"users", "guilds", "channels", "messages", "reactions", "members", "roles", "threads", "invites", "webhooks", "emojis", "stickers"}}, {"network", []string{"http", "hosts"}}, {"resources", []string{"read"}}, {"automation", []string{"jobs", "events"}}}
	for _, group := range groups {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(permissionsObject[group.name], &object); err != nil {
			return fmt.Errorf("permissions.%s must be an object", group.name)
		}
		if err := requireObjectFields(object, "permissions."+group.name, group.fields); err != nil {
			return err
		}
		if group.name == "automation" {
			var events map[string]json.RawMessage
			if err := json.Unmarshal(object["events"], &events); err != nil {
				return errors.New("permissions.automation.events must be an object")
			}
			if err := requireObjectFields(events, "permissions.automation.events", []string{"member_join_leave", "moderation"}); err != nil {
				return err
			}
		}
	}
	var locales map[string]json.RawMessage
	if err := json.Unmarshal(root["locales"], &locales); err != nil {
		return errors.New("manifest locales must be an object")
	}
	return requireObjectFields(locales, "locales", []string{"default", "supported"})
}
func requireObjectFields(object map[string]json.RawMessage, location string, required []string) error {
	if object == nil {
		return fmt.Errorf("%s must be an object", location)
	}
	for _, field := range required {
		raw, exists := object[field]
		if !exists {
			return fmt.Errorf("%s missing required field %q", location, field)
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("%s field %q cannot be null", location, field)
		}
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeJSONValue(decoder, "manifest"); err != nil {
		return err
	}
	if token, err := decoder.Token(); err == nil {
		return fmt.Errorf("manifest has trailing token %v", token)
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("parse Starlark manifest: %w", err)
	}
	return nil
}
func consumeJSONValue(decoder *json.Decoder, location string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("parse Starlark manifest: %w", err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("manifest object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%s has duplicate field %q", location, key)
			}
			seen[key] = struct{}{}
			if err := consumeJSONValue(decoder, location+"."+key); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("manifest object is not closed")
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := consumeJSONValue(decoder, fmt.Sprintf("%s[%d]", location, index)); err != nil {
				return err
			}
			index++
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("manifest array is not closed")
		}
	default:
		return errors.New("manifest contains invalid delimiter")
	}
	return nil
}

func ReadStarlarkManifest(path string) (StarlarkManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StarlarkManifest{}, err
	}
	return ParseStarlarkManifest(data)
}
