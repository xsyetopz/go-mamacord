package buildinfo

import "testing"

func TestCurrentNormalizesUnknownValues(t *testing.T) {

	prevVersion := Version
	prevRepository := Repository
	prevDescription := Description
	prevDescriptionBase64 := DescriptionBase64
	prevDeveloperURL := DeveloperURL
	prevSupportServerURL := SupportServerURL
	prevMascotImageURL := MascotImageURL
	defer func() {
		Version = prevVersion
		Repository = prevRepository
		Description = prevDescription
		DescriptionBase64 = prevDescriptionBase64
		DeveloperURL = prevDeveloperURL
		SupportServerURL = prevSupportServerURL
		MascotImageURL = prevMascotImageURL
	}()

	Version = "1.2.3"
	Repository = " UNKNOWN "
	Description = " test build "
	DeveloperURL = "https://example.com/dev"
	SupportServerURL = "UNKNOWN"
	MascotImageURL = " https://example.com/mascot.png "

	info := Current()
	if info.Version != "1.2.3" {
		t.Fatalf("unexpected version: %q", info.Version)
	}
	if info.Repository != "" {
		t.Fatalf("expected empty repository, got %q", info.Repository)
	}
	if info.Description != "test build" {
		t.Fatalf("unexpected description: %q", info.Description)
	}
	if info.DeveloperURL != "https://example.com/dev" {
		t.Fatalf("unexpected developer url: %q", info.DeveloperURL)
	}
	if info.SupportServerURL != "" {
		t.Fatalf("expected empty support url, got %q", info.SupportServerURL)
	}
	if info.MascotImageURL != "https://example.com/mascot.png" {
		t.Fatalf("unexpected mascot image url: %q", info.MascotImageURL)
	}
}

func TestCurrentUsesBase64DescriptionOverride(t *testing.T) {

	prevDescription := Description
	prevDescriptionBase64 := DescriptionBase64
	defer func() {
		Description = prevDescription
		DescriptionBase64 = prevDescriptionBase64
	}()

	Description = "fallback"
	DescriptionBase64 = "IGVuY29kZWQgZGVzY3JpcHRpb24g"

	info := Current()
	if info.Description != "encoded description" {
		t.Fatalf("unexpected description: %q", info.Description)
	}
}

func TestCurrentFallsBackWhenBase64DescriptionInvalid(t *testing.T) {

	prevDescription := Description
	prevDescriptionBase64 := DescriptionBase64
	defer func() {
		Description = prevDescription
		DescriptionBase64 = prevDescriptionBase64
	}()

	Description = " fallback description "
	DescriptionBase64 = "not-base64"

	info := Current()
	if info.Description != "fallback description" {
		t.Fatalf("unexpected description: %q", info.Description)
	}
}
