package gpx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"time"

	twgpx "github.com/twpayne/go-gpx"
)

type Parser struct{}

type rawGPX struct {
	Tracks []rawTrack `xml:"trk"`
}

type rawTrack struct {
	Segments []rawSegment `xml:"trkseg"`
}

type rawSegment struct {
	Points []rawPoint `xml:"trkpt"`
}

type rawPoint struct {
	Lat  float64  `xml:"lat,attr"`
	Lon  float64  `xml:"lon,attr"`
	Ele  *float64 `xml:"ele"`
	Time *string  `xml:"time"`
}

func (Parser) Parse(reader io.Reader) (*Track, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("invalid GPX: read: %w", err)
	}
	document, err := twgpx.Read(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid GPX: %w", err)
	}

	var raw rawGPX
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid GPX: %w", err)
	}

	track := &Track{}
	if len(document.Trk) > 0 {
		track.Name = document.Trk[0].Name
	}
	for _, trk := range raw.Tracks {
		for _, segment := range trk.Segments {
			for _, point := range segment.Points {
				converted := Point{Lat: point.Lat, Lon: point.Lon, Ele: point.Ele}
				if point.Time != nil {
					parsed, parseErr := time.Parse(time.RFC3339Nano, *point.Time)
					if parseErr != nil {
						return nil, fmt.Errorf("invalid GPX: trackpoint time: %w", parseErr)
					}
					converted.Time = &parsed
				}
				track.Points = append(track.Points, converted)
			}
		}
	}
	if len(track.Points) == 0 {
		return nil, fmt.Errorf("invalid GPX: no trackpoints")
	}
	return track, nil
}
