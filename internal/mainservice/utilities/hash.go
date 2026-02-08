package utilities

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken creates a SHA-256 hash of a token string.
// Used to store refresh tokens securely in the database.
func HashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}
