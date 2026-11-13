package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"golang.org/x/time/rate"
)

const nominatimURL = "https://nominatim.openstreetmap.org/search"

// Nominatim asks clients to make at most one request per second.
var geocodeLimiter = rate.NewLimiter(rate.Every(time.Second), 1)

type location struct {
	Lat string `json:"lat"`
	Lon string `json:"lon"`
}

// For testing.
var searchLocationsFunc = searchLocations

func searchLocations(ctx context.Context, query string) ([]location, error) {
	if err := geocodeLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("format", "json")
	params.Set("limit", "1")
	params.Set("q", query)

	data, err := generic.GetBodyWithHeaders(nominatimURL+"?"+params.Encode(), map[string]string{
		"User-Agent": "Spark personal assistant (https://github.com/jovandeginste/spark-personal-assistant)",
	})
	if err != nil {
		return nil, fmt.Errorf("geocoding failed: %w", err)
	}

	var locations []location
	if err := json.Unmarshal(data, &locations); err != nil {
		return nil, fmt.Errorf("failed to parse geocoder response: %w", err)
	}

	return locations, nil
}
