package model_test

import (
	"testing"
	"time"

	"github.com/trickylab/envii/internal/model"
)

func TestNewVaultUsesInjectedClock(t *testing.T) {
	tests := []struct {
		name  string
		fixed time.Time
	}{
		{name: "fixed utc", fixed: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)},
		{name: "epoch", fixed: time.Unix(0, 0).UTC()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := model.ClockFunc(func() time.Time { return tt.fixed })
			v := model.NewVault(model.WithClock(c))
			if !v.UpdatedAt.Equal(tt.fixed) {
				t.Fatalf("expected UpdatedAt %v, got %v", tt.fixed, v.UpdatedAt)
			}
		})
	}
}
