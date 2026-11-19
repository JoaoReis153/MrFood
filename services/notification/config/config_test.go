package config

import (
	"context"
	"testing"
)

func TestLoadSMTPOptional(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
		enabled bool
	}{
		{name: "no smtp", env: map[string]string{}, enabled: false},
		{name: "full smtp", env: map[string]string{"SMTP_USER": "u", "SMTP_PASSWORD": "p", "SMTP_FROM": "f@example.com"}, enabled: true},
		{name: "partial smtp", env: map[string]string{"SMTP_USER": "u"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NOTIFICATION_SERVER_PORT", "50058")
			for _, k := range []string{"SMTP_USER", "SMTP_PASSWORD", "SMTP_FROM"} {
				t.Setenv(k, tt.env[k])
			}

			cfg, err := Load(context.Background())
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.SMTP.Enabled() != tt.enabled {
				t.Fatalf("SMTP.Enabled() = %v, want %v", cfg.SMTP.Enabled(), tt.enabled)
			}
		})
	}
}
