package status_test

import (
	"reflect"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/db/status"
)

func TestInviteStatusValues(t *testing.T) {
	tests := []struct {
		name  string
		value status.InviteStatus
		want  string
	}{
		{"pending", status.InviteStatusPending, "pending"},
		{"active", status.InviteStatusActive, "active"},
		{"blocked", status.InviteStatusBlocked, "blocked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("InviteStatus %q: got %q, want %q", tt.name, string(tt.value), tt.want)
			}
		})
	}
}

func TestStatusValues(t *testing.T) {
	tests := []struct {
		name  string
		value status.Status
		want  string
	}{
		{"pending", status.StatusPending, "pending"},
		{"accepted", status.StatusAccepted, "accepted"},
		{"revoked", status.StatusRevoked, "revoked"},
		{"expired", status.StatusExpired, "expired"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("Status %q: got %q, want %q", tt.name, string(tt.value), tt.want)
			}
		})
	}
}

// TestTypesAreDistinct verifica que InviteStatus y Status son tipos
// distintos y no se pueden mezclar sin una conversión explícita.
// La garantía se obtiene comparando los tipos reflectantes (que
// capturan el tipo declarado, no el valor): si InviteStatus y Status
// fueran el mismo tipo, reflect.TypeOf daría idéntico resultado.
func TestTypesAreDistinct(t *testing.T) {
	// Tipos inferidos del RHS: la garantía de "tipos distintos" ya no
	// depende de una declaración explícita (que era ST1023), sino de
	// la comparación de tipos reflectantes.
	is := status.InviteStatusPending
	s := status.StatusPending

	if reflect.TypeOf(is) == reflect.TypeOf(s) {
		t.Errorf("InviteStatus y Status deberían ser tipos distintos pero reflect.TypeOf los considera iguales: %v",
			reflect.TypeOf(is))
	}

	// Sanity check: los valores subyacentes (string) sí son iguales.
	if string(is) != string(s) {
		t.Errorf("ambos valores pending deberían ser iguales como string: %q != %q", string(is), string(s))
	}
}
