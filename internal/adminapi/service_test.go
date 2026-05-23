package adminapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/config"
)

func TestScaffoldPluginCreatesExpectedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := Service{
		Config: config.Config{
			UserPluginsDir: dir,
		},
	}

	resp, err := svc.ScaffoldPlugin(PluginScaffoldRequest{
		ID:                 "sample",
		Name:               "Sample",
		Version:            "0.1.0",
		Locale:             "en-US",
		CommandName:        "sample",
		CommandDescription: "Sample command",
		ResponseMessage:    "Hello from Sample.",
	})
	if err != nil {
		t.Fatalf("ScaffoldPlugin: %v", err)
	}
	if resp.ID != "sample" {
		t.Fatalf("unexpected id: %q", resp.ID)
	}

	for _, rel := range []string{
		"plugin.json",
		"plugin.lua",
		filepath.Join("commands", "hello.lua"),
		filepath.Join("locales", "en-US", "messages.json"),
	} {
		if _, err := os.Stat(filepath.Join(dir, "sample", rel)); err != nil {
			t.Fatalf("expected file %q: %v", rel, err)
		}
	}

	bytes, err := os.ReadFile(filepath.Join(dir, "sample", "plugin.json"))
	if err != nil {
		t.Fatalf("ReadFile(plugin.json): %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		t.Fatalf("json.Unmarshal(plugin.json): %v", err)
	}
	if got, _ := manifest["id"].(string); got != "sample" {
		t.Fatalf("unexpected manifest id: %q", got)
	}
}
