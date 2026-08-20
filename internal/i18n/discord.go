package i18n

import "strings"

func IsSupportedDiscordLocale(locale string) bool {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return false
	}

	switch locale {
	case "bg",
		"cs",
		"da",
		"de",
		"el",
		"en-GB",
		"en-US",
		"es-419",
		"es-ES",
		"fi",
		"fr",
		"hi",
		"hr",
		"hu",
		"id",
		"it",
		"ja",
		"ko",
		"lt",
		"nl",
		"no",
		"pl",
		"pt-BR",
		"ro",
		"ru",
		"sv-SE",
		"th",
		"tr",
		"uk",
		"vi",
		"zh-CN",
		"zh-TW":
		return true
	default:
		return false
	}
}
