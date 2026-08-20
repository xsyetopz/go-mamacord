package contextapi

import (
	"errors"
	"net"
	"net/url"
	"slices"
	"strings"
)

func authorizedHTTPURL(raw string, allowedHosts []string) (string, error) {
	if len(raw) == 0 || len(raw) > 4096 {
		return "", errors.New("HTTP URL length is invalid")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("HTTP URL is invalid")
	}
	if parsed.Scheme != "https" || parsed.User != nil || parsed.Fragment != "" || parsed.Host == "" {
		return "", errors.New("HTTP URL must be an absolute HTTPS URL without user info or fragment")
	}
	hostname := strings.ToLower(parsed.Hostname())
	if strings.HasSuffix(hostname, ".") {
		return "", errors.New("HTTP URL host is not canonical")
	}
	if net.ParseIP(hostname) != nil || hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return "", errors.New("HTTP URL host is not allowed")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return "", errors.New("HTTP URL port is not allowed")
	}
	allowed := make([]string, 0, len(allowedHosts))
	for _, host := range allowedHosts {
		host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if host != "" {
			allowed = append(allowed, host)
		}
	}
	if !slices.Contains(allowed, strings.TrimSuffix(hostname, ".")) {
		return "", errors.New("HTTP URL host is not declared by the plugin")
	}
	parsed.Host = hostname
	if parsed.Port() == "443" {
		parsed.Host = hostname
	}
	return parsed.String(), nil
}
