package i18n_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
)

func bundledFirstPartyPluginLocalesDir(t *testing.T, pluginID string) string {
	t.Helper()

	dir, err := bundles.NewLocalRepository().ResolveActiveDir(filepath.Join("..", "..", "plugins", pluginID))
	if err != nil {
		t.Fatalf("ResolveActiveDir(%q): %v", pluginID, err)
	}
	return filepath.Join(dir, "locales")
}

func TestLoadPluginLocales_OnlyLoadsSupportedDiscordLocales(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	mustWrite := func(path string, b []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	mustWrite(filepath.Join(dir, "en-US", "messages.json"), []byte(`[
  {"id":"ok.en_us","translation":"ok"}
]`))
	mustWrite(filepath.Join(dir, "xx-YY", "messages.json"), []byte(`[
  {"id":"only.invalid","translation":"should_not_load"}
]`))

	r, err := i18n.LoadCore(filepath.Join("..", "..", "locales"))
	if err != nil {
		t.Fatalf("LoadCore: %v", err)
	}

	if loadErr := r.LoadPluginLocales("p1", dir); loadErr != nil {
		t.Fatalf("LoadPluginLocales: %v", loadErr)
	}

	if _, ok := r.TryLocalize(i18n.Config{PluginID: "p1", Locale: "en-US", MessageID: "ok.en_us"}); !ok {
		t.Fatalf("expected en-US plugin message to be loaded")
	}

	if _, ok := r.TryLocalize(i18n.Config{PluginID: "p1", Locale: "xx-YY", MessageID: "only.invalid"}); ok {
		t.Fatalf("expected unsupported locale folder to be ignored")
	}
}

func TestFunPluginLocales_OwnTheirMessages(t *testing.T) {
	t.Parallel()

	r, err := i18n.LoadCore(filepath.Join("..", "..", "locales"))
	if err != nil {
		t.Fatalf("LoadCore: %v", err)
	}

	if _, ok := r.TryLocalize(i18n.Config{Locale: "de", MessageID: "fun.flip.result"}); ok {
		t.Fatalf("expected core locales to no longer expose fun.flip.result")
	}

	if loadErr := r.LoadPluginLocales("fun", bundledFirstPartyPluginLocalesDir(t, "fun")); loadErr != nil {
		t.Fatalf("LoadPluginLocales(fun): %v", loadErr)
	}

	got, ok := r.TryLocalize(i18n.Config{
		PluginID:     "fun",
		Locale:       "de",
		MessageID:    "fun.flip.result",
		TemplateData: map[string]any{"Pet": "pet", "User": "@user", "Result": "heads"},
	})
	if !ok {
		t.Fatalf("expected plugin locale to expose fun.flip.result")
	}
	if got == "" {
		t.Fatalf("expected localized fun.flip.result content")
	}

	desc, ok := r.TryLocalize(i18n.Config{
		PluginID:  "fun",
		Locale:    "de",
		MessageID: "cmd.flip.desc",
	})
	if !ok || desc == "" {
		t.Fatalf("expected plugin locale to expose cmd.flip.desc")
	}
}

func TestWellnessPluginLocales_OwnTheirMessages(t *testing.T) {
	t.Parallel()

	r, err := i18n.LoadCore(filepath.Join("..", "..", "locales"))
	if err != nil {
		t.Fatalf("LoadCore: %v", err)
	}

	if _, ok := r.TryLocalize(i18n.Config{Locale: "de", MessageID: "wellness.timezone.set"}); ok {
		t.Fatalf("expected core locales to no longer expose wellness.timezone.set")
	}

	if loadErr := r.LoadPluginLocales("wellness", bundledFirstPartyPluginLocalesDir(t, "wellness")); loadErr != nil {
		t.Fatalf("LoadPluginLocales(wellness): %v", loadErr)
	}

	got, ok := r.TryLocalize(i18n.Config{
		PluginID:     "wellness",
		Locale:       "de",
		MessageID:    "wellness.timezone.set",
		TemplateData: map[string]any{"Timezone": "Europe/Tallinn"},
	})
	if !ok || got == "" {
		t.Fatalf("expected plugin locale to expose wellness.timezone.set")
	}

	desc, ok := r.TryLocalize(i18n.Config{
		PluginID:  "wellness",
		Locale:    "de",
		MessageID: "cmd.timezone.desc",
	})
	if !ok || desc == "" {
		t.Fatalf("expected plugin locale to expose cmd.timezone.desc")
	}

	coreFallback, ok := r.TryLocalize(i18n.Config{
		PluginID:  "wellness",
		Locale:    "de",
		MessageID: "err.generic",
	})
	if !ok || coreFallback == "" {
		t.Fatalf("expected plugin lookup to fall back to core errors")
	}
}

func TestInfoPluginLocales_OwnTheirMessages(t *testing.T) {
	t.Parallel()

	r, err := i18n.LoadCore(filepath.Join("..", "..", "locales"))
	if err != nil {
		t.Fatalf("LoadCore: %v", err)
	}

	if _, ok := r.TryLocalize(i18n.Config{Locale: "de", MessageID: "info.about.title"}); ok {
		t.Fatalf("expected core locales to no longer expose info.about.title")
	}

	if loadErr := r.LoadPluginLocales("info", bundledFirstPartyPluginLocalesDir(t, "info")); loadErr != nil {
		t.Fatalf("LoadPluginLocales(info): %v", loadErr)
	}

	got, ok := r.TryLocalize(i18n.Config{
		PluginID:     "info",
		Locale:       "de",
		MessageID:    "info.about.title",
		TemplateData: map[string]any{"Version": "1.2.3"},
	})
	if !ok || got == "" {
		t.Fatalf("expected plugin locale to expose info.about.title")
	}

	desc, ok := r.TryLocalize(i18n.Config{
		PluginID:  "info",
		Locale:    "de",
		MessageID: "cmd.lookup.desc",
	})
	if !ok || desc == "" {
		t.Fatalf("expected plugin locale to expose cmd.lookup.desc")
	}

	coreFallback, ok := r.TryLocalize(i18n.Config{
		PluginID:  "info",
		Locale:    "de",
		MessageID: "err.generic",
	})
	if !ok || coreFallback == "" {
		t.Fatalf("expected plugin lookup to fall back to core errors")
	}
}

func TestModerationPluginLocales_OwnTheirMessages(t *testing.T) {
	t.Parallel()

	r, err := i18n.LoadCore(filepath.Join("..", "..", "locales"))
	if err != nil {
		t.Fatalf("LoadCore: %v", err)
	}

	if _, ok := r.TryLocalize(i18n.Config{Locale: "de", MessageID: "mod.warn.success"}); ok {
		t.Fatalf("expected core locales to no longer expose mod.warn.success")
	}

	if loadErr := r.LoadPluginLocales("moderation", bundledFirstPartyPluginLocalesDir(t, "moderation")); loadErr != nil {
		t.Fatalf("LoadPluginLocales(moderation): %v", loadErr)
	}

	got, ok := r.TryLocalize(i18n.Config{
		PluginID:     "moderation",
		Locale:       "de",
		MessageID:    "mod.warn.success",
		TemplateData: map[string]any{"User": "@user", "Reason": "reason", "TimeoutMinutes": 10, "TimeoutFailed": false},
	})
	if !ok || got == "" {
		t.Fatalf("expected plugin locale to expose mod.warn.success")
	}

	desc, ok := r.TryLocalize(i18n.Config{
		PluginID:  "moderation",
		Locale:    "de",
		MessageID: "cmd.warn.desc",
	})
	if !ok || desc == "" {
		t.Fatalf("expected plugin locale to expose cmd.warn.desc")
	}

	coreFallback, ok := r.TryLocalize(i18n.Config{
		PluginID:  "moderation",
		Locale:    "de",
		MessageID: "err.generic",
	})
	if !ok || coreFallback == "" {
		t.Fatalf("expected plugin lookup to fall back to core errors")
	}
}

func TestManagerPluginLocales_OwnTheirMessages(t *testing.T) {
	t.Parallel()

	r, err := i18n.LoadCore(filepath.Join("..", "..", "locales"))
	if err != nil {
		t.Fatalf("LoadCore: %v", err)
	}

	if _, ok := r.TryLocalize(i18n.Config{Locale: "de", MessageID: "mgr.slowmode.set"}); ok {
		t.Fatalf("expected core locales to no longer expose mgr.slowmode.set")
	}

	if loadErr := r.LoadPluginLocales("manager", bundledFirstPartyPluginLocalesDir(t, "manager")); loadErr != nil {
		t.Fatalf("LoadPluginLocales(manager): %v", loadErr)
	}

	got, ok := r.TryLocalize(i18n.Config{
		PluginID:     "manager",
		Locale:       "de",
		MessageID:    "mgr.slowmode.set",
		TemplateData: map[string]any{"Channel": "#general", "Seconds": 5},
	})
	if !ok || got == "" {
		t.Fatalf("expected plugin locale to expose mgr.slowmode.set")
	}

	desc, ok := r.TryLocalize(i18n.Config{
		PluginID:  "manager",
		Locale:    "de",
		MessageID: "cmd.roles.desc",
	})
	if !ok || desc == "" {
		t.Fatalf("expected plugin locale to expose cmd.roles.desc")
	}

	coreFallback, ok := r.TryLocalize(i18n.Config{
		PluginID:  "manager",
		Locale:    "de",
		MessageID: "err.generic",
	})
	if !ok || coreFallback == "" {
		t.Fatalf("expected plugin lookup to fall back to core errors")
	}
}
