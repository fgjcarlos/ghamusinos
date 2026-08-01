package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/crypto"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// TokenRefresher defines the interface for refreshing Strava tokens.
type TokenRefresher interface {
	Refresh(ctx context.Context, refreshToken string) (*strava.TokenSet, error)
}

// TokenQuerier defines the interface for retrieving and persisting tokens.
type TokenQuerier interface {
	GetStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) (sqlc.StravaToken, error)
	UpsertStravaTokens(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error)
	CreateSyncSession(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error)
	GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error)
	UpdateSyncSessionStatus(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error)
	UpdateSyncSessionProgress(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error)
	UpsertActivity(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error)
	GetActivityEventByExternalID(ctx context.Context, externalID string) (sqlc.ActivityEvent, error)
	MarkActivityEventProcessed(ctx context.Context, id pgtype.UUID) error
}

// GetValidToken retrieves the current access token for a user, refreshing if necessary.
// If the token expires within 5 minutes, it calls the Strava API to refresh.
// It returns the decrypted access token string ready for use.
func GetValidToken(
	ctx context.Context,
	querier TokenQuerier,
	cipherKey []byte,
	refresher TokenRefresher,
	userID pgtype.UUID,
) (string, error) {
	// Load stored tokens from DB
	stored, err := querier.GetStravaTokensByUserID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("jobs: failed to load stored tokens: %w", err)
	}

	// Decrypt access token
	accessTokenBytes, err := crypto.Decrypt(stored.AccessCipher, cipherKey)
	if err != nil {
		return "", fmt.Errorf("jobs: failed to decrypt access token: %w", err)
	}
	accessToken := string(accessTokenBytes)

	// Check if token is still valid (expires more than 5 minutes from now)
	if stored.ExpiresAt.Valid && stored.ExpiresAt.Time.After(time.Now().Add(5*time.Minute)) {
		return accessToken, nil
	}

	// Token expires soon or is already expired; refresh it
	decryptedRefresh, err := crypto.Decrypt(stored.RefreshCipher, cipherKey)
	if err != nil {
		return "", fmt.Errorf("jobs: failed to decrypt refresh token: %w", err)
	}

	// Call Strava API to get new tokens
	newTokenSet, err := refresher.Refresh(ctx, string(decryptedRefresh))
	if err != nil {
		return "", fmt.Errorf("jobs: strava refresh failed: %w", err)
	}

	// Encrypt new tokens
	newAccessCipher, err := crypto.Encrypt([]byte(newTokenSet.AccessToken), cipherKey)
	if err != nil {
		return "", fmt.Errorf("jobs: failed to encrypt new access token: %w", err)
	}

	newRefreshCipher, err := crypto.Encrypt([]byte(newTokenSet.RefreshToken), cipherKey)
	if err != nil {
		return "", fmt.Errorf("jobs: failed to encrypt new refresh token: %w", err)
	}

	// Persist new tokens to DB
	_, err = querier.UpsertStravaTokens(ctx, sqlc.UpsertStravaTokensParams{
		UserID:        userID,
		AccessCipher:  newAccessCipher,
		RefreshCipher: newRefreshCipher,
		ExpiresAt:     pgtype.Timestamptz{Time: newTokenSet.ExpiresAt, Valid: true},
		AthleteID:     newTokenSet.AthleteID,
		Scopes:        newTokenSet.Scopes,
	})
	if err != nil {
		return "", fmt.Errorf("jobs: failed to persist refreshed tokens: %w", err)
	}

	return newTokenSet.AccessToken, nil
}
