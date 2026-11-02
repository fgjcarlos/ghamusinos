package ai

import (
	"testing"
	"time"
)

// TestActivityScope comprueba la clave de scope documentada para el
// análisis de una actividad.
func TestActivityScope(t *testing.T) {
	t.Parallel()

	got := ActivityScope("6f1c9d2e-0b7a-4a1e-9f2b-1c2d3e4f5a6b")
	want := "activity:6f1c9d2e-0b7a-4a1e-9f2b-1c2d3e4f5a6b"
	if got != want {
		t.Errorf("ActivityScope() = %q, quiere %q", got, want)
	}
}

// TestActivityScopeEmpty devuelve una clave vacía cuando no hay id: un
// scope vacío no es válido en ai_analysis (CHECK scope <> ”).
func TestActivityScopeEmpty(t *testing.T) {
	t.Parallel()

	if got := ActivityScope(""); got != "" {
		t.Errorf("ActivityScope(\"\") = %q, quiere \"\"", got)
	}
}

// TestWeeklyScope comprueba la clave de scope semanal: la fecha del
// inicio de la ventana.
func TestWeeklyScope(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	got := WeeklyScope(start)
	want := "weekly:2026-03-02"
	if got != want {
		t.Errorf("WeeklyScope() = %q, quiere %q", got, want)
	}
}

// TestWeeklyScopeUsesGivenLocation comprueba que la fecha es la del
// instante en su propia zona: el job y la API pasan el mismo instante.
func TestWeeklyScopeUsesGivenLocation(t *testing.T) {
	t.Parallel()

	zone := time.FixedZone("CET", 2*60*60)
	start := time.Date(2026, time.March, 2, 23, 30, 0, 0, zone)
	got := WeeklyScope(start)
	want := "weekly:2026-03-02"
	if got != want {
		t.Errorf("WeeklyScope() = %q, quiere %q", got, want)
	}
}

// TestLoadScope comprueba la clave de scope de carga: fecha de inicio y
// número de días de la ventana.
func TestLoadScope(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	got := LoadScope(start, 28)
	want := "load:2026-03-02:28"
	if got != want {
		t.Errorf("LoadScope() = %q, quiere %q", got, want)
	}
}

// TestScopesAreDistinct comprueba que los tres análisis no comparten
// scope: la fila de ai_analysis es única por (user_id, kind, scope).
func TestScopesAreDistinct(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	seen := map[string]string{
		ActivityScope("abc"): "activity",
		WeeklyScope(start):   "weekly",
		LoadScope(start, 28): "load",
	}
	if len(seen) != 3 {
		t.Errorf("los scopes deben ser distintos entre tipos: %v", seen)
	}
}

// TestParseScopeActivity comprueba que el scope de una actividad se
// puede volver a interpretar: es el único sitio donde vive el formato.
func TestParseScopeActivity(t *testing.T) {
	t.Parallel()

	target, err := ParseScope(ActivityScope("6f1c9d2e-0b7a-4a1e-9f2b-1c2d3e4f5a6b"))
	if err != nil {
		t.Fatalf("ParseScope() = %v, quiere nil", err)
	}
	if target.Kind != KindActivity {
		t.Errorf("Kind = %q, quiere %q", target.Kind, KindActivity)
	}
	if target.ActivityID != "6f1c9d2e-0b7a-4a1e-9f2b-1c2d3e4f5a6b" {
		t.Errorf("ActivityID = %q, quiere el id de la actividad", target.ActivityID)
	}
}

// TestParseScopeWeekly comprueba la ida y vuelta del scope semanal.
func TestParseScopeWeekly(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	target, err := ParseScope(WeeklyScope(start))
	if err != nil {
		t.Fatalf("ParseScope() = %v, querer nil", err)
	}
	if target.Kind != KindWeekly {
		t.Errorf("Kind = %q, quiere %q", target.Kind, KindWeekly)
	}
	if got := target.Date.Format(ScopeDateLayout); got != "2026-03-02" {
		t.Errorf("Date = %q, quiere 2026-03-02", got)
	}
}

// TestParseScopeLoad comprueba la ida y vuelta del scope de carga.
func TestParseScopeLoad(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	target, err := ParseScope(LoadScope(start, 28))
	if err != nil {
		t.Fatalf("ParseScope() = %v, quiere nil", err)
	}
	if target.Kind != KindLoad {
		t.Errorf("Kind = %q, quiere %q", target.Kind, KindLoad)
	}
	if got := target.Date.Format(ScopeDateLayout); got != "2026-03-02" {
		t.Errorf("Date = %q, quiere 2026-03-02", got)
	}
	if target.Days != 28 {
		t.Errorf("Days = %d, quiere 28", target.Days)
	}
}

// TestParseScopeInvalid comprueba que un scope desconocido o mal formado
// se rechaza: el job nunca adivina el objetivo.
func TestParseScopeInvalid(t *testing.T) {
	t.Parallel()

	for _, scope := range []string{
		"",
		"activity:",
		"weekly:2026-13-45",
		"load:2026-03-02",
		"load:2026-03-02:0",
		"load:2026-03-02:many",
		"daily:2026-03-02",
	} {
		if _, err := ParseScope(scope); err == nil {
			t.Errorf("ParseScope(%q) = nil, quiere error", scope)
		}
	}
}
