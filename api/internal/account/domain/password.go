package domain

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

// Password length in characters (runes), ADR-0037 and ADR-0066. No composition rules.
const (
	MinPasswordLength = 10
	MaxPasswordLength = 128
)

// PasswordRuleMessage is the client message when a new password breaks the length rule.
const PasswordRuleMessage = "the password must be 10 to 128 characters long"

// CheckPasswordRule returns an invalid_input error unless pw is 10 to 128 characters long.
// It applies to new passwords only; a login checks the password it is given against the stored
// hash, whatever its length.
func CheckPasswordRule(pw string) error {
	if n := utf8.RuneCountInString(pw); n < MinPasswordLength || n > MaxPasswordLength {
		return apperr.New(apperr.InvalidInput, PasswordRuleMessage)
	}
	return nil
}

// HashParams are the argon2id parameters (ADR-0066).
type HashParams struct {
	// Memory in KiB.
	Memory uint32
	// Time is the number of passes.
	Time uint32
	// Threads is the parallelism.
	Threads uint8
	// SaltLength and KeyLength in bytes.
	SaltLength uint32
	KeyLength  uint32
}

// DefaultHashParams are the production parameters: 19 MiB, 2 passes, 1 thread (the OWASP
// minimum), a 16-byte salt and a 32-byte key.
var DefaultHashParams = HashParams{Memory: 19 * 1024, Time: 2, Threads: 1, SaltLength: 16, KeyLength: 32}

// Limits on the parameters read back from a stored hash, so a damaged row cannot make one
// verification take unbounded memory or time.
const (
	maxMemoryKiB = 1 << 20 // 1 GiB
	maxTime      = 100
	maxKeyLength = 1024
)

// ErrBadHash: a stored hash is not an argon2id string this package can read.
var ErrBadHash = errors.New("the stored password hash is not a readable argon2id string")

// Hasher hashes and verifies passwords with argon2id. Hashes are stored in the standard string
// form $argon2id$v=19$m=19456,t=2,p=1$<salt>$<key> (unpadded standard base64), so the parameters
// can be raised later: verification reads them from the stored string.
//
// Hashing is CPU and memory work: never call it inside a database transaction (ADR-0066).
type Hasher struct {
	params HashParams
	// dummy is a hash of a random password, made once with the same parameters. VerifyOrDummy
	// checks against it when there is no real hash, so every failed login costs the same work.
	dummy string
}

// NewHasher returns a Hasher using p for new hashes. It makes the dummy hash at once.
func NewHasher(p HashParams) *Hasher {
	h := &Hasher{params: p}
	h.dummy = h.Hash(rand.Text())
	return h
}

// Hash returns the encoded argon2id hash of pw with a fresh random salt.
func (h *Hasher) Hash(pw string) string {
	salt := make([]byte, h.params.SaltLength)
	// crypto/rand.Read never returns an error; it crashes the program instead.
	_, _ = rand.Read(salt)
	key := argon2.IDKey([]byte(pw), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLength)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.params.Memory, h.params.Time, h.params.Threads, b64.EncodeToString(salt), b64.EncodeToString(key))
}

// Verify reports whether pw matches encoded, in constant time. It reads the parameters from
// encoded and fails with ErrBadHash when encoded cannot be read.
func (h *Hasher) Verify(encoded, pw string) (bool, error) {
	p, salt, key, err := decode(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(pw), salt, p.Time, p.Memory, p.Threads, p.KeyLength)
	return subtle.ConstantTimeCompare(got, key) == 1, nil
}

// VerifyOrDummy is Verify, except that an empty encoded (no account, or an invite not yet
// accepted) is checked against the dummy hash and reports false: the work is the same either way.
func (h *Hasher) VerifyOrDummy(encoded, pw string) (bool, error) {
	if encoded == "" {
		_, err := h.Verify(h.dummy, pw)
		return false, err
	}
	return h.Verify(encoded, pw)
}

// decode parses $argon2id$v=19$m=..,t=..,p=..$salt$key.
func decode(encoded string) (HashParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return HashParams{}, nil, nil, ErrBadHash
	}
	if parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return HashParams{}, nil, nil, fmt.Errorf("%w: unsupported version", ErrBadHash)
	}
	var p HashParams
	var threads uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &threads); err != nil {
		return HashParams{}, nil, nil, fmt.Errorf("%w: parameters", ErrBadHash)
	}
	if p.Memory == 0 || p.Memory > maxMemoryKiB || p.Time == 0 || p.Time > maxTime || threads == 0 || threads > 255 {
		return HashParams{}, nil, nil, fmt.Errorf("%w: parameters out of range", ErrBadHash)
	}
	p.Threads = uint8(threads)
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return HashParams{}, nil, nil, fmt.Errorf("%w: salt", ErrBadHash)
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) == 0 || len(key) > maxKeyLength {
		return HashParams{}, nil, nil, fmt.Errorf("%w: key", ErrBadHash)
	}
	p.SaltLength, p.KeyLength = uint32(len(salt)), uint32(len(key)) //nolint:gosec // both bounded above
	return p, salt, key, nil
}
