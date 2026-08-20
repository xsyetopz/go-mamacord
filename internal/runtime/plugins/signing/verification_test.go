package signing_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/signing"
	marketstore "github.com/xsyetopz/go-mamacord/internal/storage/marketplace"
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

	payload := signing.TrustedKeys{
		Schema: signing.TrustedKeysSchemaURL,
		Keys: []signing.TrustedKey{
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

	readKeys, err := signing.ReadTrustedKeysFile(filePath)
	if err != nil {
		t.Fatalf("ReadTrustedKeysFile: %v", err)
	}
	if !reflect.DeepEqual(readKeys["file-key"], filePublicKey) {
		t.Fatalf("unexpected file key bytes")
	}

	loadedKeys, err := signing.LoadTrustedKeys(
		context.Background(),
		filePath,
		trustedSignerStoreStub{
			signers: []marketstore.TrustedSigner{
				{
					KeyID:        "store-key",
					PublicKeyB64: base64.StdEncoding.EncodeToString(storePublicKey),
					AddedAt:      time.Unix(1700000000, 0).UTC(),
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
	mustWriteFile(t, filepath.Join(dir, "plugin.star"), []byte(`return "ok"`))

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	hashBefore, err := bundles.HashDir(dir)
	if err != nil {
		t.Fatalf("HashDir(before): %v", err)
	}

	mustWriteFile(t, filepath.Join(dir, "signature.json"), []byte(`{"ignored":true}`))

	hashAfter, err := bundles.HashDir(dir)
	if err != nil {
		t.Fatalf("HashDir(after): %v", err)
	}
	if hashBefore != hashAfter {
		t.Fatalf("expected signature.json to be ignored by HashDir")
	}

	signatureBytes := ed25519.Sign(privateKey, hashBefore[:])
	signature := signing.Signature{
		Schema:       signing.SignatureSchemaURL,
		KeyID:        "key-1",
		HashB64:      base64.StdEncoding.EncodeToString(hashBefore[:]),
		SignatureB64: base64.StdEncoding.EncodeToString(signatureBytes),
		Algorithm:    "ed25519-sha256",
	}

	keys := map[string]ed25519.PublicKey{"key-1": publicKey}
	if err := signing.VerifyDirSignature(dir, signature, keys); err != nil {
		t.Fatalf("VerifyDirSignature(valid): %v", err)
	}

	if err := signing.VerifyDirSignature(dir, signature, map[string]ed25519.PublicKey{}); err == nil {
		t.Fatalf("expected unknown signer error")
	}

	signature.HashB64 = base64.StdEncoding.EncodeToString([]byte("bad-hash"))
	if err := signing.VerifyDirSignature(dir, signature, keys); err == nil {
		t.Fatalf("expected hash mismatch error")
	}
}

func TestTrackedOfficialPluginSignaturesVerify(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	trustedKeysPath := filepath.Join(repoRoot, "config", "trusted_keys.json")
	keys, err := signing.ReadTrustedKeysFile(trustedKeysPath)
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

			rootDir := filepath.Join(repoRoot, rel)
			dir, err := bundles.NewLocalRepository().ResolveActiveDir(rootDir)
			if err != nil {
				t.Fatalf("ResolveActiveDir(%q): %v", rootDir, err)
			}
			sig, err := signing.ReadSignature(filepath.Join(dir, "signature.json"))
			if err != nil {
				t.Fatalf("ReadSignature(%q): %v", dir, err)
			}
			if err := signing.VerifyDirSignature(dir, sig, keys); err != nil {
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

type trustedSignerStoreStub struct {
	signers []marketstore.TrustedSigner
	err     error
}

func (s trustedSignerStoreStub) ListTrustedSigners(context.Context) ([]marketstore.TrustedSigner, error) {
	return s.signers, s.err
}

func (trustedSignerStoreStub) PutTrustedSigner(context.Context, marketstore.TrustedSigner) error {
	return nil
}

func (trustedSignerStoreStub) DeleteTrustedSigner(context.Context, string) error {
	return nil
}

func TestReadSignatureRejectsNoncanonicalAuthority(t *testing.T) {
	t.Parallel()
	valid := `{"$schema":"` + signing.SignatureSchemaURL + `","key_id":"release-key","hash_b64":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","signature_b64":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==","algorithm":"ed25519-sha256"}`
	cases := map[string]string{"unknown": valid[:len(valid)-1] + `,"extra":true}`, "duplicate": strings.Replace(valid, `"key_id":"release-key"`, `"key_id":"release-key","key_id":"other"`, 1), "missing_hash": strings.Replace(valid, `"hash_b64":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",`, "", 1), "null_algorithm": strings.Replace(valid, `"algorithm":"ed25519-sha256"`, `"algorithm":null`, 1)}
	for name, payload := range cases {
		name, payload := name, payload
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "signature.json")
			if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := signing.ReadSignature(path); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
