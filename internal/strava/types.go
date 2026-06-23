package strava

// Activity represents a Strava activity.
type Activity struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	StartDate           string   `json:"start_date"`
	StartDateLocal      string   `json:"start_date_local"`
	DistanceMeters      float64  `json:"distance"`
	MovingTimeSeconds   int      `json:"moving_time"`
	ElapsedTimeSeconds  int      `json:"elapsed_time"`
	ElevationGainMeters float64  `json:"total_elevation_gain"`
	AverageCadence      *float64 `json:"average_cadence,omitempty"`
	MaxCadence          *int     `json:"max_cadence,omitempty"`
	AverageHeartrate    *float64 `json:"average_heartrate,omitempty"`
	MaxHeartrate        *int     `json:"max_heartrate,omitempty"`
	AverageWatts        *float64 `json:"average_watts,omitempty"`
	MaxWatts            *int     `json:"max_watts,omitempty"`
	Kilojoules          *float64 `json:"kilojoules,omitempty"`
	DeviceName          *string  `json:"device_name,omitempty"`
	Athlete             *Athlete `json:"athlete,omitempty"`
}

// Athlete represents minimal athlete info in activity responses.
type Athlete struct {
	ID   int64  `json:"id"`
	Name string `json:"firstname"`
}

// Streams contains time-series data for an activity.
type Streams struct {
	Time      *Stream `json:"time,omitempty"`
	Latlng    *Stream `json:"latlng,omitempty"`
	Distance  *Stream `json:"distance,omitempty"`
	Altitude  *Stream `json:"altitude,omitempty"`
	Heartrate *Stream `json:"heartrate,omitempty"`
	Cadence   *Stream `json:"cadence,omitempty"`
	Watts     *Stream `json:"watts,omitempty"`
	Grade     *Stream `json:"grade_smooth,omitempty"`
	Temp      *Stream `json:"temp,omitempty"`
}

// Stream is a single data stream (array of values).
type Stream struct {
	Data       []interface{} `json:"data"`
	SeriesType string        `json:"series_type"`
	OrigType   string        `json:"original_size"`
}

// Zone represents a heartrate zone.
type Zone struct {
	Score int    `json:"score"`
	Type  string `json:"type"`
	Max   int    `json:"max,omitempty"`
	Min   int    `json:"min,omitempty"`
}

// Subscription represents a webhook subscription response.
type Subscription struct {
	ID       int64  `json:"id"`
	Resource string `json:"resource_state"`
}

// TokenResponse is returned by Strava's token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresAt    int64  `json:"expires_at"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}
