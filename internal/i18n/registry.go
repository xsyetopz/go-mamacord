package i18n

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type Registry struct {
	state *registryState
}

type registryState struct {
	mu sync.RWMutex

	core    *i18n.Bundle
	plugins map[string]*i18n.Bundle
	locales []string
}

const (
	localeEnUS = "en-US"
	localeEnGB = "en-GB"
)

func LoadCore(localesDir string) (Registry, error) {
	locales, err := listLocales(localesDir)
	if err != nil {
		return Registry{}, err
	}

	bundle := i18n.NewBundle(language.MustParse(localeEnUS))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	for _, locale := range locales {
		path := filepath.Join(localesDir, locale, "messages.json")
		if loadErr := loadMessages(bundle, locale, path); loadErr != nil {
			return Registry{}, fmt.Errorf("load %q: %w", path, loadErr)
		}
	}

	return Registry{
		state: &registryState{
			core:    bundle,
			plugins: map[string]*i18n.Bundle{},
			locales: locales,
		},
	}, nil
}

func (r *Registry) LoadPluginLocales(pluginID, pluginLocalesDir string) error {
	if strings.TrimSpace(pluginID) == "" {
		return errors.New("plugin id is required")
	}
	if r == nil || r.state == nil {
		return errors.New("i18n registry not initialized")
	}

	locales, err := listLocales(pluginLocalesDir)
	if err != nil {
		return err
	}

	bundle := i18n.NewBundle(language.MustParse(localeEnUS))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	for _, locale := range locales {
		path := filepath.Join(pluginLocalesDir, locale, "messages.json")
		if loadErr := loadMessages(bundle, locale, path); loadErr != nil {
			return fmt.Errorf("load %q: %w", path, loadErr)
		}
	}

	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	if r.state.plugins == nil {
		r.state.plugins = map[string]*i18n.Bundle{}
	}

	r.state.plugins[pluginID] = bundle
	return nil
}

func (r *Registry) SupportedLocales() []string {
	if r == nil || r.state == nil {
		return nil
	}
	r.state.mu.RLock()
	defer r.state.mu.RUnlock()
	return append([]string(nil), r.state.locales...)
}

func (r *Registry) ResetPluginLocales() {
	if r == nil || r.state == nil {
		return
	}
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	r.state.plugins = map[string]*i18n.Bundle{}
}

func normalizeLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return localeEnUS
	}
	return locale
}

type Config struct {
	Locale       string
	PluginID     string
	MessageID    string
	TemplateData map[string]any
	PluralCount  any
}

func (r *Registry) Localize(cfg Config) (string, error) {
	if r == nil || r.state == nil {
		return "", errors.New("i18n registry not initialized")
	}

	messageID := strings.TrimSpace(cfg.MessageID)
	if messageID == "" {
		return "", nil
	}

	locale := normalizeLocale(cfg.Locale)
	fallbackPrimary := localeEnUS
	fallbackSecondary := localeEnGB

	if cfg.PluginID != "" {
		r.state.mu.RLock()
		bundle, ok := r.state.plugins[cfg.PluginID]
		r.state.mu.RUnlock()
		if !ok || bundle == nil {
			return "", fmt.Errorf("missing plugin locale bundle for %q", cfg.PluginID)
		}

		localizer := i18n.NewLocalizer(bundle, locale, fallbackPrimary, fallbackSecondary)
		text, err := localizer.Localize(&i18n.LocalizeConfig{
			MessageID:    messageID,
			TemplateData: cfg.TemplateData,
			PluralCount:  cfg.PluralCount,
		})
		if err == nil {
			return text, nil
		}
	}

	r.state.mu.RLock()
	core := r.state.core
	r.state.mu.RUnlock()
	localizer := i18n.NewLocalizer(core, locale, fallbackPrimary, fallbackSecondary)
	return localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: cfg.TemplateData,
		PluralCount:  cfg.PluralCount,
	})
}

func (r *Registry) MustLocalize(cfg Config) string {
	s, err := r.Localize(cfg)
	if err != nil {
		if cfg.MessageID == "" {
			return ""
		}
		return cfg.MessageID
	}
	return s
}

func (r *Registry) TryLocalize(cfg Config) (string, bool) {
	s, err := r.Localize(cfg)
	if err != nil || strings.TrimSpace(s) == "" {
		return "", false
	}
	return s, true
}

func listLocales(localesDir string) ([]string, error) {
	localesDir = strings.TrimSpace(localesDir)
	if localesDir == "" {
		return nil, errors.New("locales dir is required")
	}

	entries, err := os.ReadDir(localesDir)
	if err != nil {
		return nil, fmt.Errorf("read locales dir %q: %w", localesDir, err)
	}

	var locales []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		locale := entry.Name()
		if !IsSupportedDiscordLocale(locale) {
			continue
		}
		path := filepath.Join(localesDir, locale, "messages.json")
		if _, statErr := os.Stat(path); statErr != nil {
			continue
		}

		locales = append(locales, locale)
	}

	sort.Strings(locales)
	return locales, nil
}

func loadMessages(bundle *i18n.Bundle, locale, filePath string) error {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	if err := validateMessagesJSON(b); err != nil {
		return err
	}

	// go-i18n infers language tags from file names, not directory names.
	// Keep our on-disk layout `/<locale>/messages.json`, but pass a synthetic
	// file name with the locale embedded so tags are parsed correctly.
	synthetic := "active." + strings.TrimSpace(locale) + ".json"
	_, err = bundle.ParseMessageFileBytes(b, synthetic)
	return err
}

func NewPluginSnapshot(base *Registry, pluginID, pluginLocalesDir string) (Registry, error) {
	core := i18n.NewBundle(language.MustParse(localeEnUS))
	core.RegisterUnmarshalFunc("json", json.Unmarshal)
	locales := []string{localeEnUS}
	if base != nil && base.state != nil {
		base.state.mu.RLock()
		core = base.state.core
		locales = append([]string(nil), base.state.locales...)
		base.state.mu.RUnlock()
	}
	snapshot := Registry{state: &registryState{core: core, plugins: map[string]*i18n.Bundle{}, locales: locales}}
	if err := snapshot.LoadPluginLocales(pluginID, pluginLocalesDir); err != nil {
		return Registry{}, err
	}
	return snapshot, nil
}

type messageJSONEntry struct {
	ID          json.RawMessage `json:"id"`
	Description json.RawMessage `json:"description"`
	Translation json.RawMessage `json:"translation"`
}

func validateMessagesJSON(data []byte) error {
	if len(data) == 0 || len(data) > 512*1024 {
		return errors.New("messages JSON size is invalid")
	}
	if !utf8.Valid(data) {
		return errors.New("messages JSON is not valid UTF-8")
	}
	if err := rejectDuplicateMessageJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var entries []messageJSONEntry
	if err := decoder.Decode(&entries); err != nil {
		return fmt.Errorf("parse messages JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("messages JSON contains trailing data")
	}
	if entries == nil || len(entries) > 5000 {
		return errors.New("messages JSON entry count is invalid")
	}
	seen := map[string]struct{}{}
	for index, entry := range entries {
		var id string
		if len(entry.ID) == 0 || bytes.Equal(bytes.TrimSpace(entry.ID), []byte("null")) || json.Unmarshal(entry.ID, &id) != nil || id != strings.TrimSpace(id) || id == "" || utf8.RuneCountInString(id) > 200 {
			return fmt.Errorf("message %d id is invalid", index+1)
		}
		for _, character := range id {
			if unicode.IsControl(character) {
				return fmt.Errorf("message %d id is invalid", index+1)
			}
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("message id %q is duplicated", id)
		}
		seen[id] = struct{}{}
		if len(entry.Description) != 0 {
			var description string
			if bytes.Equal(bytes.TrimSpace(entry.Description), []byte("null")) || json.Unmarshal(entry.Description, &description) != nil || !utf8.ValidString(description) || utf8.RuneCountInString(description) > 2000 {
				return fmt.Errorf("message %q description is invalid", id)
			}
		}
		if err := validateTranslationJSON(id, entry.Translation); err != nil {
			return err
		}
	}
	return nil
}
func validateTranslationJSON(id string, raw json.RawMessage) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("message %q translation is required", id)
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return validateTranslationText(id, text)
	}
	var plural map[string]string
	if json.Unmarshal(raw, &plural) != nil || plural == nil || len(plural) == 0 || len(plural) > 20 {
		return fmt.Errorf("message %q translation is invalid", id)
	}
	for form, value := range plural {
		if form != strings.TrimSpace(form) || form == "" || len(form) > 32 {
			return fmt.Errorf("message %q plural form is invalid", id)
		}
		if err := validateTranslationText(id, value); err != nil {
			return err
		}
	}
	return nil
}
func validateTranslationText(id, text string) error {
	if text == "" || !utf8.ValidString(text) || utf8.RuneCountInString(text) > 16384 {
		return fmt.Errorf("message %q translation text is invalid", id)
	}
	return nil
}
func rejectDuplicateMessageJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var consume func(string) error
	consume = func(location string) error {
		token, err := decoder.Token()
		if err != nil {
			return err
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
					return errors.New("messages JSON object key is invalid")
				}
				if _, exists := seen[key]; exists {
					return fmt.Errorf("messages JSON duplicate key %q at %s", key, location)
				}
				seen[key] = struct{}{}
				if err := consume(location + "." + key); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim('}') {
				return errors.New("messages JSON object is invalid")
			}
		case '[':
			index := 0
			for decoder.More() {
				if err := consume(fmt.Sprintf("%s[%d]", location, index)); err != nil {
					return err
				}
				index++
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim(']') {
				return errors.New("messages JSON array is invalid")
			}
		default:
			return errors.New("messages JSON delimiter is invalid")
		}
		return nil
	}
	if err := consume("messages"); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("messages JSON contains trailing data")
	}
	return nil
}
