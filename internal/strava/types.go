package strava

import "time"

// AthleteInfo es la información del atleta autenticado desde GET /athlete.
type AthleteInfo struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Profile   string `json:"profile"`
}

// ActivitySummary es el resumen de una actividad desde GET /athlete/activities.
type ActivitySummary struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	Type               string    `json:"type"`
	StartDate          time.Time `json:"start_date"`
	Distance           float64   `json:"distance"`
	MovingTime         int       `json:"moving_time"`
	ElapsedTime        int       `json:"elapsed_time"`
	TotalElevationGain float64   `json:"total_elevation_gain"`
}

// ActivityDetail es la información completa de una actividad desde GET /activities/:id.
type ActivityDetail struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	Type               string    `json:"type"`
	StartDate          time.Time `json:"start_date"`
	Distance           float64   `json:"distance"`
	MovingTime         int       `json:"moving_time"`
	ElapsedTime        int       `json:"elapsed_time"`
	TotalElevationGain float64   `json:"total_elevation_gain"`
	Description        string    `json:"description"`
}

// StreamFrame es un único stream desde GET /activities/:id/streams.
type StreamFrame struct {
	Type         string        `json:"type"`
	Data         []interface{} `json:"data"`
	SeriesType   string        `json:"series_type"`
	OriginalSize int           `json:"original_size"`
	Resolution   int           `json:"resolution"`
}
