package pluginhost

import (
	"strings"
	"testing"
)

func TestCustomIDIsCanonicalAndBounded(t *testing.T) {
	valid, err := BuildCustomID("example", "save")
	if err != nil || valid != "mamacord:pl:example:save" {
		t.Fatalf("valid=%q err=%v", valid, err)
	}
	if plugin, local, ok := ParseCustomID(valid); !ok || plugin != "example" || local != "save" {
		t.Fatalf("parsed=%q %q %v", plugin, local, ok)
	}
	for _, ids := range [][2]string{{" example", "save"}, {"Example", "save"}, {"example", "Save"}, {"example", "save:other"}} {
		if _, err := BuildCustomID(ids[0], ids[1]); err == nil {
			t.Errorf("accepted %#v", ids)
		}
	}
	for _, raw := range []string{" " + valid, valid + " ", "mamacord:pl:Example:save", "mamacord:pl:example:Save", "mamacord:pl:example:" + strings.Repeat("a", 80)} {
		if _, _, ok := ParseCustomID(raw); ok {
			t.Errorf("parsed %q", raw)
		}
	}
}
