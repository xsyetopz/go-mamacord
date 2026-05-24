package pluginhost

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Signature struct {
	KeyID        string `json:"key_id"`
	HashB64      string `json:"hash_b64"`
	SignatureB64 string `json:"signature_b64"`
	Algorithm    string `json:"algorithm"`
}

type TrustedKeys struct {
	Keys []TrustedKey `json:"keys"`
}

type TrustedKey struct {
	KeyID        string `json:"key_id"`
	PublicKeyB64 string `json:"public_key_b64"`
}

type TrustedSignerSource interface {
	TrustedSigners() store.TrustedSignerStore
}

func ReadTrustedKeysFile(path string) (map[string]ed25519.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var file TrustedKeys
	if unmarshalErr := json.Unmarshal(b, &file); unmarshalErr != nil {
		return nil, fmt.Errorf("parse trusted keys file: %w", unmarshalErr)
	}

	out := map[string]ed25519.PublicKey{}
	for _, k := range file.Keys {
		if k.KeyID == "" || k.PublicKeyB64 == "" {
			continue
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
	src TrustedSignerSource,
) (map[string]ed25519.PublicKey, error) {
	out := map[string]ed25519.PublicKey{}

	if strings.TrimSpace(filePath) != "" {
		if keys, err := ReadTrustedKeysFile(filePath); err == nil {
			maps.Copy(out, keys)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if src != nil {
		signers, err := src.TrustedSigners().ListTrustedSigners(ctx)
		if err != nil {
			return nil, err
		}
		for _, signer := range signers {
			if signer.KeyID == "" || signer.PublicKeyB64 == "" {
				continue
			}
			pub, pubErr := decodeEd25519PublicKey(signer.PublicKeyB64)
			if pubErr != nil {
				return nil, fmt.Errorf("decode signer %q: %w", signer.KeyID, pubErr)
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

	var sig Signature
	if unmarshalErr := json.Unmarshal(b, &sig); unmarshalErr != nil {
		return Signature{}, fmt.Errorf("parse signature: %w", unmarshalErr)
	}

	return sig, nil
}

func VerifyDirSignature(dir string, sig Signature, keys map[string]ed25519.PublicKey) error {
	pub, ok := keys[sig.KeyID]
	if !ok {
		return fmt.Errorf("unknown signer key_id %q", sig.KeyID)
	}

	if sig.Algorithm != "" && sig.Algorithm != "ed25519-sha256" {
		return fmt.Errorf("unsupported signature algorithm %q", sig.Algorithm)
	}

	hash, err := bundles.HashDir(dir)
	if err != nil {
		return err
	}

	if sig.HashB64 != "" {
		expected, decodeErr := base64.StdEncoding.DecodeString(sig.HashB64)
		if decodeErr != nil {
			return fmt.Errorf("decode hash_b64: %w", decodeErr)
		}
		if !bytes.Equal(expected, hash[:]) {
			return errors.New("signature hash mismatch")
		}
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
