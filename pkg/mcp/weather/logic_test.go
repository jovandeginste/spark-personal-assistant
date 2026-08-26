package weather

import (
	"errors"
	"testing"

	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
	"github.com/stretchr/testify/assert"
)

func TestQueryFor(t *testing.T) {
	// Backup and restore the original searchLocationsFunc
	originalSearchLocations := searchLocationsFunc
	defer func() { searchLocationsFunc = originalSearchLocations }()

	tests := []struct {
		name          string
		location      string
		mockResult    []geocoder.Result
		mockError     error
		expectedError string
	}{
		{
			name:     "Valid Location",
			location: "Amsterdam",
			mockResult: []geocoder.Result{
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
			mockResult:    []geocoder.Result{},
			mockError:     nil,
			expectedError: "no location found for \"Nowhere\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchLocationsFunc = func(query string) ([]geocoder.Result, error) {
				return tt.mockResult, tt.mockError
			}

			params, err := queryFor(tt.location)

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

	searchLocationsFunc = func(query string) ([]geocoder.Result, error) {
		return []geocoder.Result{{Lat: "40.7128", Lon: "-74.0060"}}, nil
	}

	params, err := queryFor("New York")
	assert.NoError(t, err)
	assert.Equal(t, "auto", params.Timezone)
}
