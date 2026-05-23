package ops_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/ops"
)

func TestNewHandler_HealthAndReadiness(t *testing.T) {
	snap := ops.Snapshot{}
	handler := ops.NewHandler(func() ops.Snapshot { return snap })

	t.Run("healthz", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != "ok" {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("readyz not ready", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
	})

	t.Run("readyz ready", func(t *testing.T) {
		snap.Ready = true
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != "ready" {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

func TestNewHandler_Metrics(t *testing.T) {
	metrics := ops.NewMetrics()
	metrics.IncInteractions()
	metrics.IncInteractionFailures()
	metrics.IncPluginFailures()
	metrics.IncAutomationFailures()
	metrics.IncReminderFailures()
	snap := ops.Snapshot{Ready: true}
	metrics.FillSnapshot(&snap)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	ops.NewHandler(func() ops.Snapshot { return snap }).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"mamacord_ready 1",
		"mamacord_interactions_total 1",
		"mamacord_interaction_failures_total 1",
		"mamacord_plugin_failures_total 1",
		"mamacord_plugin_automation_failures_total 1",
		"mamacord_reminder_failures_total 1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("metrics missing %q in %q", want, text)
		}
	}
}
