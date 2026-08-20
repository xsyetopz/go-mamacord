package auth

import (
	"encoding/json"
	"testing"
)

func TestOAuthPermissionsUnmarshal_StringAndNumber(t *testing.T) {
	t.Parallel()

	var asString OAuthGuild
	if err := json.Unmarshal([]byte(`{"id":"1","name":"Guild","owner":false,"permissions":"8"}`), &asString); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if got := string(asString.Permissions); got != "8" {
		t.Fatalf("permissions=%q want %q", got, "8")
	}

	var asNumber OAuthGuild
	if err := json.Unmarshal([]byte(`{"id":"1","name":"Guild","owner":false,"permissions":8}`), &asNumber); err != nil {
		t.Fatalf("unmarshal number: %v", err)
	}
	if got := string(asNumber.Permissions); got != "8" {
		t.Fatalf("permissions=%q want %q", got, "8")
	}
}
