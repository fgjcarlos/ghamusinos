package reports

import (
	"encoding/json"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// date construye una fecha de calendario (columna DATE) desde ISO.
func date(t *testing.T, iso string) pgtype.Date {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, iso)
	require.NoError(t, err)
	return pgtype.Date{Time: parsed, Valid: true}
}

// meters construye la columna NUMERIC de distancia o desnivel.
func meters(t *testing.T, v float64) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	require.NoError(t, n.Scan(strconv.FormatFloat(v, 'f', 2, 64)))
	return n
}

func activity(t *testing.T, sport string, startedAt time.Time, moving int32, distance, elevation float64, hr, power int16) sqlc.ListActivitiesInRangeRow {
	t.Helper()
	row := sqlc.ListActivitiesInRangeRow{SportType: sport, StartedAt: pgtype.Timestamptz{Time: startedAt, Valid: true},
		ElapsedSeconds: moving, MovingSeconds: moving, DistanceMeters: meters(t, distance), ElevationGainM: meters(t, elevation)}
	if hr > 0 {
		row.AvgHr = pgtype.Int2{Int16: hr, Valid: true}
	}
	if power > 0 {
		row.AvgPower = pgtype.Int2{Int16: power, Valid: true}
	}
	return row
}

func session(t *testing.T, iso string, status workouts.Status) sqlc.ListPlannedSessionsByRangeRow {
	t.Helper()
	return sqlc.ListPlannedSessionsByRangeRow{ScheduledOn: date(t, iso), Status: string(status)}
}

// zeroDays builds the zero-filled daily series de un periodo sin carga.
func zeroDays(iso string, days int) []DailyLoad {
	start, err := time.Parse(time.DateOnly, iso)
	if err != nil {
		panic(err)
	}
	out := make([]DailyLoad, 0, days)
	for i := range days {
		out = append(out, DailyLoad{Date: start.AddDate(0, 0, i).Format(time.DateOnly)})
	}
	return out
}

func TestBuild(t *testing.T) {
	madrid := zone(t, "Europe/Madrid")
	blockID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	raceID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	block := sqlc.TrainingBlock{ID: pgtype.UUID{Bytes: blockID, Valid: true}, Name: "Base aerobica",
		StartDate: date(t, "2027-01-04"), EndDate: date(t, "2027-01-10"),
		RaceID: pgtype.UUID{Bytes: raceID, Valid: true}}
	race := sqlc.Race{ID: pgtype.UUID{Bytes: raceID, Valid: true}, Name: "Media maraton",
		RaceDate: date(t, "2027-01-24"), Priority: "A"}

	cases := []struct {
		name string
		in   Input
		want Snapshot
	}{
		{
			name: "periodo sin actividades",
			in: Input{Kind: KindWeek, Now: at(t, "2026-03-09 09:00", madrid), Location: madrid,
				PeriodStart: at(t, "2026-03-02 00:00", madrid), PeriodEnd: at(t, "2026-03-08 00:00", madrid)},
			want: Snapshot{SchemaVersion: 1, Kind: KindWeek, PeriodStart: "2026-03-02", PeriodEnd: "2026-03-08", Days: 7,
				Load:     Load{Daily: zeroDays("2026-03-02", 7)},
				Previous: Previous{PeriodStart: "2026-02-23", PeriodEnd: "2026-03-01"}},
		},
		{
			name: "semana 2026-10-26 que cruza un mes",
			in: Input{Kind: KindWeek, Now: at(t, "2026-11-02 09:00", madrid), Location: madrid,
				PeriodStart: at(t, "2026-10-26 00:00", madrid), PeriodEnd: at(t, "2026-11-01 00:00", madrid),
				Load: []sqlc.TrainingLoadDaily{
					{Day: date(t, "2026-10-24"), Tss: 40, ActivityCount: 1},
					{Day: date(t, "2026-10-26"), Tss: 60, ActivityCount: 1},
					{Day: date(t, "2026-10-28"), Tss: 80, ActivityCount: 1},
					{Day: date(t, "2026-11-05"), Tss: 100, ActivityCount: 1}, // despues del periodo
				},
				Activities: []sqlc.ListActivitiesInRangeRow{
					activity(t, "Ride", at(t, "2026-10-26 18:00", madrid), 3600, 30000, 400, 150, 200),
					activity(t, "Run", at(t, "2026-10-28 07:00", madrid), 2400, 10000, 100, 160, 0),
					activity(t, "Walk", at(t, "2026-11-01 10:00", madrid), 1800, 3000, 20, 0, 0),
					activity(t, "Run", at(t, "2026-10-25 09:00", madrid), 1800, 6000, 0, 150, 0),       // periodo anterior
					activity(t, "Ride", at(t, "2026-11-03 08:00", madrid), 3600, 20000, 200, 140, 180), // fuera de ambos
				},
				Sessions: []sqlc.ListPlannedSessionsByRangeRow{
					session(t, "2026-10-26", workouts.StatusCompleted),
					session(t, "2026-10-28", workouts.StatusCompleted),
					session(t, "2026-10-30", workouts.StatusModified),
					session(t, "2026-11-01", workouts.StatusSkipped),
					session(t, "2026-10-20", workouts.StatusPlanned), // periodo anterior
				}},
			want: Snapshot{SchemaVersion: 1, Kind: KindWeek, PeriodStart: "2026-10-26", PeriodEnd: "2026-11-01", Days: 7,
				Load: Load{TSS: 140, Daily: []DailyLoad{
					{Date: "2026-10-26", TSS: 60, ActivityCount: 1},
					{Date: "2026-10-27"},
					{Date: "2026-10-28", TSS: 80, ActivityCount: 1},
					{Date: "2026-10-29"}, {Date: "2026-10-30"}, {Date: "2026-10-31"}, {Date: "2026-11-01"}},
					CTLStart: 39.06, ATLStart: 34.68, TSBStart: 4.38,
					CTLEnd: 36, ATLEnd: 22.16, TSBEnd: 13.84},
				Volume:      Volume{Activities: 3, DistanceM: 43000, ElevationGainM: 520, MovingSeconds: 7800},
				Performance: Performance{AvgEF: f(1.45), EFSamples: 2, AvgGAPSecPerKm: f(253.31), GAPSamples: 1},
				Plan:        Plan{Completed: 2, Skipped: 1, Modified: 1, Adherence: f(0.75)},
				Previous: Previous{PeriodStart: "2026-10-19", PeriodEnd: "2026-10-25", Activities: 1, TSS: 40,
					DistanceM: 6000, AvgEF: f(1.33), AvgGAPSecPerKm: f(300)}},
		},
		{
			name: "bloque con carrera",
			in: Input{Kind: KindBlock, Now: at(t, "2027-01-08 10:00", madrid), Location: madrid,
				PeriodStart: at(t, "2027-01-04 00:00", madrid), PeriodEnd: at(t, "2027-01-10 00:00", madrid),
				Block: &block, Race: &race,
				Load:       []sqlc.TrainingLoadDaily{{Day: date(t, "2027-01-06"), Tss: 55, ActivityCount: 1}},
				Activities: []sqlc.ListActivitiesInRangeRow{activity(t, "Run", at(t, "2027-01-05 07:00", madrid), 3600, 12000, 120, 0, 0)},
				Sessions:   []sqlc.ListPlannedSessionsByRangeRow{session(t, "2027-01-05", workouts.StatusPlanned)}},
			want: Snapshot{SchemaVersion: 1, Kind: KindBlock, PeriodStart: "2027-01-04", PeriodEnd: "2027-01-10", Days: 7,
				Block: &BlockRef{ID: blockID, Name: "Base aerobica"},
				Race:  &RaceRef{ID: raceID, Name: "Media maraton", RaceDate: "2027-01-24", Priority: "A", DaysToRace: 16},
				Load: Load{TSS: 55, Daily: []DailyLoad{
					{Date: "2027-01-04"}, {Date: "2027-01-05"},
					{Date: "2027-01-06", TSS: 55, ActivityCount: 1},
					{Date: "2027-01-07"}, {Date: "2027-01-08"}, {Date: "2027-01-09"}, {Date: "2027-01-10"}},
					CTLEnd: 1.18, ATLEnd: 4.13, TSBEnd: -2.96},
				Volume:      Volume{Activities: 1, DistanceM: 12000, ElevationGainM: 120, MovingSeconds: 3600},
				Performance: Performance{AvgGAPSecPerKm: f(316.63), GAPSamples: 1},
				Plan:        Plan{Planned: 1, Adherence: f(0)},
				Previous:    Previous{PeriodStart: "2026-12-28", PeriodEnd: "2027-01-03"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Build(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestBuildRejectsInvalidInput(t *testing.T) {
	madrid := zone(t, "Europe/Madrid")
	base := Input{Now: at(t, "2026-11-02 09:00", madrid), Location: madrid,
		PeriodStart: at(t, "2026-10-26 00:00", madrid), PeriodEnd: at(t, "2026-11-01 00:00", madrid)}
	for _, tc := range []struct {
		name string
		in   Input
	}{
		{"tipo desconocido", func() Input { in := base; in.Kind = "day"; return in }()},
		{"rango invertido", func() Input {
			in := base
			in.Kind = KindWeek
			in.PeriodEnd = in.PeriodStart.AddDate(0, 0, -1)
			return in
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Build(tc.in)
			require.ErrorIs(t, err, ErrInvalid)
		})
	}
}

// TestSnapshotJSONFieldNames fija los nombres JSON del payload version 1: son
// parte del contrato que documenta docs/architecture/report-snapshot.md.
func TestSnapshotJSONFieldNames(t *testing.T) {
	blockID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	snapshot := Snapshot{SchemaVersion: 1, Kind: KindBlock, PeriodStart: "2027-01-04", PeriodEnd: "2027-01-10", Days: 7,
		Block: &BlockRef{ID: blockID, Name: "Base"}, Race: nil,
		Load:        Load{Daily: []DailyLoad{{Date: "2027-01-04", TSS: 55, ActivityCount: 2}}, CTLEnd: 1.18},
		Volume:      Volume{Activities: 1},
		Performance: Performance{EFSamples: 0},
		Plan:        Plan{Adherence: nil},
		Previous:    Previous{PeriodStart: "2026-12-28", PeriodEnd: "2027-01-03"}}
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)

	var members map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &members))
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	require.Equal(t, []string{"block", "days", "kind", "load", "performance", "period_end", "period_start", "plan",
		"previous", "race", "schema_version", "volume"}, names)

	require.JSONEq(t, `{"id":"11111111-1111-1111-1111-111111111111","name":"Base"}`, string(members["block"]))
	require.JSONEq(t, `null`, string(members["race"]))
	require.JSONEq(t, `{"activities":1,"distance_m":0,"elevation_gain_m":0,"moving_seconds":0}`, string(members["volume"]))
	require.JSONEq(t, `{"avg_ef":null,"avg_gap_sec_per_km":null,"ef_samples":0,"gap_samples":0}`, string(members["performance"]))
	require.JSONEq(t, `{"adherence":null,"completed":0,"modified":0,"planned":0,"skipped":0}`, string(members["plan"]))
	require.JSONEq(t, `{"avg_ef":null,"avg_gap_sec_per_km":null,"activities":0,"distance_m":0,"elevation_gain_m":0,"period_end":"2027-01-03","period_start":"2026-12-28","tss":0}`, string(members["previous"]))
	require.JSONEq(t, `{"atl_end":0,"atl_start":0,"ctl_end":1.18,"ctl_start":0,"daily":[{"activity_count":2,"date":"2027-01-04","tss":55}],"tsb_end":0,"tsb_start":0,"tss":0}`, string(members["load"]))
}
