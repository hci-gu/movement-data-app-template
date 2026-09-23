package study

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

func randomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func hash(value string) string { s := sha256.Sum256([]byte(value)); return hex.EncodeToString(s[:]) }

// Associated data prevents ciphertext from being moved to another record/purpose.
func (c Config) Seal(purpose string, value any) (string, error) {
	block, err := aes.NewCipher(c.EncryptionKeys[c.ActiveKey])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, plain, []byte(purpose))
	return c.ActiveKey + ":" + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (c Config) Open(purpose, value string, target any) error {
	id, encoded, ok := strings.Cut(value, ":")
	if !ok {
		return errors.New("invalid encrypted value")
	}
	block, err := aes.NewCipher(c.EncryptionKeys[id])
	if err != nil {
		return errors.New("encrypted value requires an unavailable key")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(raw) < aead.NonceSize() {
		return errors.New("invalid encrypted value")
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte(purpose))
	if err != nil {
		return errors.New("evidence authentication failed")
	}
	return json.Unmarshal(plain, target)
}
