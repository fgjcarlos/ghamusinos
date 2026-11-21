package workouts

import "time"

// Helpers de calendario en la zona del usuario (fase 2.2). Los limites
// de cada dia son civiles y los decide users.timezone, no el servidor:
// estos helpers son la unica traduccion instante<->dia civil del feature
// de sesiones planificadas, y la bandeja de actividades del calendario
// (T3) los reutiliza en lugar de reimplementarlos.

// UserLocation resuelve la zona guardada del usuario (users.timezone) a
// una ubicacion IANA. Un nombre vacio o desconocido cae a UTC: la
// comparacion nunca falla por una zona mala (la base garantiza NOT NULL
// pero no validez IANA).
func UserLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// LocalDate carries the instant's local calendar date as a UTC value, not
// as a local midnight: some time-zone transitions skip or repeat midnight.
func LocalDate(t time.Time, loc *time.Location) time.Time {
	year, month, day := t.In(loc).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// WeekStart devuelve el lunes 00:00 de la semana ISO que contiene la
// fecha civil dada (lunes-based, decision L2).
func WeekStart(date time.Time) time.Time {
	offset := (int(date.Weekday()) + 6) % 7
	return date.AddDate(0, 0, -offset)
}
