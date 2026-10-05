package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/metrics"
)

// RecalcTrainingLoadArgs triggers a training-load recalculation for a user.
// From/To are optional: when nil, the worker derives the window from
// dashboard_metadata (or, on first run, from the user's first activity).
type RecalcTrainingLoadArgs struct {
	UserID string  `json:"user_id"`
	From   *string `json:"from,omitempty"`
	To     *string `json:"to,omitempty"`
}

// Kind is the River job-kind identifier.
func (RecalcTrainingLoadArgs) Kind() string { return "recalc_training_load" }

// RecalcWindowStrategy chooses how the worker derives its recalc window when
// From/To are not supplied.
type RecalcWindowStrategy int

const (
	// WindowFromDashboardMetadata reads dashboard_metadata.last_recalc_at
	// and falls back to FirstActivityForUser when missing.
	WindowFromDashboardMetadata RecalcWindowStrategy = iota
	// WindowFromFirstActivity always derives the window from the user's
	// earliest activity (useful for /dashboard/recalc manual trigger).
	WindowFromFirstActivity
)

// TrainingLoadStore is the SQLC subset required by the recalc worker.
type TrainingLoadStore interface {
	GetDashboardMetadata(ctx context.Context, userID pgtype.UUID) (sqlc.DashboardMetadatum, error)
	UpsertDashboardMetadata(ctx context.Context, arg sqlc.UpsertDashboardMetadataParams) (sqlc.DashboardMetadatum, error)
	UpsertTrainingLoadDaily(ctx context.Context, arg sqlc.UpsertTrainingLoadDailyParams) (sqlc.TrainingLoadDaily, error)
	ListTrainingLoadRange(ctx context.Context, arg sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error)
}

// ActivityRangeAdapter expone las queries de actividades necesarias para
// ComputeDailyLoad. Cumple metrics.ActivityRangeQuerier.
type ActivityRangeAdapter interface {
	ListActivitiesInRange(ctx context.Context, userID pgtype.UUID, from, to time.Time) ([]metrics.ActivitySummary, error)
	FirstActivityForUser(ctx context.Context, userID pgtype.UUID) (pgtype.Timestamptz, error)
}

// userPhysiologyLoaderAdapter implementa metrics.UserLoader leyendo de la
// fila `users`.
type userPhysiologyLoaderAdapter interface {
	LoadUserPhysiology(ctx context.Context, userID pgtype.UUID) (metrics.UserPhysiology, error)
}

// RecalcTrainingLoadWorker recalcula training_load_daily para un usuario.
// La ventana de recálculo se elige así:
//  1. Args.From/To si vienen (job manual desde /dashboard/recalc).
//  2. last_recalc_at de dashboard_metadata, si existe y strategy=FromMetadata.
//  3. first_activity del usuario.
//  4. Si el usuario no tiene actividades, status=`ok` con 0 filas (no error).
//
// Cada fila diaria se upserta con `computed_at=now()`. El TTL/EMA se aplica
// desde el cliente (`metrics.CTL/ATL`), aquí solo se persiste TSS agregado.
type RecalcTrainingLoadWorker struct {
	river.WorkerDefaults[RecalcTrainingLoadArgs]

	store          TrainingLoadStore
	activityLoader ActivityRangeAdapter
	userLoader     userPhysiologyLoaderAdapter
	logger         *slog.Logger
	now            func() time.Time
	strategy       RecalcWindowStrategy
}

// NewRecalcTrainingLoadWorker builds a worker with all dependencies wired.
// logger can be nil (slog.Default() is used). now can be nil
// (time.Now is used). strategy defaults to WindowFromDashboardMetadata.
func NewRecalcTrainingLoadWorker(
	store TrainingLoadStore,
	activityLoader ActivityRangeAdapter,
	userLoader userPhysiologyLoaderAdapter,
	logger *slog.Logger,
	now func() time.Time,
	strategy RecalcWindowStrategy,
) *RecalcTrainingLoadWorker {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return &RecalcTrainingLoadWorker{
		store:          store,
		activityLoader: activityLoader,
		userLoader:     userLoader,
		logger:         logger,
		now:            now,
		strategy:       strategy,
	}
}

// Work is the River entry point. It parses the user_id from job args and
// delegates the actual work to recomputeForUser (testable without River).
func (w *RecalcTrainingLoadWorker) Work(ctx context.Context, job *river.Job[RecalcTrainingLoadArgs]) error {
	var userID pgtype.UUID
	if err := userID.Scan(job.Args.UserID); err != nil {
		return fmt.Errorf("recalc_training_load: parse user_id %q: %w", job.Args.UserID, err)
	}
	return w.recomputeForUser(ctx, userID, job.Args)
}

// recomputeForUser implements the worker logic so tests can exercise the
// happy / sad path without going through River.
func (w *RecalcTrainingLoadWorker) recomputeForUser(ctx context.Context, userID pgtype.UUID, args RecalcTrainingLoadArgs) error {
	from, to, err := w.resolveWindow(ctx, userID, args)
	if err != nil {
		w.recordError(ctx, userID, err)
		return err
	}
	if to.Before(from) {
		err := fmt.Errorf("recalc window invalid: to (%s) before from (%s)", to, from)
		w.recordError(ctx, userID, err)
		return err
	}

	rows, err := metrics.ComputeDailyLoad(ctx, metrics.ComputeDailyLoadDeps{
		ActivityRangeQuerier: w.activityLoader,
		UserLoader:           w.userLoader,
	}, userID, from, to)
	if err != nil {
		w.recordError(ctx, userID, err)
		return fmt.Errorf("compute daily load: %w", err)
	}

	totalRows := 0
	for _, row := range rows {
		if _, err := w.store.UpsertTrainingLoadDaily(ctx, sqlc.UpsertTrainingLoadDailyParams{
			UserID:         userID,
			Day:            pgDate(row.Day),
			Tss:            row.TSS,
			ActivityCount:  int32(row.ActivityCount),
			DistanceM:      row.DistanceM,
			ElevationGainM: row.ElevationGainM,
			ComputedAt:     pgtype.Timestamptz{Time: w.now(), Valid: true},
		}); err != nil {
			w.recordError(ctx, userID, fmt.Errorf("upsert training_load_daily: %w", err))
			return err
		}
		totalRows++
	}

	if err := w.recordOK(ctx, userID, totalRows); err != nil {
		return fmt.Errorf("update dashboard_metadata: %w", err)
	}

	w.logger.Info("recalc_training_load ok",
		slog.String("user_id", userID.String()),
		slog.Int("rows", totalRows),
		slog.Time("from", from),
		slog.Time("to", to),
	)
	return nil
}

// resolveWindow implements the window-selection strategy documented above.
func (w *RecalcTrainingLoadWorker) resolveWindow(ctx context.Context, userID pgtype.UUID, args RecalcTrainingLoadArgs) (time.Time, time.Time, error) {
	if args.From != nil && args.To != nil {
		from, err := time.Parse(time.RFC3339, *args.From)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse from: %w", err)
		}
		to, err := time.Parse(time.RFC3339, *args.To)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse to: %w", err)
		}
		return from, to, nil
	}

	if w.strategy == WindowFromDashboardMetadata {
		meta, err := w.store.GetDashboardMetadata(ctx, userID)
		if err == nil && meta.LastRecalcAt.Valid {
			return meta.LastRecalcAt.Time, w.now(), nil
		}
		// fallthrough to first-activity when meta row absent or last_recalc_at NULL
	}

	first, err := w.activityLoader.FirstActivityForUser(ctx, userID)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("first activity for user: %w", err)
	}
	if !first.Valid {
		// no activities yet: return a degenerate window that triggers 0 rows
		return w.now(), w.now(), nil
	}
	return first.Time, w.now(), nil
}

func (w *RecalcTrainingLoadWorker) recordOK(ctx context.Context, userID pgtype.UUID, rows int) error {
	now := pgtype.Timestamptz{Time: w.now(), Valid: true}
	_, err := w.store.UpsertDashboardMetadata(ctx, sqlc.UpsertDashboardMetadataParams{
		UserID:           userID,
		LastRecalcAt:     now,
		LastRecalcStatus: pgtype.Text{String: "ok", Valid: true},
		LastRecalcError:  pgtype.Text{Valid: false},
		TrainingLoadRows: int32(rows),
		UpdatedAt:        now,
	})
	return err
}

func (w *RecalcTrainingLoadWorker) recordError(ctx context.Context, userID pgtype.UUID, cause error) {
	now := pgtype.Timestamptz{Time: w.now(), Valid: true}
	msg := cause.Error()
	if _, err := w.store.UpsertDashboardMetadata(ctx, sqlc.UpsertDashboardMetadataParams{
		UserID:           userID,
		LastRecalcAt:     now,
		LastRecalcStatus: pgtype.Text{String: "error", Valid: true},
		LastRecalcError:  pgtype.Text{String: msg, Valid: true},
		TrainingLoadRows: 0,
		UpdatedAt:        now,
	}); err != nil {
		w.logger.Error("recalc_training_load: persist error status failed",
			slog.String("user_id", userID.String()),
			slog.String("cause", cause.Error()),
			slog.String("persist_error", err.Error()),
		)
	}
}

// pgDate wraps a time.Time as pgtype.Date at UTC-day granularity.
func pgDate(t time.Time) pgtype.Date {
	t = t.UTC()
	return pgtype.Date{
		Time:             time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC),
		Valid:            true,
		InfinityModifier: pgtype.Finite,
	}
}
