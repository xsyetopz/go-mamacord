package pluginhost

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"regexp"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

var signerKeyIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Signature struct {
	Schema       string `json:"$schema"`
	KeyID        string `json:"key_id"`
	HashB64      string `json:"hash_b64"`
	SignatureB64 string `json:"signature_b64"`
	Algorithm    string `json:"algorithm"`
}

type TrustedKeys struct {
	Schema string       `json:"$schema"`
	Keys   []TrustedKey `json:"keys"`
}

type TrustedKey struct {
	KeyID        string `json:"key_id"`
	PublicKeyB64 string `json:"public_key_b64"`
}

func ReadTrustedKeysFile(path string) (map[string]ed25519.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := rejectDuplicateJSONKeys(b); err != nil {
		return nil, fmt.Errorf("parse trusted keys file: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	var file TrustedKeys
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("parse trusted keys file: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("trusted keys file contains trailing data")
	}
	if file.Schema != TrustedKeysSchemaURL || file.Keys == nil || len(file.Keys) > 1000 {
		return nil, errors.New("trusted keys file has invalid authority metadata")
	}
	out := map[string]ed25519.PublicKey{}
	for _, k := range file.Keys {
		if !signerKeyIDPattern.MatchString(k.KeyID) || k.PublicKeyB64 == "" {
			return nil, errors.New("trusted key is invalid")
		}
		if _, exists := out[k.KeyID]; exists {
			return nil, fmt.Errorf("duplicate trusted key %q", k.KeyID)
		}
		pub, pubErr := decodeEd25519PublicKey(k.PublicKeyB64)
		if pubErr != nil {
			return nil, fmt.Errorf("decode trusted key %q: %w", k.KeyID, pubErr)
		}
		out[k.KeyID] = pub
	}

	return out, nil
}

func LoadTrustedKeys(
	ctx context.Context,
	filePath string,
	signers store.TrustedSignerStore,
) (map[string]ed25519.PublicKey, error) {
	out := map[string]ed25519.PublicKey{}

	if strings.TrimSpace(filePath) != "" {
		if keys, err := ReadTrustedKeysFile(filePath); err == nil {
			maps.Copy(out, keys)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if signers != nil {
		trustedSigners, err := signers.ListTrustedSigners(ctx)
		if err != nil {
			return nil, err
		}
		if len(trustedSigners) > 1000 {
			return nil, errors.New("too many trusted signers")
		}
		for _, signer := range trustedSigners {
			if !signerKeyIDPattern.MatchString(signer.KeyID) || signer.PublicKeyB64 == "" {
				return nil, errors.New("stored trusted signer is invalid")
			}
			pub, pubErr := decodeEd25519PublicKey(signer.PublicKeyB64)
			if pubErr != nil {
				return nil, fmt.Errorf("decode signer %q: %w", signer.KeyID, pubErr)
			}
			if existing, found := out[signer.KeyID]; found && !bytes.Equal(existing, pub) {
				return nil, fmt.Errorf("conflicting trusted signer %q", signer.KeyID)
			}
			out[signer.KeyID] = pub
		}
	}

	return out, nil
}

func ReadSignature(path string) (Signature, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Signature{}, err
	}
	return ParseSignature(b)
}
func ParseSignature(data []byte) (Signature, error) {
	if len(data) == 0 || len(data) > 16*1024 {
		return Signature{}, errors.New("signature file size is invalid")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return Signature{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var signature Signature
	if err := decoder.Decode(&signature); err != nil {
		return Signature{}, fmt.Errorf("parse signature: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Signature{}, errors.New("signature contains trailing data")
	}
	if err := signature.Validate(); err != nil {
		return Signature{}, err
	}
	return signature, nil
}
func (sig Signature) Validate() error {
	if sig.Schema != SignatureSchemaURL {
		return errors.New("signature schema is invalid")
	}
	if !signerKeyIDPattern.MatchString(sig.KeyID) {
		return errors.New("signature key_id is invalid")
	}
	if sig.Algorithm != "ed25519-sha256" {
		return errors.New("signature algorithm is invalid")
	}
	hash, err := base64.StdEncoding.DecodeString(sig.HashB64)
	if err != nil || len(hash) != 32 {
		return errors.New("signature hash_b64 is invalid")
	}
	raw, err := base64.StdEncoding.DecodeString(sig.SignatureB64)
	if err != nil || len(raw) != ed25519.SignatureSize {
		return errors.New("signature signature_b64 is invalid")
	}
	return nil
}

func VerifyDirSignature(dir string, sig Signature, keys map[string]ed25519.PublicKey) error {
	if err := sig.Validate(); err != nil {
		return err
	}
	pub, ok := keys[sig.KeyID]
	if !ok {
		return fmt.Errorf("unknown signer key_id %q", sig.KeyID)
	}

	hash, err := bundles.HashDir(dir)
	if err != nil {
		return err
	}

	expected, _ := base64.StdEncoding.DecodeString(sig.HashB64)
	if !bytes.Equal(expected, hash[:]) {
		return errors.New("signature hash mismatch")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sig.SignatureB64)
	if err != nil {
		return fmt.Errorf("decode signature_b64: %w", err)
	}

	if !ed25519.Verify(pub, hash[:], sigBytes) {
		return errors.New("invalid signature")
	}

	return nil
}
func decodeEd25519PublicKey(b64 string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("unexpected public key size %d", len(b))
	}
	return ed25519.PublicKey(b), nil
}
