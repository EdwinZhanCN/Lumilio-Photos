package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"server/platform/fsprivacy"
)

// Box encrypts and decrypts small application secrets with a scoped key.
type Box struct {
	key []byte
}

// New creates a Box from the configured Lumilio secret key file and scope.
func New(configuredPath string, scope string) (*Box, error) {
	rootSecret, err := LoadOrCreateLumilioSecretKey(configuredPath)
	if err != nil {
		return nil, err
	}
	return &Box{key: DeriveScopedSecret(rootSecret, scope)}, nil
}

// NewFromRoot creates a Box from an already loaded root secret and scope.
func NewFromRoot(rootSecret string, scope string) *Box {
	return &Box{key: DeriveScopedSecret(rootSecret, scope)}
}

// NewWithKey creates a Box from an already derived 32-byte AES key.
func NewWithKey(key []byte) *Box {
	copied := make([]byte, len(key))
	copy(copied, key)
	return &Box{key: copied}
}

// Key returns a copy of the scoped encryption key.
func (b *Box) Key() []byte {
	key := make([]byte, len(b.key))
	copy(key, b.key)
	return key
}

// Encrypt seals plaintext using AES-GCM and prefixes the nonce.
func (b *Box) Encrypt(plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt opens ciphertext produced by Encrypt.
func (b *Box) Decrypt(ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	if len(ciphertext) < aead.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}

	nonce := ciphertext[:aead.NonceSize()]
	payload := ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

// LoadOrCreateLumilioSecretKey loads the root secret from a file path, creating
// it on first boot. The env/config value must be a path, never raw secret text.
func LoadOrCreateLumilioSecretKey(configuredPath string) (string, error) {
	keyFile := strings.TrimSpace(configuredPath)
	if keyFile == "" {
		return "", errors.New("secret key file path is required")
	}
	if !filepath.IsAbs(keyFile) {
		return "", errors.New("secret key file path must be absolute")
	}
	keyFile = filepath.Clean(keyFile)

	content, err := os.ReadFile(keyFile)
	switch {
	case err == nil:
		secret := strings.TrimSpace(string(content))
		if secret == "" {
			return "", fmt.Errorf("LUMILIO secret key file is empty: %s", keyFile)
		}
		if err := fsprivacy.ApplyFileMode(keyFile, 0o600); err != nil {
			return "", fmt.Errorf("protect LUMILIO secret key: %w", err)
		}
		return secret, nil
	case errors.Is(err, os.ErrNotExist):
	default:
		return "", fmt.Errorf("read LUMILIO secret key file %s: %w", keyFile, err)
	}

	if err := os.MkdirAll(filepath.Dir(keyFile), 0o700); err != nil {
		return "", fmt.Errorf("create secret key directory: %w", err)
	}
	if err := fsprivacy.ApplyDirectoryMode(filepath.Dir(keyFile), 0o700); err != nil {
		return "", fmt.Errorf("protect secret key directory: %w", err)
	}

	random := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", fmt.Errorf("generate LUMILIO secret key: %w", err)
	}

	secret := fmt.Sprintf("%x", random)
	if err := os.WriteFile(keyFile, []byte(secret+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("persist LUMILIO secret key: %w", err)
	}
	if err := fsprivacy.ApplyFileMode(keyFile, 0o600); err != nil {
		return "", fmt.Errorf("protect LUMILIO secret key: %w", err)
	}

	return secret, nil
}

// DeriveScopedSecret derives a 32-byte scoped secret from the root key.
func DeriveScopedSecret(rootSecret string, scope string) []byte {
	sum := sha256.Sum256([]byte(scope + "\x00" + rootSecret))
	derived := make([]byte, len(sum))
	copy(derived, sum[:])
	return derived
}
