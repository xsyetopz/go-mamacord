package signing_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/signing"
)

func TestSignDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "plugin.star"), []byte("return {}"))

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	sig, publicKey, err := signing.SignDir(dir, "key-1", privateKey)
	if err != nil {
		t.Fatalf("SignDir: %v", err)
	}

	if sig.KeyID != "key-1" {
		t.Fatalf("unexpected key id: %q", sig.KeyID)
	}
	if sig.Algorithm != "ed25519-sha256" {
		t.Fatalf("unexpected algorithm: %q", sig.Algorithm)
	}
	if len(publicKey) != ed25519.PublicKeySize {
		t.Fatalf("unexpected public key size: %d", len(publicKey))
	}
	if err := signing.VerifyDirSignature(dir, sig, map[string]ed25519.PublicKey{"key-1": publicKey}); err != nil {
		t.Fatalf("VerifyDirSignature: %v", err)
	}
}

func TestWriteEd25519PrivateKeyFileAndRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "keys", "owner.key")

	_, privateKey, err := signing.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	if err := signing.WriteEd25519PrivateKeyFile(path, privateKey); err != nil {
		t.Fatalf("WriteEd25519PrivateKeyFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected file mode: %v", info.Mode().Perm())
	}

	readKey, err := signing.ReadEd25519PrivateKeyFile(path)
	if err != nil {
		t.Fatalf("ReadEd25519PrivateKeyFile: %v", err)
	}
	if !reflect.DeepEqual([]byte(readKey), []byte(privateKey)) {
		t.Fatalf("private key round trip mismatch")
	}
}

func TestUpsertTrustedKeyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config", "trusted_keys.json")

	pub1, _, err := signing.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key(1): %v", err)
	}
	pub2, _, err := signing.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key(2): %v", err)
	}

	if err := signing.UpsertTrustedKeyFile(path, signing.TrustedKey{
		KeyID:        "b-key",
		PublicKeyB64: base64.StdEncoding.EncodeToString(pub1),
	}); err != nil {
		t.Fatalf("UpsertTrustedKeyFile(first): %v", err)
	}
	if err := signing.UpsertTrustedKeyFile(path, signing.TrustedKey{
		KeyID:        "a-key",
		PublicKeyB64: base64.StdEncoding.EncodeToString(pub2),
	}); err != nil {
		t.Fatalf("UpsertTrustedKeyFile(second): %v", err)
	}
	if err := signing.UpsertTrustedKeyFile(path, signing.TrustedKey{
		KeyID:        "b-key",
		PublicKeyB64: base64.StdEncoding.EncodeToString(pub2),
	}); err != nil {
		t.Fatalf("UpsertTrustedKeyFile(replace): %v", err)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var payload struct {
		Schema string               `json:"$schema"`
		Keys   []signing.TrustedKey `json:"keys"`
	}
	if err := json.Unmarshal(bytes, &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if payload.Schema != signing.TrustedKeysSchemaURL {
		t.Fatalf("unexpected schema: %q", payload.Schema)
	}
	if len(payload.Keys) != 2 {
		t.Fatalf("unexpected key count: %d", len(payload.Keys))
	}
	if payload.Keys[0].KeyID != "a-key" || payload.Keys[1].KeyID != "b-key" {
		t.Fatalf("unexpected key order: %#v", payload.Keys)
	}
	if payload.Keys[1].PublicKeyB64 != base64.StdEncoding.EncodeToString(pub2) {
		t.Fatalf("expected replacement public key to be persisted")
	}
}
