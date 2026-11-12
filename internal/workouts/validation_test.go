package workouts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestValidate covers the domain validation table: each case states an
// input and whether it must be rejected.
func TestValidate(t *testing.T) {
	validDate := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	negative := -1
	badIntensity := Intensity("z9")

	tests := []struct {
		name    string
		session Session
		wantErr bool
	}{
		{
			name: "sesion minima valida",
			session: Session{
				ScheduledOn: validDate,
				Title:       "Tirada larga",
				WorkoutType: WorkoutTypeTiradaLarga,
			},
		},
		{
			name: "sesion completa valida",
			session: Session{
				ScheduledOn:          validDate,
				Title:                "Series en cuesta",
				Notes:                "subir al monte",
				WorkoutType:          WorkoutTypeCuestas,
				TargetDurationSec:    intPtr(3600),
				TargetDistanceM:      intPtr(8000),
				TargetElevationGainM: intPtr(400),
				TargetIntensity:      intensityPtr(IntensityZ3),
			},
		},
		{
			name: "titulo vacio",
			session: Session{
				ScheduledOn: validDate,
				Title:       "",
				WorkoutType: WorkoutTypeRodaje,
			},
			wantErr: true,
		},
		{
			name: "titulo demasiado largo",
			session: Session{
				ScheduledOn: validDate,
				Title:       strings.Repeat("x", MaxTitleLength+1),
				WorkoutType: WorkoutTypeRodaje,
			},
			wantErr: true,
		},
		{
			name: "notas demasiado largas",
			session: Session{
				ScheduledOn: validDate,
				Title:       "Rodaje",
				Notes:       strings.Repeat("y", MaxNotesLength+1),
				WorkoutType: WorkoutTypeRodaje,
			},
			wantErr: true,
		},
		{
			name: "tipo de entrenamiento desconocido",
			session: Session{
				ScheduledOn: validDate,
				Title:       "Sesion",
				WorkoutType: WorkoutType("nope"),
			},
			wantErr: true,
		},
		{
			name: "duracion objetivo negativa",
			session: Session{
				ScheduledOn:       validDate,
				Title:             "Rodaje",
				WorkoutType:       WorkoutTypeRodaje,
				TargetDurationSec: &negative,
			},
			wantErr: true,
		},
		{
			name: "distancia objetivo negativa",
			session: Session{
				ScheduledOn:     validDate,
				Title:           "Rodaje",
				WorkoutType:     WorkoutTypeRodaje,
				TargetDistanceM: &negative,
			},
			wantErr: true,
		},
		{
			name: "desnivel objetivo negativo",
			session: Session{
				ScheduledOn:          validDate,
				Title:                "Rodaje",
				WorkoutType:          WorkoutTypeRodaje,
				TargetElevationGainM: &negative,
			},
			wantErr: true,
		},
		{
			name: "intensidad desconocida",
			session: Session{
				ScheduledOn:     validDate,
				Title:           "Rodaje",
				WorkoutType:     WorkoutTypeRodaje,
				TargetIntensity: &badIntensity,
			},
			wantErr: true,
		},
		{
			name: "fecha ausente",
			session: Session{
				Title:       "Rodaje",
				WorkoutType: WorkoutTypeRodaje,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.session)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Validate() expected an error, got nil")
				}
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("Validate() error = %v, want it to wrap ErrValidation", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
		})
	}
}

// TestWorkoutTypeVocabularyMatchesTheDatabase proves the Go vocabulary and
// the CHECK constraint of the table admit exactly the same workout types.
// The CHECK lives in the migration; this test reads it from disk and
// compares both sides as sets, so neither copy can drift apart silently.
func TestWorkoutTypeVocabularyMatchesTheDatabase(t *testing.T) {
	db := databaseVocabulary(t, "planned_sessions_workout_type_valido", "workout_type")

	got := make(map[string]bool, len(validWorkoutTypes))
	for workoutType := range validWorkoutTypes {
		got[string(workoutType)] = true
	}
	assertSameVocabulary(t, "workout_type", db, got)
}

// TestIntensityVocabularyMatchesTheDatabase proves the Go intensity
// vocabulary and the CHECK constraint of the table agree.
func TestIntensityVocabularyMatchesTheDatabase(t *testing.T) {
	db := databaseVocabulary(t, "planned_sessions_target_intensity_valido", "target_intensity")

	got := make(map[string]bool, len(validIntensities))
	for intensity := range validIntensities {
		got[string(intensity)] = true
	}
	assertSameVocabulary(t, "target_intensity", db, got)
}

// databaseVocabulary extracts the values of a CHECK constraint from the
// migration. The constraint is identified by its name and the column it
// guards, so the extraction cannot pick up the vocabulary of another
// constraint.
func databaseVocabulary(t *testing.T, constraint, column string) []string {
	t.Helper()
	migration := readMigration(t)

	marker := "CONSTRAINT " + constraint
	start := strings.Index(migration, marker)
	if start < 0 {
		t.Fatalf("the migration does not declare the constraint %q", constraint)
	}
	rest := migration[start+len(marker):]

	// La columna puede aparecer como "col IN (...)" o como
	// "col IS NULL OR col IN (...)": se busca el fragmento de la lista.
	open := strings.Index(rest, column+" IN (")
	if open < 0 {
		t.Fatalf("the constraint %q does not check %q with an IN list", constraint, column)
	}
	rest = rest[open+len(column+" IN ("):]

	end := strings.Index(rest, ")")
	if end < 0 {
		t.Fatalf("the constraint %q has an unterminated IN list", constraint)
	}

	var values []string
	for _, quoted := range strings.Split(rest[:end], ",") {
		quoted = strings.TrimSpace(quoted)
		quoted = strings.Trim(quoted, "'")
		if quoted == "" {
			continue
		}
		values = append(values, quoted)
	}
	return values
}

// assertSameVocabulary proves that the database and the domain accept
// exactly the same set of values, in both directions.
func assertSameVocabulary(t *testing.T, name string, dbValues []string, domain map[string]bool) {
	t.Helper()

	db := make(map[string]bool, len(dbValues))
	for _, value := range dbValues {
		db[value] = true
	}

	for value := range db {
		if !domain[value] {
			t.Errorf("the database admits %q but the domain rejects it", value)
		}
	}
	for value := range domain {
		if !db[value] {
			t.Errorf("the domain admits %q but the database rejects it", value)
		}
	}
	if len(db) != len(domain) {
		t.Errorf("%s vocabulary: the database has %d values, the domain has %d", name, len(db), len(domain))
	}
}

// readMigration reads the planned_sessions migration from disk.
func readMigration(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "db", "migrations", "00016_planned_sessions.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	return string(b)
}

// intPtr devuelve un puntero al entero dado.
func intPtr(v int) *int { return &v }

// intensityPtr devuelve un puntero a la zona dada.
func intensityPtr(v Intensity) *Intensity { return &v }
