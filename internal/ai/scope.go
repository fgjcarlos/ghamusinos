package ai

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ScopeDateLayout es el formato de fecha que usan las claves de scope.
//
// La fecha es siempre la del inicio de la ventana en la zona horaria del
// usuario: la API (T7) y el job (T5) construyen la misma clave a partir del
// mismo instante, así que siempre apuntan a la misma fila de ai_analysis.
const ScopeDateLayout = "2006-01-02"

// Prefijos de las claves de scope. Son parte del contrato con la API: la
// columna `scope` de ai_analysis guarda esta clave y su longitud está
// acotada por el CHECK del esquema.
const (
	// ScopePrefixActivity precede al id de la actividad analizada.
	ScopePrefixActivity = "activity"
	// ScopePrefixWeekly precede a la fecha de inicio de la ventana semanal.
	ScopePrefixWeekly = "weekly"
	// ScopePrefixLoad precede a la fecha de inicio y a los días de la
	// ventana de carga.
	ScopePrefixLoad = "load"
)

// ActivityScope devuelve la clave de scope del análisis de una actividad:
// "activity:<id>".
//
// Devuelve "" si el id está vacío: una clave vacía no es válida en
// ai_analysis (CHECK scope <> ”) y el job la rechaza antes de enviar nada.
func ActivityScope(activityID string) string {
	if activityID == "" {
		return ""
	}
	return ScopePrefixActivity + ":" + activityID
}

// WeeklyScope devuelve la clave de scope del análisis semanal:
// "weekly:<YYYY-MM-DD>", con la fecha del inicio de la ventana.
//
// La fecha se lee en la zona horaria del propio instante, así que la clave
// no depende de la zona del servidor.
func WeeklyScope(start time.Time) string {
	return ScopePrefixWeekly + ":" + start.Format(ScopeDateLayout)
}

// LoadScope devuelve la clave de scope del análisis de carga:
// "load:<YYYY-MM-DD>:<días>", con la fecha del inicio de la ventana y su
// número de días.
//
// La fecha se lee en la zona horaria del propio instante, así que la clave
// no depende de la zona del servidor.
func LoadScope(start time.Time, days int) string {
	return ScopePrefixLoad + ":" + start.Format(ScopeDateLayout) + ":" + strconv.Itoa(days)
}

// ScopeTarget describe el objetivo que codifica una clave de scope.
//
// Es la inversa de las funciones de scope: el job la usa para saber qué
// analizar a partir de la clave, sin depender de otros campos.
type ScopeTarget struct {
	// Kind es el tipo de análisis que codifica el scope.
	Kind Kind
	// ActivityID es el id de la actividad (Kind == KindActivity).
	ActivityID string
	// Date es la fecha del inicio de la ventana (Kind == KindWeekly o
	// KindLoad), en UTC.
	Date time.Time
	// Days es el número de días de la ventana (Kind == KindLoad).
	Days int
}

// ParseScope interpreta una clave de scope construida por ActivityScope,
// WeeklyScope o LoadScope.
//
// Es la única fuente de verdad del formato: si la clave no se puede
// interpretar, se devuelve un error y el job no adivina el objetivo.
func ParseScope(scope string) (ScopeTarget, error) {
	switch {
	case strings.HasPrefix(scope, ScopePrefixActivity+":"):
		id := strings.TrimPrefix(scope, ScopePrefixActivity+":")
		if id == "" {
			return ScopeTarget{}, fmt.Errorf("ai: scope de actividad sin id: %q", scope)
		}
		return ScopeTarget{Kind: KindActivity, ActivityID: id}, nil

	case strings.HasPrefix(scope, ScopePrefixWeekly+":"):
		date, err := time.Parse(ScopeDateLayout, strings.TrimPrefix(scope, ScopePrefixWeekly+":"))
		if err != nil {
			return ScopeTarget{}, fmt.Errorf("ai: scope semanal inválido %q: %w", scope, err)
		}
		return ScopeTarget{Kind: KindWeekly, Date: date}, nil

	case strings.HasPrefix(scope, ScopePrefixLoad+":"):
		parts := strings.Split(strings.TrimPrefix(scope, ScopePrefixLoad+":"), ":")
		if len(parts) != 2 {
			return ScopeTarget{}, fmt.Errorf("ai: scope de carga inválido %q", scope)
		}
		date, err := time.Parse(ScopeDateLayout, parts[0])
		if err != nil {
			return ScopeTarget{}, fmt.Errorf("ai: scope de carga inválido %q: %w", scope, err)
		}
		days, err := strconv.Atoi(parts[1])
		if err != nil {
			return ScopeTarget{}, fmt.Errorf("ai: scope de carga inválido %q: %w", scope, err)
		}
		if days <= 0 {
			return ScopeTarget{}, fmt.Errorf("ai: scope de carga con %d días: %q", days, scope)
		}
		return ScopeTarget{Kind: KindLoad, Date: date, Days: days}, nil

	default:
		return ScopeTarget{}, fmt.Errorf("ai: scope desconocido %q", scope)
	}
}
