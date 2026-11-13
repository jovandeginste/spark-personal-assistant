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

func searchLocations(query string) ([]location, error) {
	if err := geocodeLimiter.Wait(context.Background()); err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("format", "json")
	params.Set("limit", "1")
	params.Set("q", query)

	data, err := generic.GetBody(nominatimURL + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("geocoding failed: %w", err)
	}

	var locations []location
	if err := json.Unmarshal(data, &locations); err != nil {
		return nil, fmt.Errorf("failed to parse geocoder response: %w", err)
	}

	return locations, nil
}
