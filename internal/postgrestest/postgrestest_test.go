package postgrestest

import (
	"net/url"
	"testing"
)

func TestOpenSchemaDSNStaysUniqueAcrossCounterReset(t *testing.T) {
	t.Parallel()

	schemaCounter = 0
	first := OpenSchemaDSN(t)

	schemaCounter = 0
	second := OpenSchemaDSN(t)

	if schemaSearchPath(t, first) == schemaSearchPath(t, second) {
		t.Fatalf("expected distinct schema search paths, got %q", schemaSearchPath(t, first))
	}
}

func schemaSearchPath(t *testing.T, raw string) string {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", raw, err)
	}
	return parsed.Query().Get("search_path")
}
