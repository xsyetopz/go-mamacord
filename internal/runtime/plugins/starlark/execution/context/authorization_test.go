package contextapi

import (
	"strings"
	"testing"
)

func TestAuthorizedHTTPURLRejectsAmbientTargets(t *testing.T) {
	t.Parallel()
	tests := []string{"http://kawaii.red/x", "https://user@kawaii.red/x", "https://kawaii.red:8443/x", "https://127.0.0.1/x", "https://metadata.internal/x", "https://kawaii.red/x#fragment", "https://kawaii.red./x", "https://kawaii.red/" + strings.Repeat("x", 4097)}
	for _, raw := range tests {
		if _, err := authorizedHTTPURL(raw, []string{"kawaii.red"}); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
