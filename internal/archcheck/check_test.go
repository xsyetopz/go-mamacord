package archcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditReportsFileGroupsAndStructFields(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	for index := 0; index < 11; index++ {
		name := fmt.Sprintf("group%02d.go", index)
		body := "package sample\n"
		if index == 0 {
			body += "type Oversized struct { A, B, C, D, E, F, G, H, I, J, K string }\n"
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "group00_test.go"), []byte("package sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Audit(root)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if report.OK() {
		t.Fatal("expected architecture violations")
	}
	if len(report.Violations) != 2 {
		t.Fatalf("violations = %#v, want 2", report.Violations)
	}

	fileViolation := report.Violations[0]
	if fileViolation.Kind != FileGroups || fileViolation.Path != "internal/sample" || fileViolation.Count != 11 {
		t.Fatalf("file violation = %#v", fileViolation)
	}
	structViolation := report.Violations[1]
	if structViolation.Kind != StructFields || structViolation.Name != "Oversized" || structViolation.Count != 11 {
		t.Fatalf("struct violation = %#v", structViolation)
	}
}

func TestAuditAcceptsLimitsAndGroupsTestsWithSource(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < MaxFileGroups; index++ {
		name := fmt.Sprintf("group%02d.go", index)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package sample\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "group00_test.go"), []byte("package sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Audit(root)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if !report.OK() {
		t.Fatalf("violations = %#v", report.Violations)
	}
}

func TestAuditRequiresInternalDirectory(t *testing.T) {
	_, err := Audit(t.TempDir())
	if err == nil {
		t.Fatal("expected missing internal directory error")
	}
}

func TestAuditReportsCategoricalFileColony(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"service_config.go", "service_modules.go", "service_plugins.go", "service_plugins_test.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package sample\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Audit(root)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(report.Violations) != 1 {
		t.Fatalf("violations = %#v, want one categorical colony", report.Violations)
	}
	violation := report.Violations[0]
	if violation.Kind != CategoricalFiles || violation.Name != "service" || violation.Count != 3 {
		t.Fatalf("categorical violation = %#v", violation)
	}
}

func TestAuditCountsCategoryRootInColony(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"context.go", "context_http.go", "context_reads.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package sample\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	report, err := Audit(root)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(report.Violations) != 1 || report.Violations[0].Kind != CategoricalFiles || report.Violations[0].Count != 3 {
		t.Fatalf("violations = %#v, want root-inclusive categorical colony", report.Violations)
	}
}

func TestAuditCountsStandaloneTestAsItsOwnGroup(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < MaxFileGroups; index++ {
		name := fmt.Sprintf("unit%02d.go", index)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package sample\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "standalone_test.go"), []byte("package sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Audit(root)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(report.Violations) != 1 || report.Violations[0].Kind != FileGroups || report.Violations[0].Count != 11 {
		t.Fatalf("violations = %#v, want standalone test counted as group 11", report.Violations)
	}
}

func TestAuditDoesNotGroupNonGoTestSuffixes(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 9; index++ {
		name := fmt.Sprintf("unit%02d.go", index)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package sample\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"config.json", "config_test.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	report, err := Audit(root)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(report.Violations) != 1 || report.Violations[0].Kind != FileGroups || report.Violations[0].Count != 11 {
		t.Fatalf("violations = %#v, want distinct non-Go files counted", report.Violations)
	}
}

func TestAuditReportsTwoFilePrefixAndSuffixCategories(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"service_config.go", "service_routes.go", "discord_locales.go", "plugin_locales_test.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package sample\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Audit(root)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(report.Violations) != 2 {
		t.Fatalf("violations = %#v, want prefix and suffix violations", report.Violations)
	}
	if got := report.Violations[0]; got.Kind != CategoricalFiles || got.Axis != "suffix" || got.Name != "locales" || got.Count != 2 {
		t.Fatalf("suffix violation = %#v", got)
	}
	if got := report.Violations[1]; got.Kind != CategoricalFiles || got.Axis != "prefix" || got.Name != "service" || got.Count != 2 {
		t.Fatalf("prefix violation = %#v", got)
	}
}

func TestAuditReportsDirectoryNameRepetition(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sample.go", "sample_cli.go", "contract_sample.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package sample\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Audit(root)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	var names []string
	for _, violation := range report.Violations {
		if violation.Kind == RedundantName {
			names = append(names, violation.Name)
		}
	}
	if got, want := fmt.Sprint(names), "[contract_sample sample sample_cli]"; got != want {
		t.Fatalf("redundant names = %s, want %s; violations = %#v", got, want, report.Violations)
	}
}

func TestAuditReportsDirectoryNameRepetitionAcrossWordSeparators(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "customid")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "custom_id.go"), []byte("package customid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Audit(root)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(report.Violations) != 1 || report.Violations[0].Kind != RedundantName || report.Violations[0].Name != "custom_id" {
		t.Fatalf("violations = %#v, want compact directory-name repetition", report.Violations)
	}
}
