package app

import "testing"

// TestRun comprueba que el esqueleto arranca sin error. Sirve además para
// establecer el harness de tests del proyecto (go test ./...).
func TestRun(t *testing.T) {
	if err := Run(); err != nil {
		t.Fatalf("Run() devolvió error inesperado: %v", err)
	}
}
