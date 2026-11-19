package config

import (
	"context"
	"testing"
)

func TestLoadStripeKeyOrDemoMode(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		demoMode string
		wantErr  bool
	}{
		{name: "key set", key: "sk_test_x"},
		{name: "demo mode without key", demoMode: "true"},
		{name: "no key and no demo mode", wantErr: true},
		{name: "demo mode false without key", demoMode: "false", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("STRIPE_SECRET_KEY", tt.key)
			t.Setenv("PAYMENT_DEMO_MODE", tt.demoMode)

			_, err := Load(context.Background())
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
