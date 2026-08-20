package starlark

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

const maxHTTPJSONBytes int64 = 64 * 1024

func (value *contextValue) httpGetJSON(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var rawURL string
	maxBytes := maxHTTPJSONBytes
	if err := starlarkgo.UnpackArgs("context.http_get_json", args, kwargs, "url", &rawURL, "max_bytes?", &maxBytes); err != nil {
		return nil, err
	}
	if value.services.HTTP == nil {
		return nil, errors.New("HTTP service is unavailable")
	}
	if !value.services.allows(contract.CapabilityNetworkHTTP) {
		return nil, fmt.Errorf("capability %q is not granted", contract.CapabilityNetworkHTTP)
	}
	if maxBytes < 1 || maxBytes > maxHTTPJSONBytes {
		return nil, fmt.Errorf("max_bytes must be between 1 and %d", maxHTTPJSONBytes)
	}
	canonical, err := authorizedHTTPURL(rawURL, value.services.HTTPHosts)
	if err != nil {
		return nil, err
	}
	if value.calls >= value.maxCalls {
		return nil, fmt.Errorf("host call limit %d exceeded", value.maxCalls)
	}
	value.calls++
	result, ok, err := value.services.HTTP.GetJSON(value.context, canonical, int64(maxBytes))
	if err != nil {
		return nil, err
	}
	if !ok {
		return starlarkgo.None, nil
	}
	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("HTTP JSON result: %w", err)
	}
	return raisePersistentValue(result)
}

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
