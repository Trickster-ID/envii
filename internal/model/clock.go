package model

import "time"

// Clock abstracts time for testability.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a function to Clock.
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }

type vaultOptions struct {
	clock Clock
}

// VaultOption configures NewVault.
type VaultOption func(*vaultOptions)

// WithClock injects a Clock used for UpdatedAt.
func WithClock(c Clock) VaultOption {
	return func(o *vaultOptions) { o.clock = c }
}
