package pluginhost

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const SignatureSchemaURL = "https://raw.githubusercontent.com/xsyetopz/go-mamacord/refs/heads/main/schemas/signature.schema.v1.json"
const TrustedKeysSchemaURL = "https://raw.githubusercontent.com/xsyetopz/go-mamacord/refs/heads/main/schemas/trusted_keys.schema.v1.json"

func GenerateEd25519Key() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

func DecodeEd25519PrivateKey(raw string) (ed25519.PrivateKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	switch len(decoded) {
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(decoded), nil
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), nil
	default:
		return nil, fmt.Errorf("unexpected private key size %d", len(decoded))
	}
}

func ReadEd25519PrivateKeyFile(path string) (ed25519.PrivateKey, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(string(bytes))
	if raw == "" {
		return nil, errors.New("private key file is empty")
	}

	if strings.HasPrefix(raw, "{") {
		var payload struct {
			PrivateKeyB64 string `json:"private_key_b64"`
		}
		if err := json.Unmarshal(bytes, &payload); err != nil {
			return nil, fmt.Errorf("parse private key file: %w", err)
		}
		raw = strings.TrimSpace(payload.PrivateKeyB64)
	}

	return DecodeEd25519PrivateKey(raw)
}

func WriteEd25519PrivateKeyFile(path string, privateKey ed25519.PrivateKey) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return errors.New("invalid ed25519 private key")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload := strings.TrimSpace(base64.StdEncoding.EncodeToString(privateKey)) + "\n"
	return os.WriteFile(path, []byte(payload), 0o600)
}

func UpsertTrustedKeyFile(path string, key TrustedKey) error {
	if strings.TrimSpace(key.KeyID) == "" {
		return errors.New("trusted key id is required")
	}
	if strings.TrimSpace(key.PublicKeyB64) == "" {
		return errors.New("trusted public key is required")
	}

	payload := TrustedKeys{Keys: []TrustedKey{}}
	if bytes, err := os.ReadFile(path); err == nil {
		if unmarshalErr := json.Unmarshal(bytes, &payload); unmarshalErr != nil {
			return fmt.Errorf("parse trusted keys file: %w", unmarshalErr)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	replaced := false
	for i := range payload.Keys {
		if payload.Keys[i].KeyID != key.KeyID {
			continue
		}
		payload.Keys[i] = key
		replaced = true
	}
	if !replaced {
		payload.Keys = append(payload.Keys, key)
	}
	slices.SortFunc(payload.Keys, func(a, b TrustedKey) int {
		return strings.Compare(a.KeyID, b.KeyID)
	})

	bytes, err := json.MarshalIndent(map[string]any{
		"$schema": TrustedKeysSchemaURL,
		"version": "1",
		"keys":    payload.Keys,
	}, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0o644)
}

func SignDir(dir string, keyID string, privateKey ed25519.PrivateKey) (Signature, ed25519.PublicKey, error) {
	if strings.TrimSpace(dir) == "" {
		return Signature{}, nil, errors.New("plugin dir is required")
	}
	if strings.TrimSpace(keyID) == "" {
		return Signature{}, nil, errors.New("key id is required")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return Signature{}, nil, errors.New("invalid ed25519 private key")
	}

	hash, err := HashDir(dir)
	if err != nil {
		return Signature{}, nil, err
	}

	sig := Signature{
		KeyID:        strings.TrimSpace(keyID),
		HashB64:      base64.StdEncoding.EncodeToString(hash[:]),
		SignatureB64: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, hash[:])),
		Algorithm:    "ed25519-sha256",
	}
	return sig, privateKey.Public().(ed25519.PublicKey), nil
}
