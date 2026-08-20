package contextapi

import (
	"errors"
	"fmt"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/internal/evaluation"

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
	if !value.services.Allows(contract.CapabilityNetworkHTTP) {
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
	return evaluation.RaisePersistentValue(result)
}
