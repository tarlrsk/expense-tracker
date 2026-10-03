package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// tokenBytes is the size of a session or link token (ADR-0066).
const tokenBytes = 32

// Token is a new secret token: Plain goes to the client once, Hash is what the database stores.
type Token struct {
	Plain string
	Hash  []byte
}

// NewToken makes a session or link token: 32 bytes from crypto/rand, sent as unpadded base64url
// (43 characters). Only its SHA-256 hash is stored (ADR-0066).
func NewToken() Token {
	raw := make([]byte, tokenBytes)
	// crypto/rand.Read never returns an error; it crashes the program instead.
	_, _ = rand.Read(raw)
	sum := sha256.Sum256(raw)
	return Token{Plain: base64.RawURLEncoding.EncodeToString(raw), Hash: sum[:]}
}

// HashToken returns the stored form of a token a client sent: the SHA-256 hash of its 32 bytes.
// ok is false when plain is not a well-formed token (wrong length or alphabet); such a token
// matches nothing and is rejected like an unknown one.
func HashToken(plain string) (hash []byte, ok bool) {
	if len(plain) != base64.RawURLEncoding.EncodedLen(tokenBytes) {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(plain)
	if err != nil || len(raw) != tokenBytes {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	return sum[:], true
}
