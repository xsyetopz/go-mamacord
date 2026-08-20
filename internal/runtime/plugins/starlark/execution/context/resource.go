package contextapi

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

const maxResourcePathBytes = 240

func (value *contextValue) resource(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var resourcePath string
	if err := starlarkgo.UnpackArgs("context.resource", args, kwargs, "path", &resourcePath); err != nil {
		return nil, err
	}
	if value.services.Resources == nil {
		return nil, errors.New("resource service is unavailable")
	}
	if !value.services.Allows(contract.CapabilityResourcesRead) {
		return nil, fmt.Errorf("capability %q is not granted", contract.CapabilityResourcesRead)
	}
	if !utf8.ValidString(resourcePath) || len(resourcePath) == 0 || len(resourcePath) > maxResourcePathBytes || strings.Contains(resourcePath, "\\") || strings.HasPrefix(resourcePath, "/") || path.Clean(resourcePath) != resourcePath {
		return nil, errors.New("resource path is not canonical")
	}
	if value.calls >= value.maxCalls {
		return nil, fmt.Errorf("host call limit %d exceeded", value.maxCalls)
	}
	value.calls++
	content, err := value.services.Resources.ReadResource(value.context, resourcePath)
	if err != nil {
		return nil, err
	}
	if len(content) > 1024*1024 {
		return nil, errors.New("resource exceeds byte limit")
	}
	return starlarkgo.Bytes(string(content)), nil
}
