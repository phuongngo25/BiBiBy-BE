package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const AADVersion = "v1"

// BuildAAD creates a versioned Contextual AAD string bound to the user.
func BuildAAD(userID string) []byte {
	return []byte(fmt.Sprintf("aad:%s:user:%s", AADVersion, userID))
}

// Encrypt encrypts a plaintext using AES-256-GCM with AAD and prefixes the version.
func Encrypt(plaintext string, key string, aad []byte, version string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), aad)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	
	// Prefix with version for future-proofing and keyring lookup
	return fmt.Sprintf("%s:%s", version, encoded), nil
}

// Decrypt decrypts a versioned ciphertext using the provided keyring and AAD.
// Returns (plaintext, needsRotation, error).
func Decrypt(ciphertext string, keyring map[string]string, activeVersion string, aad []byte) (string, bool, error) {
	if ciphertext == "" {
		return "", false, nil
	}

	parts := strings.SplitN(ciphertext, ":", 2)
	if len(parts) != 2 {
		// Legacy plain-text or malformed. Return as-is but flag for rotation.
		return ciphertext, true, nil
	}

	version := parts[0]
	data := parts[1]

	key, ok := keyring[version]
	if !ok {
		return "", false, fmt.Errorf("decryption key version '%s' not found in keyring", version)
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", false, err
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", false, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", false, err
	}

	nonceSize := gcm.NonceSize()
	if len(decoded) < nonceSize {
		return "", false, errors.New("ciphertext too short")
	}

	nonce, encryptedPayload := decoded[:nonceSize], decoded[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encryptedPayload, aad)
	if err != nil {
		return "", false, err
	}

	needsRotation := version != activeVersion
	return string(plaintext), needsRotation, nil
}
