package dotenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFile_DoesNotOverride(t *testing.T) {
	keyA := "DOTENV_TEST_A_NO_OVERRIDE"
	keyB := "DOTENV_TEST_B_SET"
	t.Setenv(keyA, "already")
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(keyA+"=fromfile\n"+keyB+"=ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := LoadFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(keyA); got != "already" {
		t.Fatalf("%s=%q, want %q", keyA, got, "already")
	}
	if got := os.Getenv(keyB); got != "ok" {
		t.Fatalf("%s=%q, want %q", keyB, got, "ok")
	}
}

func TestLoadFile_ParsesQuotesExportAndComments(t *testing.T) {
	keyA := "DOTENV_TEST_A_EXPORT"
	keyB := "DOTENV_TEST_B_DOUBLE"
	keyC := "DOTENV_TEST_C_SINGLE"
	keyD := "DOTENV_TEST_D_COMMENT"
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := strings.Join([]string{
		"# comment",
		"  ",
		"export " + keyA + "=1",
		keyB + `="hello # not comment"`,
		keyC + `='world # not comment'`,
		keyD + "=value # trailing comment",
	}, "\n")
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := LoadFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(keyA); got != "1" {
		t.Fatalf("%s=%q, want %q", keyA, got, "1")
	}
	if got := os.Getenv(keyB); got != "hello # not comment" {
		t.Fatalf("%s=%q, want %q", keyB, got, "hello # not comment")
	}
	if got := os.Getenv(keyC); got != "world # not comment" {
		t.Fatalf("%s=%q, want %q", keyC, got, "world # not comment")
	}
	if got := os.Getenv(keyD); got != "value" {
		t.Fatalf("%s=%q, want %q", keyD, got, "value")
	}
}

func TestLoadAutoWithSearch_FallsBackToExecutableDir(t *testing.T) {
	workDir := t.TempDir()
	execDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(execDir, ".env.prod"), []byte("DOTENV_TEST_SEARCH=prod\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := LoadAutoWithSearch(
		[]string{".env.prod", ".env.dev"},
		[]SearchResult{
			{Path: workDir, Source: "working_dir"},
			{Path: execDir, Source: "executable_dir"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if got := os.Getenv("DOTENV_TEST_SEARCH"); got != "prod" {
		t.Fatalf("DOTENV_TEST_SEARCH=%q, want %q", got, "prod")
	}
	if res.Source != "executable_dir" {
		t.Fatalf("unexpected source: %q", res.Source)
	}
	if res.Path != filepath.Join(execDir, ".env.prod") {
		t.Fatalf("unexpected path: %q", res.Path)
	}
}
