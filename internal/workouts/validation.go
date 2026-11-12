package workouts

import (
	"errors"
	"fmt"
)

// ErrValidation marca un fallo de validacion de dominio. Los handlers lo
// traducen a 422 (RFC 9457); la base de datos tiene sus propios CHECK.
var ErrValidation = errors.New("workouts: invalid session")

const (
	// MaxTitleLength es el maximo de caracteres del titulo.
	MaxTitleLength = 200
	// MaxNotesLength es el maximo de caracteres de las notas.
	MaxNotesLength = 2000
)

// validWorkoutTypes son los tipos admitidos. Fuente unica del dominio:
// el CHECK de la tabla repite esta lista y el test de contrato verifica
// que ambos siguen coincideciendo.
var validWorkoutTypes = map[WorkoutType]bool{
	WorkoutTypeRodaje:       true,
	WorkoutTypeTiradaLarga:  true,
	WorkoutTypeSeries:       true,
	WorkoutTypeCuestas:      true,
	WorkoutTypeTempo:        true,
	WorkoutTypeFartlek:      true,
	WorkoutTypeFuerza:       true,
	WorkoutTypeRecuperacion: true,
	WorkoutTypeOtro:         true,
}

// validIntensities son las zonas de FC admitidas como objetivo.
var validIntensities = map[Intensity]bool{
	IntensityZ1: true,
	IntensityZ2: true,
	IntensityZ3: true,
	IntensityZ4: true,
	IntensityZ5: true,
}

// Validate comprueba la sesion antes de guardarla. Devuelve un error que
// envuelve ErrValidation con el campo y el motivo concretos.
func Validate(s Session) error {
	if s.Title == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if len(s.Title) > MaxTitleLength {
		return fmt.Errorf("%w: title exceeds %d characters", ErrValidation, MaxTitleLength)
	}
	if len(s.Notes) > MaxNotesLength {
		return fmt.Errorf("%w: notes exceed %d characters", ErrValidation, MaxNotesLength)
	}
	if !validWorkoutTypes[s.WorkoutType] {
		return fmt.Errorf("%w: unknown workout_type %q", ErrValidation, string(s.WorkoutType))
	}
	if s.TargetDurationSec != nil && *s.TargetDurationSec < 0 {
		return fmt.Errorf("%w: target_duration_sec cannot be negative", ErrValidation)
	}
	if s.TargetDistanceM != nil && *s.TargetDistanceM < 0 {
		return fmt.Errorf("%w: target_distance_m cannot be negative", ErrValidation)
	}
	if s.TargetElevationGainM != nil && *s.TargetElevationGainM < 0 {
		return fmt.Errorf("%w: target_elevation_gain_m cannot be negative", ErrValidation)
	}
	if s.TargetIntensity != nil && !validIntensities[*s.TargetIntensity] {
		return fmt.Errorf("%w: unknown target_intensity %q", ErrValidation, string(*s.TargetIntensity))
	}
	if s.ScheduledOn.IsZero() {
		return fmt.Errorf("%w: scheduled_on is required", ErrValidation)
	}
	return nil
}
