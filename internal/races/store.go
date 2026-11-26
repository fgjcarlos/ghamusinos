package races

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrNotFound = errors.New("race or track not found")
var ErrInvalid = errors.New("invalid race")

type Draft struct {
	Name           string `json:"name"`
	RaceDate       string `json:"race_date"`
	DistanceM      int32  `json:"distance_m"`
	ElevationGainM int32  `json:"elevation_gain_m"`
	Priority       string `json:"priority"`
	Notes          string `json:"notes"`
}

type Race struct {
	ID uuid.UUID `json:"id"`
	Draft
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Track struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type Session struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	ScheduledOn string    `json:"scheduled_on"`
	Status      string    `json:"status"`
}

type Detail struct {
	Race
	Tracks   []Track   `json:"tracks"`
	Sessions []Session `json:"sessions"`
}

type Store struct{ q sqlc.Querier }

func NewStore(q sqlc.Querier) *Store { return &Store{q: q} }

func validate(v Draft) (pgtype.Date, error) {
	date, err := time.Parse(time.DateOnly, v.RaceDate)
	if err != nil || date.Year() < 1 || date.Format(time.DateOnly) != v.RaceDate ||
		v.Name == "" || utf8.RuneCountInString(v.Name) > 200 || utf8.RuneCountInString(v.Notes) > 2000 ||
		strings.ContainsRune(v.Name+v.Notes, 0) || v.DistanceM <= 0 || v.ElevationGainM < 0 ||
		(v.Priority != "A" && v.Priority != "B" && v.Priority != "C") {
		return pgtype.Date{}, ErrInvalid
	}
	return pgtype.Date{Time: date, Valid: true}, nil
}

func (s *Store) Create(ctx context.Context, user uuid.UUID, v Draft) (Race, error) {
	v.Name = strings.TrimSpace(v.Name)
	date, err := validate(v)
	if err != nil {
		return Race{}, err
	}
	row, err := s.q.CreateRace(ctx, sqlc.CreateRaceParams{UserID: dbID(user), Name: v.Name, RaceDate: date,
		DistanceM: v.DistanceM, ElevationGainM: v.ElevationGainM, Priority: v.Priority, Notes: v.Notes})
	return fromRow(row), err
}

func (s *Store) get(ctx context.Context, user, id uuid.UUID) (Race, error) {
	row, err := s.q.GetRace(ctx, sqlc.GetRaceParams{UserID: dbID(user), ID: dbID(id)})
	return fromRow(row), mapMissing(err)
}

func (s *Store) Get(ctx context.Context, user, id uuid.UUID) (Detail, error) {
	race, err := s.get(ctx, user, id)
	if err != nil {
		return Detail{}, err
	}
	tracks, err := s.q.ListRaceTracks(ctx, sqlc.ListRaceTracksParams{UserID: dbID(user), ID: dbID(id)})
	if err != nil {
		return Detail{}, err
	}
	sessions, err := s.q.ListRaceSessions(ctx, sqlc.ListRaceSessionsParams{UserID: dbID(user), RaceID: dbID(id)})
	if err != nil {
		return Detail{}, err
	}
	v := Detail{Race: race, Tracks: make([]Track, 0, len(tracks)), Sessions: make([]Session, 0, len(sessions))}
	for _, row := range tracks {
		v.Tracks = append(v.Tracks, Track{ID: uuid.UUID(row.ID.Bytes), Name: row.Name})
	}
	for _, row := range sessions {
		v.Sessions = append(v.Sessions, Session{ID: uuid.UUID(row.ID.Bytes), Title: row.Title,
			ScheduledOn: row.ScheduledOn.Time.Format(time.DateOnly), Status: row.Status})
	}
	return v, nil
}

func (s *Store) List(ctx context.Context, user uuid.UUID, limit, offset int32) ([]Race, int64, error) {
	rows, err := s.q.ListRaces(ctx, sqlc.ListRacesParams{UserID: dbID(user), Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountRaces(ctx, dbID(user))
	if err != nil {
		return nil, 0, err
	}
	data := make([]Race, 0, len(rows))
	for _, row := range rows {
		data = append(data, fromRow(row))
	}
	return data, total, nil
}

func (s *Store) Update(ctx context.Context, user, id uuid.UUID, v Draft) (Race, error) {
	v.Name = strings.TrimSpace(v.Name)
	date, err := validate(v)
	if err != nil {
		return Race{}, err
	}
	row, err := s.q.UpdateRace(ctx, sqlc.UpdateRaceParams{UserID: dbID(user), ID: dbID(id), Name: v.Name, RaceDate: date,
		DistanceM: v.DistanceM, ElevationGainM: v.ElevationGainM, Priority: v.Priority, Notes: v.Notes})
	return fromRow(row), mapMissing(err)
}

func (s *Store) Delete(ctx context.Context, user, id uuid.UUID) error {
	n, err := s.q.DeleteRace(ctx, sqlc.DeleteRaceParams{UserID: dbID(user), ID: dbID(id)})
	if n == 0 && err == nil {
		return ErrNotFound
	}
	return err
}

func (s *Store) SetTrack(ctx context.Context, user, id, track uuid.UUID, attached bool) error {
	if !attached {
		_, err := s.q.DetachRaceTrack(ctx, sqlc.DetachRaceTrackParams{UserID: dbID(user), ID: dbID(id), ID_2: dbID(track)})
		return mapMissing(err)
	}
	_, err := s.q.AttachRaceTrack(ctx, sqlc.AttachRaceTrackParams{UserID: dbID(user), ID: dbID(id), ID_2: dbID(track)})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // A racing delete removed one endpoint.
		return ErrNotFound
	}
	return mapMissing(err)
}

func mapMissing(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func dbID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func fromRow(row sqlc.Race) Race {
	return Race{ID: uuid.UUID(row.ID.Bytes), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		Draft: Draft{Name: row.Name, RaceDate: row.RaceDate.Time.Format(time.DateOnly),
			DistanceM: row.DistanceM, ElevationGainM: row.ElevationGainM, Priority: row.Priority, Notes: row.Notes}}
}
