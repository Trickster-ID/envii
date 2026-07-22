// Package store persists the encrypted vault to disk.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/trickylab/envii/internal/crypto"
	"github.com/trickylab/envii/internal/model"
)

// ErrNotFound is returned when no vault file exists yet.
var ErrNotFound = errors.New("vault not found")

// Store reads and writes the encrypted vault at Path.
type Store struct {
	Path string
	fs   FS
}

// New returns a Store at the given path. If path is empty, it defaults
// to $XDG_CONFIG_HOME/envii/vault.age (or ~/.config/envii/vault.age).
// Optional Option values configure FS or override path.
func New(path string, opts ...Option) (*Store, error) {
	s := &Store{Path: path, fs: OSFS{}}
	for _, opt := range opts {
		opt(s)
	}
	if s.fs == nil {
		s.fs = OSFS{}
	}
	if s.Path == "" {
		dir, err := s.fs.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("resolve config dir: %w", err)
		}
		s.Path = filepath.Join(dir, "envii", "vault.age")
	}
	return s, nil
}

// Exists reports whether a vault file is present on disk.
func (s *Store) Exists() bool {
	_, err := s.fs.Stat(s.Path)
	return err == nil
}

// Load decrypts and parses the vault using the passphrase.
func (s *Store) Load(passphrase string) (*model.Vault, error) {
	data, err := s.fs.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read vault: %w", err)
	}

	plain, err := crypto.Decrypt(data, passphrase)
	if err != nil {
		return nil, err
	}

	var v model.Vault
	if err := json.Unmarshal(plain, &v); err != nil {
		return nil, fmt.Errorf("parse vault: %w", err)
	}
	return &v, nil
}

// Save serializes, encrypts, and atomically writes the vault.
func (s *Store) Save(v *model.Vault, passphrase string) error {
	v.UpdatedAt = time.Now()

	plain, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize vault: %w", err)
	}

	cipher, err := crypto.Encrypt(plain, passphrase)
	if err != nil {
		return err
	}

	if err := s.fs.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Atomic write: temp file then rename.
	tmp := s.Path + ".tmp"
	if err := s.fs.WriteFile(tmp, cipher, 0o600); err != nil {
		return fmt.Errorf("write temp vault: %w", err)
	}
	if err := s.fs.Rename(tmp, s.Path); err != nil {
		return fmt.Errorf("commit vault: %w", err)
	}
	return nil
}
