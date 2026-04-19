package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken applies SHA-256 to a plain-text token and returns the hex-encoded string.
// This ensures that actual refresh tokens are never stored as plain-text in the database.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
