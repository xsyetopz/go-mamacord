package admin

import (
	"slices"
	"testing"
)

func TestDefinitionsContainAdminCommandFamilies(t *testing.T) {
	t.Parallel()

	defs := Definitions()
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	slices.Sort(names)

	want := []string{"block", "modules", "plugins", "unblock"}
	if !slices.Equal(names, want) {
		t.Fatalf("Definitions names = %#v, want %#v", names, want)
	}
}
