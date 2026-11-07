package handlers

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// fakeAIStore es un AIAnalysisStore en memoria para los tests de los
// handlers de IA (fase 1.5, T7). No toca la base de datos: cada método
// responde con lo que el test ha preparado.
//
// Los campos exportados por el test son el contrato observable: created
// (filas creadas), listParams y getParams (consultas hechas) y listed/got
// (lo que devuelve la lectura).
type fakeAIStore struct {
	// prefs es la fila de preferencias que devuelve GetUserPreferencesByID.
	prefs sqlc.GetUserPreferencesByIDRow
	// prefsErr fuerza un error en la lectura de preferencias.
	prefsErr error
	// activity son las actividades que "pertenecen" al usuario. Una
	// actividad que no está en el mapa no existe (pgx.ErrNoRows).
	activity map[string]sqlc.GetActivityForUserRow
	// activityErr fuerza un error en la lectura de la actividad.
	activityErr error
	// created registra cada fila creada.
	created []sqlc.CreateAIAnalysisParams
	// createErr fuerza un error al crear la fila.
	createErr error
	// listed son las filas que devuelve el listado.
	listed []sqlc.AiAnalysis
	// listErr fuerza un error en el listado.
	listErr error
	// listParams registra el usuario y el límite de cada listado.
	listParams []sqlc.ListAIAnalysesParams
	// got es la fila que devuelve la lectura de un análisis concreto.
	got sqlc.AiAnalysis
	// gotErr fuerza un error en la lectura de un análisis concreto.
	gotErr error
	// getParams registra el usuario, el tipo y el scope de cada lectura.
	getParams []sqlc.GetAIAnalysisParams
}

func (m *fakeAIStore) GetUserPreferencesByID(_ context.Context, _ pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error) {
	if m.prefsErr != nil {
		return sqlc.GetUserPreferencesByIDRow{}, m.prefsErr
	}
	return m.prefs, nil
}

func (m *fakeAIStore) GetActivityForUser(_ context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	if m.activityErr != nil {
		return sqlc.GetActivityForUserRow{}, m.activityErr
	}
	row, ok := m.activity[arg.ID.String()]
	if !ok {
		return sqlc.GetActivityForUserRow{}, pgx.ErrNoRows
	}
	// La consulta real filtra por (id, user_id): una actividad que es de
	// otro usuario no existe para este usuario.
	if row.UserID.Valid && row.UserID != arg.UserID {
		return sqlc.GetActivityForUserRow{}, pgx.ErrNoRows
	}
	return row, nil
}

func (m *fakeAIStore) CreateAIAnalysis(_ context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	m.created = append(m.created, arg)
	if m.createErr != nil {
		return sqlc.AiAnalysis{}, m.createErr
	}
	moment := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return sqlc.AiAnalysis{
		ID:        pgtype.UUID{Bytes: [16]byte{9}, Valid: true},
		UserID:    arg.UserID,
		Kind:      arg.Kind,
		Scope:     arg.Scope,
		Status:    "pending",
		CreatedAt: pgtype.Timestamptz{Time: moment, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: moment, Valid: true},
	}, nil
}

func (m *fakeAIStore) GetAIAnalysis(_ context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	m.getParams = append(m.getParams, arg)
	if m.gotErr != nil {
		return sqlc.AiAnalysis{}, m.gotErr
	}
	return m.got, nil
}

func (m *fakeAIStore) ListAIAnalyses(_ context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	m.listParams = append(m.listParams, arg)
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listed, nil
}

// fakeAIInserter es un jobs.RiverJobInserter en memoria: registra los
// argumentos y las opciones de cada encolado para que el test verifique el
// presupuesto de intentos y el objetivo del job.
type fakeAIInserter struct {
	// calls registra cada encolado.
	calls []fakeAIInsertCall
	// err fuerza un error en el encolado.
	err error
}

// fakeAIInsertCall es un encolado registrado.
type fakeAIInsertCall struct {
	args river.JobArgs
	opts *river.InsertOpts
}

func (m *fakeAIInserter) Insert(_ context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	m.calls = append(m.calls, fakeAIInsertCall{args: args, opts: opts})
	if m.err != nil {
		return nil, m.err
	}
	return &rivertype.JobInsertResult{}, nil
}

// Stubs de la interfaz sqlc.Querier para las consultas de ai_analysis.
// La tabla ai_analysis (migración 00015) añadió métodos a la interfaz
// compartida, así que cada mock que la implementa completa necesita estos
// stubs. Los handlers de IA (fase 1.5) usarán mocks propios con
// comportamiento; aquí solo se cumple el contrato de interfaz.

// CreateAIAnalysis stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetAIAnalysis stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// ListAIAnalyses stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	return nil, nil
}

// CompleteAIAnalysis stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// FailAIAnalysis stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// CreateAIAnalysis stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetAIAnalysis stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// ListAIAnalyses stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	return nil, nil
}

// CompleteAIAnalysis stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// FailAIAnalysis stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// CreateAIAnalysis stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetAIAnalysis stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// ListAIAnalyses stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	return nil, nil
}

// CompleteAIAnalysis stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// FailAIAnalysis stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// CreateAIAnalysis stub para mockMeQuerier.
func (m *mockMeQuerier) CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetAIAnalysis stub para mockMeQuerier.
func (m *mockMeQuerier) GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// ListAIAnalyses stub para mockMeQuerier.
func (m *mockMeQuerier) ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	return nil, nil
}

// CompleteAIAnalysis stub para mockMeQuerier.
func (m *mockMeQuerier) CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// FailAIAnalysis stub para mockMeQuerier.
func (m *mockMeQuerier) FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}

// GetActivityForUser stub para activitiesMockQuerier.
func (m *activitiesMockQuerier) GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	return sqlc.GetActivityForUserRow{}, nil
}

// GetActivityForUser stub para dashboardMockQuerier.
func (m *dashboardMockQuerier) GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	return sqlc.GetActivityForUserRow{}, nil
}

// GetActivityForUser stub para preferencesMockQuerier.
func (m *preferencesMockQuerier) GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	return sqlc.GetActivityForUserRow{}, nil
}

// GetActivityForUser stub para mockMeQuerier.
func (m *mockMeQuerier) GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	return sqlc.GetActivityForUserRow{}, nil
}
