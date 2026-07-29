package strava

import (
	"context"
	"fmt"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// SQLCTokenStore adapta sqlc.Querier a la interfaz TokenStore.
// Es la implementación de producción: persiste los tokens cifrados en
// la tabla strava_tokens mediante UpsertStravaTokens.
//
// Aceptamos la interfaz Querier (no *sqlc.Queries) para mantener la
// flexibilidad del wiring: en producción el Server del paquete http
// recibe sqlc.Querier, y podemos inyectar la misma instancia al store
// sin casteos.
type SQLCTokenStore struct {
	q sqlc.Querier
}

// NewSQLCTokenStore devuelve un store listo para usar a partir de un
// sqlc.Querier. Si q es nil, las llamadas a SaveTokens devolverán error
// (en vez de panic nil-deref) — útil cuando el wiring falla silenciosamente.
func NewSQLCTokenStore(q sqlc.Querier) *SQLCTokenStore {
	return &SQLCTokenStore{q: q}
}

// SaveTokens implementa TokenStore. Convierte userID string → UUID,
// construye los parámetros SQLC y delega en UpsertStravaTokens.
func (s *SQLCTokenStore) SaveTokens(ctx context.Context, t PersistedTokens) error {
	if s == nil || s.q == nil {
		return fmt.Errorf("strava: SQLCTokenStore no inicializado")
	}
	parsed, err := uuid.Parse(t.UserID)
	if err != nil {
		return fmt.Errorf("strava: user_id inválido: %w", err)
	}

	_, err = s.q.UpsertStravaTokens(ctx, sqlc.UpsertStravaTokensParams{
		UserID:        pgtype.UUID{Bytes: parsed, Valid: true},
		AccessCipher:  t.AccessCipher,
		RefreshCipher: t.RefreshCipher,
		ExpiresAt:     pgtype.Timestamptz{Time: t.ExpiresAt, Valid: true},
		AthleteID:     t.AthleteID,
		Scopes:        t.Scopes,
	})
	return err
}