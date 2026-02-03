package weather

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestRegister(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0"}, &mcp.ServerOptions{})
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	config := Config{
		APIURL: "http://example.com",
	}

	err := Register(server, config, logger)
	assert.NoError(t, err)
}

func TestGetWeatherInfo(t *testing.T) {
	// Mock geocoder
	originalSearchLocations := searchLocationsFunc
	defer func() { searchLocationsFunc = originalSearchLocations }()

	searchLocationsFunc = func(query string) ([]geocoder.Result, error) {
		return []geocoder.Result{
			{
				Lat: "51.5",
				Lon: "4.5",
			},
		}, nil
	}

	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.String(), "latitude=51.5")
		assert.Contains(t, r.URL.String(), "longitude=4.5")
		assert.Contains(t, r.URL.String(), "start_date=2024-01-01")
		assert.Contains(t, r.URL.String(), "end_date=2024-01-02")

		w.Write([]byte(`{"test": "response"}`))
	}))
	defer ts.Close()

	result, err := getWeatherInfo(ts.URL, "Breda", "2024-01-01", "2024-01-02")
	assert.NoError(t, err)
	assert.Equal(t, []byte(`{"test": "response"}`), result)
}

func TestQueryFor_NoLocation(t *testing.T) {
	// Mock geocoder
	originalSearchLocations := searchLocationsFunc
	defer func() { searchLocationsFunc = originalSearchLocations }()

	searchLocationsFunc = func(query string) ([]geocoder.Result, error) {
		return []geocoder.Result{}, nil
	}

	_, err := queryFor("Unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no location found")
}
