package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateOpaqueToken returns token and its SHA-256 hash.
func GenerateOpaqueToken() (token string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	hashSum := sha256.Sum256([]byte(token))
	return token, hashSum[:], nil
}
