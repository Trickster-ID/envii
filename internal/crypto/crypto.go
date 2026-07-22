// Package crypto handles encryption of the vault at rest using a
// passphrase-derived key (age scrypt recipient).
package crypto

import (
	"bytes"
	"fmt"
	"io"

	"filippo.io/age"
)

// workFactor is the scrypt log2 work factor used for vault encryption.
var workFactor = 15

// Test seams (production defaults). Override in tests to force error paths.
var (
	newScryptRecipient = age.NewScryptRecipient
	newScryptIdentity  = age.NewScryptIdentity
	ageEncrypt         = age.Encrypt
	ageDecrypt         = age.Decrypt
)

// SetWorkFactor overrides the scrypt work factor. Intended for tests that
// need fast encryption; production code should leave the default.
func SetWorkFactor(n int) { workFactor = n }

// Encrypt encrypts plaintext with the given passphrase, returning the
// armored age ciphertext bytes.
func Encrypt(plaintext []byte, passphrase string) ([]byte, error) {
	recipient, err := newScryptRecipient(passphrase)
	if err != nil {
		return nil, fmt.Errorf("create recipient: %w", err)
	}
	// Work factor 2^15 (~1s) balances UX against brute-force resistance.
	// age's default of 2^18 is too slow for an interactive prompt.
	recipient.SetWorkFactor(workFactor)

	var buf bytes.Buffer
	w, err := ageEncrypt(&buf, recipient)
	if err != nil {
		return nil, fmt.Errorf("init encrypt: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("write plaintext: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}
	return buf.Bytes(), nil
}

// Decrypt decrypts age ciphertext with the given passphrase.
func Decrypt(ciphertext []byte, passphrase string) ([]byte, error) {
	identity, err := newScryptIdentity(passphrase)
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}

	r, err := ageDecrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return nil, fmt.Errorf("init decrypt (wrong passphrase?): %w", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read plaintext: %w", err)
	}
	return out, nil
}
