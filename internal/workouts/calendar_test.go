package workouts

import (
	"testing"
	"time"
)

func civilUTC(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04", s, time.UTC)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return parsed
}

// TestUserLocationFallsBackToUTC: la zona guardada se resuelve a una
// IANA valida; vacia o desconocida cae a UTC sin error.
func TestUserLocationFallsBackToUTC(t *testing.T) {
	if got := UserLocation("Europe/Madrid"); got.String() != "Europe/Madrid" {
		t.Errorf("UserLocation(Europe/Madrid) = %q", got.String())
	}
	for _, name := range []string{"", "Mars/Olympus", "utc"} {
		if got := UserLocation(name); got != time.UTC {
			t.Errorf("UserLocation(%q) = %v, want UTC", name, got)
		}
	}
}

// TestLocalDateCollapsesToCivilMidnight: un instante cae a la medianoche
// civil de su fecha en la zona, incluso justo despues de un cambio de
// horario.
func TestLocalDateCollapsesToCivilMidnight(t *testing.T) {
	madrid, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Skipf("Europe/Madrid unavailable: %v", err)
	}
	// 2026-10-25 03:00 CEST -> 02:00 CET: 01:30Z sigue siendo el domingo
	// a las 02:30 CET; 23:30Z ya es lunes 00:30 CET.
	cases := []struct {
		instant string
		want    string
	}{
		{"2026-10-25 01:30", "2026-10-25"},
		{"2026-10-25 22:30", "2026-10-25"},
		{"2026-10-25 23:30", "2026-10-26"},
		{"2026-10-26 07:15", "2026-10-26"},
	}
	for _, tc := range cases {
		got := LocalDate(civilUTC(t, tc.instant), madrid)
		if got.Format("2006-01-02") != tc.want || got.Location() != time.UTC {
			t.Errorf("LocalDate(%q) = %v, want UTC carrier for Madrid date %s", tc.instant, got, tc.want)
		}
		if h, m, s := got.Clock(); h != 0 || m != 0 || s != 0 {
			t.Errorf("LocalDate(%q) clock = %02d:%02d:%02d, want midnight", tc.instant, h, m, s)
		}
	}
}

// TestWeekStartIsMondayBased: lunes propio, domingo de la semana
// anterior, y el paso de ano (viernes 2027-01-01 -> lunes 2026-12-28).
func TestWeekStartIsMondayBased(t *testing.T) {
	cases := []struct{ date, want string }{
		{"2026-11-16", "2026-11-16"}, // lunes
		{"2026-11-18", "2026-11-16"}, // miercoles
		{"2026-11-22", "2026-11-16"}, // domingo
		{"2026-11-23", "2026-11-23"}, // lunes siguiente
		{"2027-01-01", "2026-12-28"}, // viernes -> lunes de 2026
	}
	for _, tc := range cases {
		date, err := time.ParseInLocation("2006-01-02", tc.date, time.UTC)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.date, err)
		}
		if got := WeekStart(date); got.Format("2006-01-02") != tc.want {
			t.Errorf("WeekStart(%s) = %s, want %s", tc.date, got.Format("2006-01-02"), tc.want)
		}
	}
}
