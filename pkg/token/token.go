// Package token mints the opaque, high-entropy strings used for refresh tokens
// and email verification links.
//
// Only the SHA-256 hash is ever stored. The plaintext exists exactly once — in
// the HTTP response or the outgoing email — so a database leak yields nothing
// an attacker can present back to the API.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// entropyBytes is 32 (256 bits), well beyond guessing range.
const entropyBytes = 32

// Generate returns the plaintext to hand out and the hash to persist.
func Generate() (plain, hashed string, err error) {
	buf := make([]byte, entropyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("token: read random bytes: %w", err)
	}

	plain = base64.RawURLEncoding.EncodeToString(buf) // URL-safe: goes in email links
	return plain, Hash(plain), nil
}

// Hash is deliberately a plain SHA-256 rather than bcrypt. These values are
// already 256 bits of randomness, so slowing an attacker down buys nothing —
// and a deterministic hash is what makes `WHERE token_hash = ?` an indexed
// lookup. bcrypt salts every row and cannot be queried at all.
// Comparison deliberately has no helper here. Lookups happen in SQL —
// `WHERE token_hash = ?` — so there is nothing for the application to compare,
// and an unused constant-time helper only invites someone to reach for the
// wrong tool later.
func Hash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
