package starlark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalBundleLabel(t *testing.T) {
	t.Parallel()
	valid := []string{"//:plugin.star", "//commands:flip.star", "//lib/shared:format.star"}
	for _, label := range valid {
		if got, err := CanonicalBundleLabel(label); err != nil || got != label {
			t.Errorf("CanonicalBundleLabel(%q) = %q, %v", label, got, err)
		}
	}
	invalid := []string{
		"", "plugin.star", "/:plugin.star", "///:plugin.star", "//plugin.star", "//:plugin.lua",
		"//:../plugin.star", "//../outside:plugin.star", "//commands/../lib:x.star", "//.hidden:x.star",
		"//commands:.hidden.star", "//commands/sub:bad/name.star", "//commands:x.star:extra", APIModuleLabel,
	}
	for _, label := range invalid {
		if _, err := CanonicalBundleLabel(label); err == nil {
			t.Errorf("CanonicalBundleLabel(%q) unexpectedly succeeded", label)
		}
	}
}

func TestDirBundleReadSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.star"), []byte("PLUGIN = None\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lib", "helper.star"), []byte("VALUE = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bundle, err := OpenDirBundle(root)
	if err != nil {
		t.Fatalf("OpenDirBundle: %v", err)
	}
	data, err := bundle.ReadSource("//:plugin.star", 256)
	if err != nil || string(data) != "PLUGIN = None\n" {
		t.Fatalf("ReadSource root = %q, %v", data, err)
	}
	data, err = bundle.ReadSource("//lib:helper.star", 256)
	if err != nil || string(data) != "VALUE = 1\n" {
		t.Fatalf("ReadSource nested = %q, %v", data, err)
	}
	if _, err := bundle.ReadSource("//:plugin.star", 4); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size error, got %v", err)
	}
}

func TestDirBundleRejectsInvalidUTF8AndSymlinks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "invalid.star"), []byte{0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.star")
	if err := os.WriteFile(outside, []byte("VALUE = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.star")); err != nil {
		t.Fatal(err)
	}
	bundle, err := OpenDirBundle(root)
	if err != nil {
		t.Fatalf("OpenDirBundle: %v", err)
	}
	if _, err := bundle.ReadSource("//:invalid.star", 256); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("expected UTF-8 error, got %v", err)
	}
	if _, err := bundle.ReadSource("//:linked.star", 256); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
}

func TestLimitsValidate(t *testing.T) {
	t.Parallel()
	limits := DefaultLimits()
	if err := limits.Validate(); err != nil {
		t.Fatalf("default limits: %v", err)
	}
	limits.MaxTotalSourceBytes = limits.MaxFileBytes - 1
	if err := limits.Validate(); err == nil {
		t.Fatal("accepted total source limit below file limit")
	}
}
