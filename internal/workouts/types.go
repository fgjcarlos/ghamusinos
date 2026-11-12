// Package workouts define el dominio de las sesiones de entrenamiento
// planificadas (fase 2.1, issue #298).
//
// El paquete no expone pgtype: la conversion entre las filas de sqlc y
// estos tipos vive en store.go, de modo que el handler HTTP (#299) habla
// de sesiones y no de la base de datos.
package workouts

import (
	"time"

	"github.com/google/uuid"
)

// WorkoutType es el tipo de sesion planificada. Los valores son
// exactamente los que admite el CHECK de la tabla (migracion
// 00016_planned_sessions.sql): el dominio y la base de datos no pueden
// discrepar.
type WorkoutType string

const (
	// WorkoutTypeRodaje: sesion de volumen easy.
	WorkoutTypeRodaje WorkoutType = "rodaje"
	// WorkoutTypeTiradaLarga: sesion larga de fin de semana.
	WorkoutTypeTiradaLarga WorkoutType = "tirada_larga"
	// WorkoutTypeSeries: series de ritmo o velocidad.
	WorkoutTypeSeries WorkoutType = "series"
	// WorkoutTypeCuestas: repeticiones en subida.
	WorkoutTypeCuestas WorkoutType = "cuestas"
	// WorkoutTypeTempo: sesion a ritmo medio-alto constante.
	WorkoutTypeTempo WorkoutType = "tempo"
	// WorkoutTypeFartlek: cambios de ritmo.
	WorkoutTypeFartlek WorkoutType = "fartlek"
	// WorkoutTypeFuerza: trabajo de fuerza en sala o gimnasio.
	WorkoutTypeFuerza WorkoutType = "fuerza"
	// WorkoutTypeRecuperacion: recuperacion activa o descanso programado.
	WorkoutTypeRecuperacion WorkoutType = "recuperacion"
	// WorkoutTypeOtro: cualquier otra sesion (catch-all del formulario).
	WorkoutTypeOtro WorkoutType = "otro"
)

// Intensity es la zona de frecuencia cardiaca objetivo de la sesion.
// Reutiliza el vocabulario de zonas que ya existe en el producto (z1..z5).
type Intensity string

const (
	// IntensityZ1: recuperacion.
	IntensityZ1 Intensity = "z1"
	// IntensityZ2: endurance.
	IntensityZ2 Intensity = "z2"
	// IntensityZ3: tempo.
	IntensityZ3 Intensity = "z3"
	// IntensityZ4: umbral.
	IntensityZ4 Intensity = "z4"
	// IntensityZ5: maxima.
	IntensityZ5 Intensity = "z5"
)

// Session es una sesion de entrenamiento planificada.
//
// Los objetivos son opcionales: nil significa "no planificado", que no es
// lo mismo que un objetivo de cero. ScheduledOn es un dia civil
// (medianoche UTC), no un instante: la zona horaria del usuario se
// resolvio antes de escribir.
type Session struct {
	ID                   uuid.UUID   `json:"id"`
	UserID               uuid.UUID   `json:"user_id"`
	ScheduledOn          time.Time   `json:"scheduled_on"`
	Title                string      `json:"title"`
	Notes                string      `json:"notes"`
	WorkoutType          WorkoutType `json:"workout_type"`
	TargetDurationSec    *int        `json:"target_duration_sec,omitempty"`
	TargetDistanceM      *int        `json:"target_distance_m,omitempty"`
	TargetElevationGainM *int        `json:"target_elevation_gain_m,omitempty"`
	TargetIntensity      *Intensity  `json:"target_intensity,omitempty"`
}
