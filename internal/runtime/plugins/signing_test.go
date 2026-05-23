package pluginhost_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

func TestReadTrustedKeysFileAndLoadTrustedKeys(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "trusted_keys.json")

	filePublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey(file): %v", err)
	}
	storePublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey(store): %v", err)
	}

	payload := pluginhost.TrustedKeys{
		Keys: []pluginhost.TrustedKey{
			{
				KeyID:        "file-key",
				PublicKeyB64: base64.StdEncoding.EncodeToString(filePublicKey),
			},
		},
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(filePath, bytes, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	readKeys, err := pluginhost.ReadTrustedKeysFile(filePath)
	if err != nil {
		t.Fatalf("ReadTrustedKeysFile: %v", err)
	}
	if !reflect.DeepEqual(readKeys["file-key"], filePublicKey) {
		t.Fatalf("unexpected file key bytes")
	}

	loadedKeys, err := pluginhost.LoadTrustedKeys(
		context.Background(),
		filePath,
		trustedSignerSourceStub{
			store: trustedSignerStoreStub{
				signers: []store.TrustedSigner{
					{
						KeyID:        "store-key",
						PublicKeyB64: base64.StdEncoding.EncodeToString(storePublicKey),
						AddedAt:      time.Unix(1700000000, 0).UTC(),
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("LoadTrustedKeys: %v", err)
	}
	if len(loadedKeys) != 2 {
		t.Fatalf("unexpected loaded key count: %d", len(loadedKeys))
	}
	if !reflect.DeepEqual(loadedKeys["store-key"], storePublicKey) {
		t.Fatalf("unexpected store key bytes")
	}
}

func TestVerifyDirSignatureAndHashDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "plugin.json"), []byte(`{"id":"example"}`))
	mustWriteFile(t, filepath.Join(dir, "plugin.lua"), []byte(`return "ok"`))

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	hashBefore, err := pluginhost.HashDir(dir)
	if err != nil {
		t.Fatalf("HashDir(before): %v", err)
	}

	mustWriteFile(t, filepath.Join(dir, "signature.json"), []byte(`{"ignored":true}`))

	hashAfter, err := pluginhost.HashDir(dir)
	if err != nil {
		t.Fatalf("HashDir(after): %v", err)
	}
	if hashBefore != hashAfter {
		t.Fatalf("expected signature.json to be ignored by HashDir")
	}

	signatureBytes := ed25519.Sign(privateKey, hashBefore[:])
	signature := pluginhost.Signature{
		KeyID:        "key-1",
		HashB64:      base64.StdEncoding.EncodeToString(hashBefore[:]),
		SignatureB64: base64.StdEncoding.EncodeToString(signatureBytes),
		Algorithm:    "ed25519-sha256",
	}

	keys := map[string]ed25519.PublicKey{"key-1": publicKey}
	if err := pluginhost.VerifyDirSignature(dir, signature, keys); err != nil {
		t.Fatalf("VerifyDirSignature(valid): %v", err)
	}

	if err := pluginhost.VerifyDirSignature(dir, signature, map[string]ed25519.PublicKey{}); err == nil {
		t.Fatalf("expected unknown signer error")
	}

	signature.HashB64 = base64.StdEncoding.EncodeToString([]byte("bad-hash"))
	if err := pluginhost.VerifyDirSignature(dir, signature, keys); err == nil {
		t.Fatalf("expected hash mismatch error")
	}
}

func TestSignDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "plugin.lua"), []byte("return {}"))

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	sig, publicKey, err := pluginhost.SignDir(dir, "key-1", privateKey)
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
	if err := pluginhost.VerifyDirSignature(dir, sig, map[string]ed25519.PublicKey{"key-1": publicKey}); err != nil {
		t.Fatalf("VerifyDirSignature: %v", err)
	}
}

func TestWriteEd25519PrivateKeyFileAndRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "keys", "owner.key")

	_, privateKey, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	if err := pluginhost.WriteEd25519PrivateKeyFile(path, privateKey); err != nil {
		t.Fatalf("WriteEd25519PrivateKeyFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected file mode: %v", info.Mode().Perm())
	}

	readKey, err := pluginhost.ReadEd25519PrivateKeyFile(path)
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

	pub1, _, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key(1): %v", err)
	}
	pub2, _, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key(2): %v", err)
	}

	if err := pluginhost.UpsertTrustedKeyFile(path, pluginhost.TrustedKey{
		KeyID:        "b-key",
		PublicKeyB64: base64.StdEncoding.EncodeToString(pub1),
	}); err != nil {
		t.Fatalf("UpsertTrustedKeyFile(first): %v", err)
	}
	if err := pluginhost.UpsertTrustedKeyFile(path, pluginhost.TrustedKey{
		KeyID:        "a-key",
		PublicKeyB64: base64.StdEncoding.EncodeToString(pub2),
	}); err != nil {
		t.Fatalf("UpsertTrustedKeyFile(second): %v", err)
	}
	if err := pluginhost.UpsertTrustedKeyFile(path, pluginhost.TrustedKey{
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
		Schema  string                  `json:"$schema"`
		Version string                  `json:"version"`
		Keys    []pluginhost.TrustedKey `json:"keys"`
	}
	if err := json.Unmarshal(bytes, &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if payload.Schema != pluginhost.TrustedKeysSchemaURL {
		t.Fatalf("unexpected schema: %q", payload.Schema)
	}
	if payload.Version != "1" {
		t.Fatalf("unexpected version: %q", payload.Version)
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

func TestTrackedOfficialPluginSignaturesVerify(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	trustedKeysPath := filepath.Join(repoRoot, "config", "trusted_keys.json")
	keys, err := pluginhost.ReadTrustedKeysFile(trustedKeysPath)
	if err != nil {
		t.Fatalf("ReadTrustedKeysFile(%q): %v", trustedKeysPath, err)
	}

	for _, rel := range []string{
		"plugins/fun",
		"plugins/info",
		"plugins/manager",
		"plugins/moderation",
		"plugins/wellness",
	} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(repoRoot, rel)
			sig, err := pluginhost.ReadSignature(filepath.Join(dir, "signature.json"))
			if err != nil {
				t.Fatalf("ReadSignature(%q): %v", dir, err)
			}
			if err := pluginhost.VerifyDirSignature(dir, sig, keys); err != nil {
				t.Fatalf("VerifyDirSignature(%q): %v", dir, err)
			}
		})
	}
}

func mustWriteFile(t *testing.T, path string, bytes []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, bytes, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

type trustedSignerSourceStub struct {
	store trustedSignerStoreStub
}

func (s trustedSignerSourceStub) TrustedSigners() store.TrustedSignerStore {
	return s.store
}

type trustedSignerStoreStub struct {
	signers []store.TrustedSigner
	err     error
}

func (s trustedSignerStoreStub) ListTrustedSigners(context.Context) ([]store.TrustedSigner, error) {
	return s.signers, s.err
}

func (trustedSignerStoreStub) PutTrustedSigner(context.Context, store.TrustedSigner) error {
	return nil
}

func (trustedSignerStoreStub) DeleteTrustedSigner(context.Context, string) error {
	return nil
}
