// Package blocks stores training blocks: named date ranges that group weeks of
// training, optionally aimed at a target race.
package blocks

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("training block or race not found")
	ErrInvalid  = errors.New("invalid training block")
	ErrOverlap  = errors.New("training block overlaps an existing block")
)

type Draft struct {
	Name      string     `json:"name"`
	StartDate string     `json:"start_date"`
	EndDate   string     `json:"end_date"`
	RaceID    *uuid.UUID `json:"race_id,omitempty"`
	Notes     string     `json:"notes"`
}

type Block struct {
	ID uuid.UUID `json:"id"`
	Draft
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

// NewStore returns a store on pool. Writes take a per-user advisory lock inside
// a transaction, so they need the pool and not a single connection.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{q: sqlc.New(pool), pool: pool}
}

func validate(v Draft) (start, end pgtype.Date, err error) {
	from, fromErr := time.Parse(time.DateOnly, v.StartDate)
	to, toErr := time.Parse(time.DateOnly, v.EndDate)
	if fromErr != nil || from.Format(time.DateOnly) != v.StartDate || from.Year() < 1 ||
		toErr != nil || to.Format(time.DateOnly) != v.EndDate || to.Before(from) ||
		v.Name == "" || utf8.RuneCountInString(v.Name) > 200 || utf8.RuneCountInString(v.Notes) > 2000 ||
		strings.ContainsRune(v.Name+v.Notes, 0) {
		return pgtype.Date{}, pgtype.Date{}, ErrInvalid
	}
	return day(from), day(to), nil
}

// Create inserts a block. The per-user lock makes the overlap check hold under
// concurrent writes.
func (s *Store) Create(ctx context.Context, user uuid.UUID, v Draft) (Block, error) {
	v.Name = strings.TrimSpace(v.Name)
	start, end, err := validate(v)
	if err != nil {
		return Block{}, err
	}
	var row sqlc.TrainingBlock
	err = s.withUserLock(ctx, user, func(q *sqlc.Queries) error {
		if err := checkRace(ctx, q, user, v.RaceID); err != nil {
			return err
		}
		if err := checkOverlap(ctx, q, user, uuid.Nil, start, end); err != nil {
			return err
		}
		inserted, insertErr := q.CreateTrainingBlock(ctx, sqlc.CreateTrainingBlockParams{
			UserID: dbID(user), Name: v.Name, StartDate: start, EndDate: end, RaceID: dbOptID(v.RaceID), Notes: v.Notes})
		row = inserted
		return insertErr
	})
	return fromRow(row), err
}

// Get returns a block of the user. A missing or foreign block answers ErrNotFound.
func (s *Store) Get(ctx context.Context, user, id uuid.UUID) (Block, error) {
	row, err := s.q.GetTrainingBlock(ctx, sqlc.GetTrainingBlockParams{UserID: dbID(user), ID: dbID(id)})
	return fromRow(row), mapMissing(err)
}

// List returns one page of the user's blocks ordered by start date, and the total.
func (s *Store) List(ctx context.Context, user uuid.UUID, limit, offset int32) ([]Block, int64, error) {
	rows, err := s.q.ListTrainingBlocks(ctx, sqlc.ListTrainingBlocksParams{UserID: dbID(user), Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountTrainingBlocks(ctx, dbID(user))
	if err != nil {
		return nil, 0, err
	}
	data := make([]Block, 0, len(rows))
	for _, row := range rows {
		data = append(data, fromRow(row))
	}
	return data, total, nil
}

// Update replaces the editable fields of a block of the user.
func (s *Store) Update(ctx context.Context, user, id uuid.UUID, v Draft) (Block, error) {
	v.Name = strings.TrimSpace(v.Name)
	start, end, err := validate(v)
	if err != nil {
		return Block{}, err
	}
	var row sqlc.TrainingBlock
	err = s.withUserLock(ctx, user, func(q *sqlc.Queries) error {
		if _, err := q.GetTrainingBlock(ctx, sqlc.GetTrainingBlockParams{UserID: dbID(user), ID: dbID(id)}); err != nil {
			return mapMissing(err)
		}
		if err := checkRace(ctx, q, user, v.RaceID); err != nil {
			return err
		}
		if err := checkOverlap(ctx, q, user, id, start, end); err != nil {
			return err
		}
		updated, updateErr := q.UpdateTrainingBlock(ctx, sqlc.UpdateTrainingBlockParams{
			UserID: dbID(user), ID: dbID(id), Name: v.Name, StartDate: start, EndDate: end, RaceID: dbOptID(v.RaceID), Notes: v.Notes})
		row = updated
		return updateErr
	})
	return fromRow(row), err
}

// Delete removes a block of the user. Its race and sessions are kept.
func (s *Store) Delete(ctx context.Context, user, id uuid.UUID) error {
	n, err := s.q.DeleteTrainingBlock(ctx, sqlc.DeleteTrainingBlockParams{UserID: dbID(user), ID: dbID(id)})
	if n == 0 && err == nil {
		return ErrNotFound
	}
	return err
}

// withUserLock runs fn in a transaction that holds the user's advisory lock.
func (s *Store) withUserLock(ctx context.Context, user uuid.UUID, fn func(q *sqlc.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }() // No-op once committed.
	q := s.q.WithTx(tx)
	if err := q.LockUserTrainingBlocks(ctx, user.String()); err != nil {
		return err
	}
	if err := fn(q); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// checkRace rejects a target race that is missing or belongs to another user.
func checkRace(ctx context.Context, q *sqlc.Queries, user uuid.UUID, race *uuid.UUID) error {
	if race == nil {
		return nil
	}
	_, err := q.GetRace(ctx, sqlc.GetRaceParams{UserID: dbID(user), ID: dbID(*race)})
	return mapMissing(err)
}

func checkOverlap(ctx context.Context, q *sqlc.Queries, user, exclude uuid.UUID, start, end pgtype.Date) error {
	n, err := q.CountOverlappingTrainingBlocks(ctx, sqlc.CountOverlappingTrainingBlocksParams{
		UserID: dbID(user), ExcludeID: dbOptID(&exclude), StartDate: start, EndDate: end})
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrOverlap
	}
	return nil
}

func mapMissing(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func dbID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// dbOptID maps a nil pointer to NULL. uuid.Nil is never a stored id, so callers
// pass a nil pointer for "no value".
func dbOptID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return dbID(*id)
}

func day(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func fromOptID(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	id := uuid.UUID(v.Bytes)
	return &id
}

func fromRow(row sqlc.TrainingBlock) Block {
	return Block{ID: uuid.UUID(row.ID.Bytes), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		Draft: Draft{Name: row.Name, StartDate: row.StartDate.Time.Format(time.DateOnly),
			EndDate: row.EndDate.Time.Format(time.DateOnly), RaceID: fromOptID(row.RaceID), Notes: row.Notes}}
}
