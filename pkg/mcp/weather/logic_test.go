package weather

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestQueryFor(t *testing.T) {
	// Backup and restore the original searchLocationsFunc
	originalSearchLocations := searchLocationsFunc
	defer func() { searchLocationsFunc = originalSearchLocations }()

	tests := []struct {
		name          string
		location      string
		mockResult    []location
		mockError     error
		expectedError string
	}{
		{
			name:     "Valid Location",
			location: "Amsterdam",
			mockResult: []location{
				{Lat: "52.3676", Lon: "4.9041"},
			},
			mockError:     nil,
			expectedError: "",
		},
		{
			name:          "Geocoding Error",
			location:      "ErrorCity",
			mockResult:    nil,
			mockError:     errors.New("geocoding failed"),
			expectedError: "geocoding failed",
		},
		{
			name:          "No Results",
			location:      "Nowhere",
			mockResult:    []location{},
			mockError:     nil,
			expectedError: "no location found for \"Nowhere\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchLocationsFunc = func(ctx context.Context, query string) ([]location, error) {
				return tt.mockResult, tt.mockError
			}

			params, err := queryFor(context.Background(), tt.location)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				assert.Nil(t, params)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, params)
				assert.Equal(t, tt.mockResult[0].Lat, params.Latitude)
				assert.Equal(t, tt.mockResult[0].Lon, params.Longitude)
			}
		})
	}
}

func TestSearchLocations(t *testing.T) {
	originalGetBody := generic.GetBodyWithHeaders
	defer func() { generic.GetBodyWithHeaders = originalGetBody }()

	var (
		requested string
		userAgent string
	)

	generic.GetBodyWithHeaders = func(url string, headers map[string]string) ([]byte, error) {
		requested = url
		userAgent = headers["User-Agent"]

		return []byte(`[{"lat":"52.3676","lon":"4.9041"}]`), nil
	}

	locations, err := searchLocations(context.Background(), "Amsterdam")
	require.NoError(t, err)
	require.Len(t, locations, 1)
	assert.Equal(t, "52.3676", locations[0].Lat)
	assert.Contains(t, requested, "q=Amsterdam")
	assert.Contains(t, userAgent, "Spark")
}

func TestSearchLocationsCancellation(t *testing.T) {
	originalLimiter := geocodeLimiter
	geocodeLimiter = rate.NewLimiter(rate.Every(time.Hour), 1)
	defer func() { geocodeLimiter = originalLimiter }()

	require.True(t, geocodeLimiter.Allow())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := searchLocations(ctx, "Amsterdam")
	require.ErrorIs(t, err, context.Canceled)
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		expectedError string
	}{
		{
			name:          "Valid Config",
			config:        Config{APIURL: "https://api.example.com"},
			expectedError: "",
		},
		{
			name:          "Missing API URL",
			config:        Config{APIURL: ""},
			expectedError: "weather API URL is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Module{}
			m.SetConfig(tt.config)

			err := m.Enabled()
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQueryForTimezone(t *testing.T) {
	originalSearchLocations := searchLocationsFunc
	defer func() { searchLocationsFunc = originalSearchLocations }()

	searchLocationsFunc = func(ctx context.Context, query string) ([]location, error) {
		return []location{{Lat: "40.7128", Lon: "-74.0060"}}, nil
	}

	params, err := queryFor(context.Background(), "New York")
	assert.NoError(t, err)
	assert.Equal(t, "auto", params.Timezone)
}
