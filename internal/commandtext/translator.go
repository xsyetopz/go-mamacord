package commandtext

import (
	"maps"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/persona"
)

type Translator struct {
	Registry i18n.Registry
	Locale   string
	PluginID string
	UserID   uint64
}

func (t Translator) S(messageID string, data map[string]any) string {
	if t.UserID != 0 {
		data = withPersonaTemplateData(data, t.Locale, t.UserID, messageID)
	}
	return t.Registry.MustLocalize(i18n.Config{
		Locale:       strings.TrimSpace(t.Locale),
		PluginID:     strings.TrimSpace(t.PluginID),
		MessageID:    messageID,
		TemplateData: data,
	})
}

func withPersonaTemplateData(
	data map[string]any,
	locale string,
	userID uint64,
	messageID string,
) map[string]any {
	if data == nil {
		data = map[string]any{}
	} else {
		clone := make(map[string]any, len(data)+1)
		maps.Copy(clone, data)
		data = clone
	}

	if _, ok := data["Pet"]; !ok {
		data["Pet"] = personaPet(locale, userID, messageID)
	}
	if _, ok := data["Mommy"]; !ok {
		data["Mommy"] = personaMommy(locale)
	}
	return data
}

func personaPet(locale string, userID uint64, messageID string) string {
	return persona.PetName(locale, userID, messageID)
}

func personaMommy(locale string) string {
	return persona.Mommy(locale)
}
