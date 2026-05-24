package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/postgres"
)

func TestOpenRejectsEmptyDSN(t *testing.T) {
	t.Parallel()

	if _, err := postgres.Open(context.Background(), postgres.Options{}); err == nil {
		t.Fatal("expected empty dsn to fail")
	}
}

func TestOpenRejectsMalformedDSN(t *testing.T) {
	t.Parallel()

	_, err := postgres.Open(context.Background(), postgres.Options{DSN: "://bad"})
	if err == nil {
		t.Fatal("expected malformed dsn to fail")
	}
	if !strings.Contains(err.Error(), "dsn") {
		t.Fatalf("expected dsn error, got %v", err)
	}
}
