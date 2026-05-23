package pluginhost

import (
	"errors"
	"strings"
)

const (
	customIDPrefix = "mamacord:pl:"
	maxCustomIDLen = 100
	customIDParts  = 2
)

func BuildCustomID(pluginID, localID string) (string, error) {
	pluginID = strings.TrimSpace(pluginID)
	localID = strings.TrimSpace(localID)
	if pluginID == "" || localID == "" {
		return "", errors.New("plugin_id and local_id are required")
	}
	if strings.Contains(pluginID, ":") || strings.Contains(localID, ":") {
		return "", errors.New("ids must not contain ':'")
	}

	out := customIDPrefix + pluginID + ":" + localID
	if len(out) > maxCustomIDLen {
		return "", errors.New("custom_id too long")
	}
	return out, nil
}

func ParseCustomID(customID string) (string, string, bool) {
	customID = strings.TrimSpace(customID)
	if !strings.HasPrefix(customID, customIDPrefix) {
		return "", "", false
	}

	rest := strings.TrimPrefix(customID, customIDPrefix)
	parts := strings.Split(rest, ":")
	if len(parts) != customIDParts {
		return "", "", false
	}

	pluginID := strings.TrimSpace(parts[0])
	localID := strings.TrimSpace(parts[1])
	if pluginID == "" || localID == "" {
		return "", "", false
	}
	if strings.Contains(pluginID, ":") || strings.Contains(localID, ":") {
		return "", "", false
	}
	return pluginID, localID, true
}
