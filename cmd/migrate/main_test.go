package main

import (
	"strings"
	"testing"
)

// TestValidateMigrateCommand cubre la allowlist de subcomandos (issue #27).
//
// SPEC: "El binario ./cmd/migrate sólo acepta 'up' y 'status' por defecto.
//
//	Comandos destructivos requieren MIGRATE_ALLOW_DANGEROUS=1."
func TestValidateMigrateCommand(t *testing.T) {
	// Snapshot y restauración de MIGRATE_ALLOW_DANGEROUS para que el
	// test no contamine (ni sea contaminado por) el entorno del runner.
	t.Setenv("MIGRATE_ALLOW_DANGEROUS", "")

	tests := []struct {
		name        string
		command     string
		dangerousOn bool
		wantErr     bool
		errContains string
	}{
		// ── Comandos seguros (sin escape hatch) ──
		{name: "up permitido", command: "up", wantErr: false},
		{name: "status permitido", command: "status", wantErr: false},

		// ── Comandos destructivos bloqueados por defecto ──
		{name: "down bloqueado", command: "down", wantErr: true, errContains: "no está en la allowlist"},
		{name: "reset bloqueado", command: "reset", wantErr: true, errContains: "no está en la allowlist"},
		{name: "redo bloqueado", command: "redo", wantErr: true, errContains: "no está en la allowlist"},
		{name: "create bloqueado", command: "create", wantErr: true, errContains: "no está en la allowlist"},
		{name: "fix bloqueado", command: "fix", wantErr: true, errContains: "no está en la allowlist"},
		{name: "cualquier otro bloqueado", command: "rm -rf /", wantErr: true, errContains: "no está en la allowlist"},

		// ── Escape hatch: MIGRATE_ALLOW_DANGEROUS=1 ──
		{name: "down con escape hatch", command: "down", dangerousOn: true, wantErr: false},
		{name: "reset con escape hatch", command: "reset", dangerousOn: true, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dangerousOn {
				t.Setenv("MIGRATE_ALLOW_DANGEROUS", "1")
			} else {
				t.Setenv("MIGRATE_ALLOW_DANGEROUS", "")
			}

			err := ValidateMigrateCommand(tt.command)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateMigrateCommand(%q) = nil, quería error", tt.command)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("ValidateMigrateCommand(%q) error = %q, quería que contuviera %q",
						tt.command, err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateMigrateCommand(%q) = %v, quería nil", tt.command, err)
			}
		})
	}
}
