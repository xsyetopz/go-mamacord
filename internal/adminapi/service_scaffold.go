package adminapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

func (s *Service) ScaffoldPlugin(req PluginScaffoldRequest) (PluginScaffoldResponse, error) {
	id := strings.TrimSpace(req.ID)
	name := strings.TrimSpace(req.Name)
	version := strings.TrimSpace(req.Version)
	locale := strings.TrimSpace(req.Locale)
	commandName := strings.TrimSpace(req.CommandName)
	commandDescription := strings.TrimSpace(req.CommandDescription)
	responseMessage := strings.TrimSpace(req.ResponseMessage)

	switch {
	case !pluginIDPattern.MatchString(id):
		return PluginScaffoldResponse{}, errors.New("plugin id must match ^[a-z][a-z0-9_]{1,31}$")
	case name == "":
		return PluginScaffoldResponse{}, errors.New("plugin name is required")
	case version == "":
		version = "0.1.0"
	case locale == "":
		locale = "en-US"
	case !pluginIDPattern.MatchString(commandName):
		if commandName == "" {
			commandName = id
		} else {
			return PluginScaffoldResponse{}, errors.New("command name must match ^[a-z][a-z0-9_]{1,31}$")
		}
	}
	if commandDescription == "" {
		commandDescription = "Run the " + name + " plugin command"
	}
	if responseMessage == "" {
		responseMessage = "Hello from " + name + "."
	}

	dir := filepath.Join(s.userPluginsDir(), id)
	if fileExists(dir) {
		return PluginScaffoldResponse{}, fmt.Errorf("plugin %q already exists", id)
	}
	srcDir, err := os.MkdirTemp("", "mamacord-plugin-scaffold.")
	if err != nil {
		return PluginScaffoldResponse{}, err
	}
	defer func() { _ = os.RemoveAll(srcDir) }()

	descID := "cmd." + commandName + ".desc"
	messageID := id + ".hello"

	manifest := pluginhost.Manifest{
		ID:          id,
		Name:        name,
		Version:     version,
		Permissions: req.Permissions,
	}
	manifestBytes, err := json.MarshalIndent(map[string]any{
		"$schema":     "https://raw.githubusercontent.com/xsyetopz/go-mamacord/refs/heads/main/schemas/plugin.schema.v1.json",
		"id":          manifest.ID,
		"name":        manifest.Name,
		"version":     manifest.Version,
		"permissions": manifest.Permissions,
	}, "", "  ")
	if err != nil {
		return PluginScaffoldResponse{}, err
	}

	pluginLua := fmt.Sprintf(`local hello = bot.require("commands/hello.lua")

return bot.plugin({
  commands = {
    bot.command("%s", {
      description_id = "%s",
      ephemeral = true,
      run = hello
    })
  }
})
`, commandName, descID)

	commandLua := fmt.Sprintf(`local i18n = bot.i18n
local ui = bot.ui

return function(_ctx)
  return ui.reply({
    content = i18n.t("%s", nil, nil),
    ephemeral = true
  })
end
`, messageID)

	localeBytes, err := json.MarshalIndent([]map[string]string{
		{"id": descID, "translation": commandDescription},
		{"id": messageID, "translation": responseMessage},
	}, "", "  ")
	if err != nil {
		return PluginScaffoldResponse{}, err
	}

	files := []struct {
		rel  string
		data []byte
	}{
		{rel: "plugin.json", data: append(manifestBytes, '\n')},
		{rel: "plugin.lua", data: []byte(pluginLua)},
		{rel: filepath.Join("commands", "hello.lua"), data: []byte(commandLua)},
		{rel: filepath.Join("locales", locale, "messages.json"), data: append(localeBytes, '\n')},
	}

	bundleMembers := make([]string, 0, len(files))
	for _, file := range files {
		fullPath := filepath.Join(srcDir, file.rel)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return PluginScaffoldResponse{}, err
		}
		if err := os.WriteFile(fullPath, file.data, 0o644); err != nil {
			return PluginScaffoldResponse{}, err
		}
		bundleMembers = append(bundleMembers, file.rel)
	}
	bundle, err := s.bundleRepo().MaterializeBundle(srcDir, dir, scaffoldBundleRevision(version))
	if err != nil {
		return PluginScaffoldResponse{}, err
	}
	created := make([]string, 0, len(bundleMembers)+1)
	created = append(created, bundles.StateFileName)
	for _, member := range bundleMembers {
		created = append(created, filepath.Join(bundle.BundleRelativeDir, member))
	}

	resp := PluginScaffoldResponse{
		ID:    id,
		Dir:   dir,
		Files: created,
	}
	if req.Sign {
		signaturePath, err := s.SignPlugin(id)
		if err != nil {
			return PluginScaffoldResponse{}, err
		}
		resp.Signed = true
		resp.Signature = signaturePath
		if rel, relErr := filepath.Rel(dir, signaturePath); relErr == nil {
			resp.Files = append(resp.Files, rel)
		} else {
			resp.Files = append(resp.Files, filepath.Base(signaturePath))
		}
	}
	return resp, nil
}

func (s *Service) SignPlugin(pluginID string) (string, error) {
	if !signingReady(s.Config) {
		return "", errors.New("dashboard signing is not configured")
	}
	dir, err := s.pluginDir(pluginID)
	if err != nil {
		return "", err
	}
	bundleDir, err := s.bundleRepo().ResolveBundleDir(dir)
	if err != nil {
		return "", err
	}
	if !fileExists(filepath.Join(bundleDir, "plugin.json")) {
		return "", fmt.Errorf("plugin %q not found", pluginID)
	}

	privateKey, err := pluginhost.ReadEd25519PrivateKeyFile(s.Config.DashboardSigningKeyFile)
	if err != nil {
		return "", err
	}
	sig, _, err := pluginhost.SignDir(bundleDir, s.Config.DashboardSigningKeyID, privateKey)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"$schema":       pluginhost.SignatureSchemaURL,
		"key_id":        sig.KeyID,
		"hash_b64":      sig.HashB64,
		"signature_b64": sig.SignatureB64,
		"algorithm":     sig.Algorithm,
	}
	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	target, err := s.bundleRepo().WriteBundleSignature(dir, append(bytes, '\n'))
	if err != nil {
		return "", err
	}
	return target, nil
}

func scaffoldBundleRevision(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "0.1.0"
	}
	name := "manual-v" + version
	runes := make([]rune, 0, len(name))
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			runes = append(runes, r)
			continue
		}
		runes = append(runes, '-')
	}
	clean := strings.Trim(string(runes), ".-_")
	if clean == "" {
		clean = "manual"
	}
	return clean
}
