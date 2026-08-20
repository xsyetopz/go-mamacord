package postgresstore_test

import (
	"context"
	"strings"
	"testing"

	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
)

func TestOpenRejectsEmptyDSN(t *testing.T) {
	t.Parallel()

	if _, err := postgresstore.Open(context.Background(), postgresstore.Options{}); err == nil {
		t.Fatal("expected empty dsn to fail")
	}
}

func TestOpenRejectsMalformedDSN(t *testing.T) {
	t.Parallel()

	_, err := postgresstore.Open(context.Background(), postgresstore.Options{DSN: "://bad"})
	if err == nil {
		t.Fatal("expected malformed dsn to fail")
	}
	if !strings.Contains(err.Error(), "dsn") {
		t.Fatalf("expected dsn error, got %v", err)
	}
}
