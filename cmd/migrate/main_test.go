package main

import (
	"strings"
	"testing"
)

// TestGuardCommand_AllowsSafeCommands verifica que up, up-by-one, status,
// version se aceptan sin --allow-destructive.
func TestGuardCommand_AllowsSafeCommands(t *testing.T) {
	for _, cmd := range []string{"up", "up-by-one", "status", "version"} {
		t.Run(cmd, func(t *testing.T) {
			if err := guardCommand(cmd, false); err != nil {
				t.Errorf("guardCommand(%q, false) = %v, esperaba nil (comando safe)", cmd, err)
			}
		})
	}
}

// TestGuardCommand_RejectsDestructiveWithoutFlag verifica que down, down-to,
// reset, redo se rechazan sin --allow-destructive.
func TestGuardCommand_RejectsDestructiveWithoutFlag(t *testing.T) {
	for _, cmd := range []string{"down", "down-to", "reset", "redo"} {
		t.Run(cmd, func(t *testing.T) {
			err := guardCommand(cmd, false)
			if err == nil {
				t.Fatalf("guardCommand(%q, false) = nil, esperaba error destructivo", cmd)
			}
			if !strings.Contains(err.Error(), "destructivo") {
				t.Errorf("error = %q, debería mencionar 'destructivo'", err.Error())
			}
			if !strings.Contains(err.Error(), "--allow-destructive") {
				t.Errorf("error = %q, debería sugerir --allow-destructive", err.Error())
			}
		})
	}
}

// TestGuardCommand_AllowsDestructiveWithFlag verifica que down, down-to,
// reset, redo se aceptan CON --allow-destructive.
func TestGuardCommand_AllowsDestructiveWithFlag(t *testing.T) {
	for _, cmd := range []string{"down", "down-to", "reset", "redo"} {
		t.Run(cmd, func(t *testing.T) {
			if err := guardCommand(cmd, true); err != nil {
				t.Errorf("guardCommand(%q, true) = %v, esperaba nil (destructivo permitido con flag)", cmd, err)
			}
		})
	}
}

// TestGuardCommand_RejectsUnknown verifica que cualquier comando fuera de
// ambas listas se rechaza con un mensaje que menciona la allowlist.
func TestGuardCommand_RejectsUnknown(t *testing.T) {
	for _, cmd := range []string{"foo", "drop", "truncate", "wipe", "DROP-TABLE"} {
		t.Run(cmd, func(t *testing.T) {
			err := guardCommand(cmd, false)
			if err == nil {
				t.Fatalf("guardCommand(%q, false) = nil, esperaba error de comando desconocido", cmd)
			}
			if !strings.Contains(err.Error(), "allowlist") {
				t.Errorf("error = %q, debería mencionar 'allowlist'", err.Error())
			}
		})
	}
}

// TestGuardCommand_UnknownEvenWithAllowDestructive verifica que un comando
// desconocido sigue rechazado aunque se pase --allow-destructive (la flag
// sólo abre destructivos CONOCIDOS, no amplia la allowlist).
func TestGuardCommand_UnknownEvenWithAllowDestructive(t *testing.T) {
	err := guardCommand("foobar", true)
	if err == nil {
		t.Fatal("guardCommand(\"foobar\", true) = nil, esperaba error — --allow-destructive no ampla la allowlist")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("error = %q, debería mencionar 'allowlist'", err.Error())
	}
}

// TestGuardCommand_MessageDoesNotLeakDestructiveNames verifica que el
// mensaje de error para un comando desconocido no enumere los destructivos
// (eso daría pistas a un atacante sobre qué probar). Sólo los safe deben
// aparecer en la allowlist visible.
func TestGuardCommand_MessageDoesNotLeakDestructiveNames(t *testing.T) {
	err := guardCommand("foobar", false)
	if err == nil {
		t.Fatal("esperaba error")
	}
	msg := err.Error()
	for _, d := range []string{"down", "down-to", "reset", "redo"} {
		if strings.Contains(msg, d) {
			t.Errorf("mensaje %q no debería mencionar comando destructivo %q", msg, d)
		}
	}
}
