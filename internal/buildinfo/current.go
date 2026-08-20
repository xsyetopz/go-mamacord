package buildinfo

import (
	"encoding/base64"
	"strings"
)

// These variables can be overridden at build time with -ldflags.
//
//nolint:gochecknoglobals // Intentionally global to support simple -ldflags injection.
var (
	Version           = "dev"
	Repository        = "UNKNOWN"
	Description       = "A nurturing and protective Discord app."
	DescriptionBase64 = ""

	DeveloperURL     = "UNKNOWN"
	SupportServerURL = "UNKNOWN"
	MascotImageURL   = "UNKNOWN"
)

type Info struct {
	Version          string
	Repository       string
	Description      string
	DeveloperURL     string
	SupportServerURL string
	MascotImageURL   string
}

func Current() Info {
	return Info{
		Version:          strings.TrimSpace(Version),
		Repository:       normalizeMetadata(Repository),
		Description:      currentDescription(),
		DeveloperURL:     normalizeMetadata(DeveloperURL),
		SupportServerURL: normalizeMetadata(SupportServerURL),
		MascotImageURL:   normalizeMetadata(MascotImageURL),
	}
}

func currentDescription() string {
	encoded := strings.TrimSpace(DescriptionBase64)
	if encoded == "" {
		return strings.TrimSpace(Description)
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return strings.TrimSpace(Description)
	}
	return strings.TrimSpace(string(decoded))
}

func normalizeMetadata(value string) string {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "UNKNOWN") {
		return ""
	}
	return value
}
