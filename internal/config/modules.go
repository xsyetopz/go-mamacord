package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ModulesSchemaURL = "https://raw.githubusercontent.com/xsyetopz/go-mamacord/refs/heads/main/schemas/modules.schema.json"

type ModulesFile struct {
	Schema   string                 `json:"$schema"`
	Defaults ModuleDefaults         `json:"defaults,omitempty"`
	Modules  map[string]ModuleEntry `json:"modules,omitempty"`
}

type ModuleDefaults struct {
	PluginEnabled   *bool `json:"plugin_enabled,omitempty"`
	OfficialEnabled *bool `json:"official_enabled,omitempty"`
	UserEnabled     *bool `json:"user_enabled,omitempty"`
}

type ModuleEntry struct {
	Enabled *bool `json:"enabled,omitempty"`
}

func LoadModulesFile(path string) (ModulesFile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ModulesFile{Modules: map[string]ModuleEntry{}}, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ModulesFile{Modules: map[string]ModuleEntry{}}, nil
		}
		return ModulesFile{}, fmt.Errorf("read modules file %q: %w", path, err)
	}

	var file ModulesFile
	if err := json.Unmarshal(b, &file); err != nil {
		return ModulesFile{}, fmt.Errorf("parse modules file %q: %w", path, err)
	}
	if file.Schema != ModulesSchemaURL {
		return ModulesFile{}, fmt.Errorf("modules file $schema must be %q", ModulesSchemaURL)
	}
	if file.Modules == nil {
		file.Modules = map[string]ModuleEntry{}
	}
	return file, nil
}

func WriteModulesFile(path string, file ModulesFile) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("modules file path is required")
	}
	file.Schema = ModulesSchemaURL
	if file.Modules == nil {
		file.Modules = map[string]ModuleEntry{}
	}

	bytes, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal modules file %q: %w", path, err)
	}
	bytes = append(bytes, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create modules dir for %q: %w", path, err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		return fmt.Errorf("write modules file %q: %w", path, err)
	}
	return nil
}
