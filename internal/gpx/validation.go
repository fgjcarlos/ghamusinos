package gpx

import "fmt"

const (
	MinFileSize = int64(1024)
	MaxFileSize = int64(10 * 1024 * 1024)
)

type Validator struct{}

func (Validator) Validate(size int64, track *Track) error {
	if size > MaxFileSize {
		return fmt.Errorf("file exceeds 10 MB limit")
	}
	if size < MinFileSize {
		return fmt.Errorf("file is smaller than 1 KB minimum")
	}
	if track == nil {
		return fmt.Errorf("track is required")
	}
	if len(track.Points) < 2 {
		return fmt.Errorf("track must contain at least 2 trackpoints")
	}
	return nil
}
