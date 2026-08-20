package pluginhost

import (
	"errors"
	"regexp"
	"strings"
)

var customLocalIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

const (
	customIDPrefix = "mamacord:pl:"
	maxCustomIDLen = 100
	customIDParts  = 2
)

func BuildCustomID(pluginID, localID string) (string, error) {
	if pluginID != strings.TrimSpace(pluginID) || localID != strings.TrimSpace(localID) || !manifestIDPattern.MatchString(pluginID) || !customLocalIDPattern.MatchString(localID) {
		return "", errors.New("plugin_id or local_id is invalid")
	}
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
	if customID != strings.TrimSpace(customID) || len(customID) > maxCustomIDLen {
		return "", "", false
	}
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
	if !manifestIDPattern.MatchString(pluginID) || !customLocalIDPattern.MatchString(localID) {
		return "", "", false
	}
	if strings.Contains(pluginID, ":") || strings.Contains(localID, ":") {
		return "", "", false
	}
	return pluginID, localID, true
}
