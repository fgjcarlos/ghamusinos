package config

import (
	"os"
	"testing"
)

// TestLoadRequiredStravaConfig verifies that missing Strava config fields are caught.
func TestLoadRequiredStravaConfig(t *testing.T) {
	// Save current env vars
	savedVars := map[string]string{
		"STRAVA_CLIENT_ID":            os.Getenv("STRAVA_CLIENT_ID"),
		"STRAVA_CLIENT_SECRET":        os.Getenv("STRAVA_CLIENT_SECRET"),
		"STRAVA_CALLBACK_URL":         os.Getenv("STRAVA_CALLBACK_URL"),
		"STRAVA_WEBHOOK_SECRET":       os.Getenv("STRAVA_WEBHOOK_SECRET"),
		"STRAVA_TOKEN_ENCRYPTION_KEY": os.Getenv("STRAVA_TOKEN_ENCRYPTION_KEY"),
		"DATABASE_URL":                os.Getenv("DATABASE_URL"),
		"CLERK_JWKS_URL":              os.Getenv("CLERK_JWKS_URL"),
	}
	defer func() {
		for k, v := range savedVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set required base config
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("CLERK_JWKS_URL", "https://example.com/.well-known/jwks.json")

	tests := []struct {
		name    string
		setup   func()
		wantErr bool
		errMsg  string
	}{
		{
			name: "Missing STRAVA_CLIENT_ID",
			setup: func() {
				os.Unsetenv("STRAVA_CLIENT_ID")
				os.Setenv("STRAVA_CLIENT_SECRET", "secret")
				os.Setenv("STRAVA_CALLBACK_URL", "https://example.com/callback")
				os.Setenv("STRAVA_WEBHOOK_SECRET", "webhook_secret")
				os.Setenv("STRAVA_TOKEN_ENCRYPTION_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
			},
			wantErr: true,
			errMsg:  "STRAVA_CLIENT_ID",
		},
		{
			name: "Missing STRAVA_CLIENT_SECRET",
			setup: func() {
				os.Setenv("STRAVA_CLIENT_ID", "client_id")
				os.Unsetenv("STRAVA_CLIENT_SECRET")
				os.Setenv("STRAVA_CALLBACK_URL", "https://example.com/callback")
				os.Setenv("STRAVA_WEBHOOK_SECRET", "webhook_secret")
				os.Setenv("STRAVA_TOKEN_ENCRYPTION_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
			},
			wantErr: true,
			errMsg:  "STRAVA_CLIENT_SECRET",
		},
		{
			name: "Missing STRAVA_CALLBACK_URL",
			setup: func() {
				os.Setenv("STRAVA_CLIENT_ID", "client_id")
				os.Setenv("STRAVA_CLIENT_SECRET", "secret")
				os.Unsetenv("STRAVA_CALLBACK_URL")
				os.Setenv("STRAVA_WEBHOOK_SECRET", "webhook_secret")
				os.Setenv("STRAVA_TOKEN_ENCRYPTION_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
			},
			wantErr: true,
			errMsg:  "STRAVA_CALLBACK_URL",
		},
		{
			name: "Missing STRAVA_WEBHOOK_SECRET",
			setup: func() {
				os.Setenv("STRAVA_CLIENT_ID", "client_id")
				os.Setenv("STRAVA_CLIENT_SECRET", "secret")
				os.Setenv("STRAVA_CALLBACK_URL", "https://example.com/callback")
				os.Unsetenv("STRAVA_WEBHOOK_SECRET")
				os.Setenv("STRAVA_TOKEN_ENCRYPTION_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
			},
			wantErr: true,
			errMsg:  "STRAVA_WEBHOOK_SECRET",
		},
		{
			name: "Missing STRAVA_TOKEN_ENCRYPTION_KEY",
			setup: func() {
				os.Setenv("STRAVA_CLIENT_ID", "client_id")
				os.Setenv("STRAVA_CLIENT_SECRET", "secret")
				os.Setenv("STRAVA_CALLBACK_URL", "https://example.com/callback")
				os.Setenv("STRAVA_WEBHOOK_SECRET", "webhook_secret")
				os.Unsetenv("STRAVA_TOKEN_ENCRYPTION_KEY")
			},
			wantErr: true,
			errMsg:  "STRAVA_TOKEN_ENCRYPTION_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load: got err=%v, want err=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Fatalf("error should contain %q, got %q", tt.errMsg, err.Error())
				}
			}
			if !tt.wantErr && cfg == nil {
				t.Fatal("expected config, got nil")
			}
		})
	}
}

// TestLoadStravaKeyValidation verifies that the encryption key is validated.
func TestLoadStravaKeyValidation(t *testing.T) {
	// Save current env vars
	savedVars := map[string]string{
		"STRAVA_CLIENT_ID":            os.Getenv("STRAVA_CLIENT_ID"),
		"STRAVA_CLIENT_SECRET":        os.Getenv("STRAVA_CLIENT_SECRET"),
		"STRAVA_CALLBACK_URL":         os.Getenv("STRAVA_CALLBACK_URL"),
		"STRAVA_WEBHOOK_SECRET":       os.Getenv("STRAVA_WEBHOOK_SECRET"),
		"STRAVA_TOKEN_ENCRYPTION_KEY": os.Getenv("STRAVA_TOKEN_ENCRYPTION_KEY"),
		"DATABASE_URL":                os.Getenv("DATABASE_URL"),
		"CLERK_JWKS_URL":              os.Getenv("CLERK_JWKS_URL"),
	}
	defer func() {
		for k, v := range savedVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set required base config
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("CLERK_JWKS_URL", "https://example.com/.well-known/jwks.json")
	os.Setenv("STRAVA_CLIENT_ID", "client_id")
	os.Setenv("STRAVA_CLIENT_SECRET", "secret")
	os.Setenv("STRAVA_CALLBACK_URL", "https://example.com/callback")
	os.Setenv("STRAVA_WEBHOOK_SECRET", "webhook_secret")

	tests := []struct {
		name    string
		keyHex  string
		wantErr bool
	}{
		{
			name:    "Valid 64-char hex (32 bytes)",
			keyHex:  "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			wantErr: false,
		},
		{
			name:    "Too short (32 chars = 16 bytes)",
			keyHex:  "0102030405060708090a0b0c0d0e0f10",
			wantErr: true,
		},
		{
			name:    "Too long (96 chars = 48 bytes)",
			keyHex:  "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f30",
			wantErr: true,
		},
		{
			name:    "Invalid hex characters",
			keyHex:  "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1fZZ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("STRAVA_TOKEN_ENCRYPTION_KEY", tt.keyHex)

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load: got err=%v, want err=%v", err, tt.wantErr)
			}
			if !tt.wantErr && cfg == nil {
				t.Fatal("expected config, got nil")
			}
		})
	}
}

// TestLoadStravaBackfillDays verifies that STRAVA_BACKFILL_DAYS is loaded correctly.
func TestLoadStravaBackfillDays(t *testing.T) {
	// Save current env vars
	savedVars := map[string]string{
		"STRAVA_CLIENT_ID":            os.Getenv("STRAVA_CLIENT_ID"),
		"STRAVA_CLIENT_SECRET":        os.Getenv("STRAVA_CLIENT_SECRET"),
		"STRAVA_CALLBACK_URL":         os.Getenv("STRAVA_CALLBACK_URL"),
		"STRAVA_WEBHOOK_SECRET":       os.Getenv("STRAVA_WEBHOOK_SECRET"),
		"STRAVA_TOKEN_ENCRYPTION_KEY": os.Getenv("STRAVA_TOKEN_ENCRYPTION_KEY"),
		"STRAVA_BACKFILL_DAYS":        os.Getenv("STRAVA_BACKFILL_DAYS"),
		"DATABASE_URL":                os.Getenv("DATABASE_URL"),
		"CLERK_JWKS_URL":              os.Getenv("CLERK_JWKS_URL"),
	}
	defer func() {
		for k, v := range savedVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set required base config
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("CLERK_JWKS_URL", "https://example.com/.well-known/jwks.json")
	os.Setenv("STRAVA_CLIENT_ID", "client_id")
	os.Setenv("STRAVA_CLIENT_SECRET", "secret")
	os.Setenv("STRAVA_CALLBACK_URL", "https://example.com/callback")
	os.Setenv("STRAVA_WEBHOOK_SECRET", "webhook_secret")
	os.Setenv("STRAVA_TOKEN_ENCRYPTION_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")

	tests := []struct {
		name         string
		dayStr       string
		expectedDays int
	}{
		{"Default (not set)", "", 42},
		{"Custom value", "7", 7},
		{"Large value", "365", 365},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dayStr == "" {
				os.Unsetenv("STRAVA_BACKFILL_DAYS")
			} else {
				os.Setenv("STRAVA_BACKFILL_DAYS", tt.dayStr)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if cfg.StravaBackfillDays != tt.expectedDays {
				t.Fatalf("StravaBackfillDays: got %d, want %d", cfg.StravaBackfillDays, tt.expectedDays)
			}
		})
	}
}

// helper function
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
