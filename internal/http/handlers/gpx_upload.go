package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type UploadGPXStore interface {
	FindByHash(context.Context, pgtype.UUID, string) (*gpx.StoredTrack, error)
	CreateDetail(context.Context, *gpx.Track, *gpx.Analysis, []gpx.Climb, []gpx.RiskZone, *gpx.Climb) (*gpx.StoredTrackDetail, error)
}

func UploadGPX(
	store UploadGPXStore,
	parser gpx.GPXParser,
	validator gpx.GPXValidator,
	analyzer gpx.GPXAnalyzer,
	climbDetector gpx.ClimbDetector,
	riskDetector gpx.RiskZoneDetector,
	typeDetector gpx.TrackTypeDetector,
	hasher gpx.GPXHasher,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		user := auth.AuthUser(r.Context())
		if user == nil {
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}

		data, filename, err := parseMultipartFile(r)
		if err != nil {
			status := http.StatusUnprocessableEntity
			if errors.Is(err, errGPXTooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			WriteProblem(w, ProblemDetail{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: err.Error(), Instance: requestID})
			return
		}

		hash, err := hasher.Hash(bytes.NewReader(data))
		if err != nil {
			WriteProblem(w, NewInternalError("failed to hash GPX", requestID))
			return
		}
		userID := parseUserID(user.ID)
		existing, findErr := store.FindByHash(r.Context(), userID, hash)
		if findErr == nil && existing != nil {
			writeDuplicateProblem(w, requestID, existing.Track.ID)
			return
		}
		if findErr != nil && !errors.Is(findErr, pgx.ErrNoRows) {
			WriteProblem(w, NewInternalError("failed to check duplicate GPX", requestID))
			return
		}

		track, err := parser.Parse(bytes.NewReader(data))
		if err != nil {
			WriteProblem(w, ProblemDetail{Type: "about:blank", Title: "Unprocessable Entity", Status: http.StatusUnprocessableEntity, Detail: err.Error(), Instance: requestID})
			return
		}
		if err := validator.Validate(int64(len(data)), track); err != nil {
			WriteProblem(w, ProblemDetail{Type: "about:blank", Title: "Unprocessable Entity", Status: http.StatusUnprocessableEntity, Detail: err.Error(), Instance: requestID})
			return
		}
		track.UserID, track.FileHash, track.FileSizeBytes = userID, hash, int64(len(data))
		if track.Name == "" {
			track.Name = filename
		}

		analysis := analyzeUploadedTrack(track, analyzer)
		climbs, err := climbDetector.FindAllClimbs(track)
		if err != nil {
			WriteProblem(w, NewInternalError("failed to analyze GPX climbs", requestID))
			return
		}
		kingClimb, err := climbDetector.FindKingClimb(track, climbs)
		if err != nil {
			WriteProblem(w, NewInternalError("failed to analyze GPX king climb", requestID))
			return
		}
		markKingClimb(climbs, kingClimb)
		muros, err := climbDetector.FindMuros(track)
		if err != nil {
			WriteProblem(w, NewInternalError("failed to analyze GPX walls", requestID))
			return
		}
		recoveryZones, err := climbDetector.FindRecoveryZones(track, climbs)
		if err != nil {
			WriteProblem(w, NewInternalError("failed to analyze GPX recovery zones", requestID))
			return
		}
		kmVertical, err := climbDetector.FindKmVertical(track)
		if err != nil {
			WriteProblem(w, NewInternalError("failed to analyze GPX vertical segment", requestID))
			return
		}
		riskZones, err := riskDetector.Detect(track)
		if err != nil {
			WriteProblem(w, NewInternalError("failed to analyze GPX risks", requestID))
			return
		}
		trackType, err := typeDetector.Detect(track)
		if err != nil {
			WriteProblem(w, NewInternalError("failed to classify GPX", requestID))
			return
		}
		track.TrackType, track.Direction = trackType.Type, trackType.Direction

		detail, err := store.CreateDetail(r.Context(), track, analysis, climbs, riskZones, kingClimb)
		if err != nil {
			WriteProblem(w, NewInternalError("failed to persist GPX", requestID))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		escribirJSON(w, struct {
			*gpx.StoredTrackDetail
			Muros         []gpx.Muro            `json:"muros"`
			RecoveryZones []gpx.RecoveryZone    `json:"recovery_zones"`
			KmVertical    *gpx.KmVerticalResult `json:"km_vertical"`
		}{StoredTrackDetail: detail, Muros: muros, RecoveryZones: recoveryZones, KmVertical: kmVertical})
	})
}

var errGPXTooLarge = errors.New("file exceeds 10 MB limit")

func parseMultipartFile(r *http.Request) ([]byte, string, error) {
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, "", fmt.Errorf("file field is required")
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// The upload has already been consumed; there is no useful recovery path.
			return
		}
	}()
	if header.Size > gpx.MaxFileSize {
		return nil, "", errGPXTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(file, gpx.MaxFileSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("failed to read GPX file")
	}
	if int64(len(data)) > gpx.MaxFileSize {
		return nil, "", errGPXTooLarge
	}
	if int64(len(data)) < gpx.MinFileSize {
		return nil, "", fmt.Errorf("file must be at least 1 KB")
	}
	return data, header.Filename, nil
}

func writeDuplicateProblem(w http.ResponseWriter, requestID string, trackID pgtype.UUID) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusConflict)
	escribirJSON(w, struct {
		ProblemDetail
		TrackID string `json:"track_id"`
	}{ProblemDetail: ProblemDetail{Type: "about:blank", Title: "Conflict", Status: http.StatusConflict, Detail: "duplicate track", Instance: requestID}, TrackID: trackID.String()})
}

func markKingClimb(climbs []gpx.Climb, king *gpx.Climb) {
	if king == nil {
		return
	}
	for i := range climbs {
		if climbs[i].StartIdx == king.StartIdx && climbs[i].EndIdx == king.EndIdx {
			climbs[i].IsKingClimb = true
		}
	}
}

func analyzeUploadedTrack(track *gpx.Track, analyzer gpx.GPXAnalyzer) *gpx.Analysis {
	points := track.Points
	distance := analyzer.CalculatePathDistance(points)
	dPlus, dMinus := analyzer.CalculateTotalDPlus(points, 30), analyzer.CalculateTotalDMinus(points, 30)
	movingTime := analyzer.CalculateMovingTime(points, 60)
	slopes := make([]float64, 0, len(points)-1)
	var maxSlope float64
	for i := 1; i < len(points); i++ {
		d := analyzer.CalculateDistance(points[i-1], points[i])
		slope := 0.0
		if d > 0 && points[i-1].Ele != nil && points[i].Ele != nil {
			slope = (*points[i].Ele - *points[i-1].Ele) / d * 100
		}
		slopes = append(slopes, slope)
		maxSlope = math.Max(maxSlope, math.Abs(slope))
	}
	maximum, minimum := elevationRange(points)
	label := analyzer.CalculateDifficulty(distance, dPlus, maxSlope)
	return &gpx.Analysis{
		DistanceM: distance, MovingTimeS: movingTime, DPlusM: dPlus, DMinusM: dMinus,
		MaxElevationM: maximum, MinElevationM: minimum,
		AverageSlopePct: analyzer.CalculateAverageSlope(distance, dPlus), MaxSlopePct: maxSlope,
		EffortIndex: analyzer.CalculateEffortIndex(distance/1000, dPlus), ITRAPoints: analyzer.CalculateITRAPoints(distance/1000, dPlus),
		LegBreakerIndex: analyzer.CalculateLegBreakerIndex(slopes), EstimatedVAM: analyzer.CalculateEstimatedVAM(distance, dPlus, float64(movingTime)),
		DifficultyScore: difficultyScore(label), DifficultyLabel: label, RunnabilityPct: analyzer.CalculateRunnabilityIndex(track) * 100,
	}
}

func elevationRange(points []gpx.Point) (*float64, *float64) {
	var maximum, minimum *float64
	for _, point := range points {
		if point.Ele == nil {
			continue
		}
		if maximum == nil || *point.Ele > *maximum {
			value := *point.Ele
			maximum = &value
		}
		if minimum == nil || *point.Ele < *minimum {
			value := *point.Ele
			minimum = &value
		}
	}
	return maximum, minimum
}

func difficultyScore(label gpx.DifficultyLabel) int {
	switch label {
	case gpx.DifficultyBeginner:
		return 20
	case gpx.DifficultyIntermediate:
		return 40
	case gpx.DifficultyAdvanced:
		return 60
	case gpx.DifficultyPro:
		return 80
	default:
		return 0
	}
}
