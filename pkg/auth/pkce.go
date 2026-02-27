package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCEParams holds a PKCE code verifier and its S256 challenge.
type PKCEParams struct {
	Verifier  string
	Challenge string
}

// GeneratePKCE generates a PKCE code verifier (32 random bytes, base64url)
// and its SHA-256 challenge (base64url, no padding).
func GeneratePKCE() (*PKCEParams, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}

	verifier := base64.RawURLEncoding.EncodeToString(buf)

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &PKCEParams{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}
