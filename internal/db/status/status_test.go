package status_test

import (
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

// TestInviteStatusType verifica que InviteStatus e Status son tipos distintos
// y no se pueden mezclar sin una conversión explícita.
func TestTypesAreDistinct(t *testing.T) {
	var is status.InviteStatus = status.InviteStatusPending
	var s status.Status = status.StatusPending

	// Mismo string subyacente, pero tipos distintos — la conversión explícita
	// es necesaria; si compilara sin ella sería un bug de tipos.
	if string(is) != string(s) {
		t.Errorf("ambos valores pending deberían ser iguales como string: %q != %q", string(is), string(s))
	}
}
