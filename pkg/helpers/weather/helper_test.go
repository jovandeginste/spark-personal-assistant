package weather

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a valid WeatherData payload
func createMockWeatherData(days int, pastDays int) *WeatherData {
	wd := &WeatherData{
		Latitude:  50.8500,
		Longitude: 4.3500,
		DailyUnits: DailyUnits{
			RainSum:           "mm",
			ShowersSum:        "mm",
			SnowfallSum:       "cm",
			Sunrise:           "iso8601",
			Sunset:            "iso8601",
			Temperature2MMax:  "°C",
			Temperature2MMean: "°C",
			Temperature2MMin:  "°C",
			Time:              "iso8601",
			WindSpeed10MMax:   "km/h",
		},
		Daily: Daily{
			RainSum:           make([]float64, days),
			ShowersSum:        make([]float64, days),
			SnowfallSum:       make([]float64, days),
			Sunrise:           make([]string, days),
			Sunset:            make([]string, days),
			Temperature2MMax:  make([]float64, days),
			Temperature2MMean: make([]float64, days),
			Temperature2MMin:  make([]float64, days),
			Time:              make([]string, days),
			WindSpeed10MMax:   make([]float64, days),
		},
		Reason: "",
		Error:  false,
	}

	now := time.Now().Truncate(24 * time.Hour)

	for i := 0; i < days; i++ {
		currentDay := now.AddDate(0, 0, i-pastDays)
		dateStr := currentDay.Format("2006-01-02")
		sunriseTime := currentDay.Add(6*time.Hour + 30*time.Minute).Format(time.RFC3339)
		sunsetTime := currentDay.Add(18*time.Hour + 30*time.Minute).Format(time.RFC3339)

		wd.Daily.Time[i] = dateStr
		wd.Daily.Temperature2MMin[i] = float64(5 + i)
		wd.Daily.Temperature2MMean[i] = float64(10 + i)
		wd.Daily.Temperature2MMax[i] = float64(15 + i)
		wd.Daily.RainSum[i] = float64(i * 2)
		wd.Daily.ShowersSum[i] = float64(i)
		wd.Daily.SnowfallSum[i] = float64(0) // Assuming no snow for simplicity
		wd.Daily.Sunrise[i] = sunriseTime
		wd.Daily.Sunset[i] = sunsetTime
		wd.Daily.WindSpeed10MMax[i] = float64(10 + i*5)
	}

	return wd
}

// TestQueryFor tests the queryFor function.
func TestQueryFor(t *testing.T) {
	mockLocation := "Brussels"
	mockLat := "50.8503"
	mockLon := "4.3517"
	mockAddr := []geocoder.Result{{Lat: mockLat, Lon: mockLon}}

	tests := []struct {
		name          string
		location      string
		mockGeoResult []geocoder.Result
		mockGeoError  error
		expectError   bool
		expectedQuery url.Values
	}{
		{
			name:          "Successful geocoding",
			location:      mockLocation,
			mockGeoResult: mockAddr,
			mockGeoError:  nil,
			expectError:   false,
			expectedQuery: url.Values{
				"latitude":  []string{mockLat},
				"longitude": []string{mockLon},
				"daily":     []string{strings.Join(attributes, ",")},
				"timezone":  []string{"GMT+1"},
				"past_days": []string{"1"},
			},
		},
		{
			name:          "Geocoding returns no results",
			location:      mockLocation,
			mockGeoResult: []geocoder.Result{},
			mockGeoError:  nil,
			expectError:   true,
		},
		{
			name:          "Geocoding returns an error",
			location:      mockLocation,
			mockGeoResult: nil,
			mockGeoError:  errors.New("geocoder failed"),
			expectError:   true,
		},
		{
			name:          "Empty location string", // Geocoder might handle this differently
			location:      "",
			mockGeoResult: []geocoder.Result{}, // Simulate geocoder returning empty for empty string
			mockGeoError:  nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Patch geocoder.SearchLocations
			patchGeo := gomonkey.ApplyFunc(geocoder.SearchLocations, func(loc string) ([]geocoder.Result, error) {
				assert.Equal(t, tt.location, loc, "geocoder.SearchLocations called with incorrect location")
				return tt.mockGeoResult, tt.mockGeoError
			})
			defer patchGeo.Reset()

			// Call the function under test
			q, err := queryFor(tt.location)

			if tt.expectError {
				assert.Error(t, err, "Expected an error")
			} else {
				assert.NoError(t, err, "Did not expect an error")
				assert.Equal(t, tt.expectedQuery, q, "Generated query mismatch")
			}
		})
	}
}

// TestGetWeatherInfo tests the getWeatherInfo function.
func TestGetWeatherInfo(t *testing.T) {
	mockLocation := "Paris"
	mockQuery := url.Values{"loc": []string{"Paris"}} // Simplified query for testing generic.GetBody call
	mockBody := []byte(`{"weather": "sunny"}`)
	mockError := errors.New("http error")

	tests := []struct {
		name           string
		location       string
		mockQueryValue url.Values
		mockQueryError error
		mockGetBodyRes []byte
		mockGetBodyErr error
		expectError    bool
		expectedBody   []byte
	}{
		{
			name:           "Successful API call",
			location:       mockLocation,
			mockQueryValue: mockQuery,
			mockQueryError: nil,
			mockGetBodyRes: mockBody,
			mockGetBodyErr: nil,
			expectError:    false,
			expectedBody:   mockBody,
		},
		{
			name:           "queryFor returns error",
			location:       mockLocation,
			mockQueryValue: nil,
			mockQueryError: errors.New("query error"),
			mockGetBodyRes: nil, // Should not be called
			mockGetBodyErr: nil, // Should not be called
			expectError:    true,
			expectedBody:   nil,
		},
		{
			name:           "generic.GetBody returns error",
			location:       mockLocation,
			mockQueryValue: mockQuery,
			mockQueryError: nil,
			mockGetBodyRes: nil,
			mockGetBodyErr: mockError,
			expectError:    true,
			expectedBody:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Patch queryFor
			patchQuery := gomonkey.ApplyFunc(queryFor, func(loc string) (url.Values, error) {
				assert.Equal(t, tt.location, loc, "queryFor called with incorrect location")
				return tt.mockQueryValue, tt.mockQueryError
			})
			defer patchQuery.Reset()

			// Patch generic.GetBody
			patchGetBody := gomonkey.ApplyFunc(generic.GetBody, func(u string) ([]byte, error) {
				// Construct the expected URL from the mock query value
				expectedURL := omURL + "?" + tt.mockQueryValue.Encode()
				assert.Equal(t, expectedURL, u, "generic.GetBody called with incorrect URL")
				return tt.mockGetBodyRes, tt.mockGetBodyErr
			})
			defer patchGetBody.Reset()

			// Call the function under test
			body, err := getWeatherInfo(tt.location)

			if tt.expectError {
				assert.Error(t, err, "Expected an error")
			} else {
				assert.NoError(t, err, "Did not expect an error")
				assert.Equal(t, tt.expectedBody, body, "Received body mismatch")
			}
		})
	}
}

// TestGetWeather tests the GetWeather function.
func TestGetWeather(t *testing.T) {
	mockLocation := "Berlin"
	mockWeatherData := createMockWeatherData(7, 1) // 7 days including 1 past day
	mockWeatherJSON, _ := json.Marshal(mockWeatherData)

	mockApiErrorData := &WeatherData{Error: true, Reason: "API key invalid"}
	mockApiErrorJSON, _ := json.Marshal(mockApiErrorData)

	tests := []struct {
		name                string
		location            string
		mockGetInfoRes      []byte
		mockGetInfoErr      error
		expectError         bool
		expectedWeatherData *WeatherData
		expectedErrSubstr   string // For API errors
	}{
		{
			name:                "Successful weather data fetch",
			location:            mockLocation,
			mockGetInfoRes:      mockWeatherJSON,
			mockGetInfoErr:      nil,
			expectError:         false,
			expectedWeatherData: mockWeatherData,
		},
		{
			name:                "getWeatherInfo returns error",
			location:            mockLocation,
			mockGetInfoRes:      nil,
			mockGetInfoErr:      errors.New("info fetch failed"),
			expectError:         true,
			expectedWeatherData: nil,
		},
		{
			name:                "Invalid JSON response",
			location:            mockLocation,
			mockGetInfoRes:      []byte(`invalid json`),
			mockGetInfoErr:      nil,
			expectError:         true,
			expectedWeatherData: nil,
		},
		{
			name:                "Open-Meteo API returns error",
			location:            mockLocation,
			mockGetInfoRes:      mockApiErrorJSON,
			mockGetInfoErr:      nil,
			expectError:         true,
			expectedWeatherData: nil,
			expectedErrSubstr:   "could not get forecast: API key invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Patch getWeatherInfo
			patchGetInfo := gomonkey.ApplyFunc(getWeatherInfo, func(loc string) ([]byte, error) {
				assert.Equal(t, tt.location, loc, "getWeatherInfo called with incorrect location")
				return tt.mockGetInfoRes, tt.mockGetInfoErr
			})
			defer patchGetInfo.Reset()

			// Call the function under test
			wd, err := getWeatherData(tt.location)

			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				if tt.expectedErrSubstr != "" {
					assert.Contains(t, err.Error(), tt.expectedErrSubstr, "Error message mismatch")
				}
				assert.Nil(t, wd, "Expected nil WeatherData on error")
			} else {
				assert.NoError(t, err, "Did not expect an error")
				require.NotNil(t, wd, "Expected non-nil WeatherData on success")
				// DeepEqual is needed to compare struct fields
				assert.True(t, reflect.DeepEqual(tt.expectedWeatherData, wd), "WeatherData mismatch")
			}
		})
	}
}

// TestNewEventFromOpenMeteo tests the newEventFromOpenMeteo function.
func TestNewEventFromOpenMeteo(t *testing.T) {
	mockLocation := "New York"
	// Create a mock WeatherData for 3 days starting from a fixed date
	fixedDate := time.Date(2024, time.April, 10, 0, 0, 0, 0, time.UTC) // Use UTC for simplicity, as time.Parse("2006-01-02") returns UTC
	wd := &WeatherData{
		Latitude:  40.7128,
		Longitude: -74.0060,
		DailyUnits: DailyUnits{
			RainSum:           "mm",
			ShowersSum:        "mm",
			SnowfallSum:       "cm",
			Sunrise:           "iso8601",
			Sunset:            "iso8601",
			Temperature2MMax:  "°C",
			Temperature2MMean: "°C",
			Temperature2MMin:  "°C",
			Time:              "iso8601",
			WindSpeed10MMax:   "km/h",
		},
		Daily: Daily{
			Time:              []string{fixedDate.Format("2006-01-02"), fixedDate.AddDate(0, 0, 1).Format("2006-01-02"), fixedDate.AddDate(0, 0, 2).Format("2006-01-02")},
			Temperature2MMin:  []float64{2.5, 3.0, 4.1},
			Temperature2MMean: []float64{8.0, 9.5, 10.0},
			Temperature2MMax:  []float64{15.1, 16.0, 17.5},
			RainSum:           []float64{0.0, 5.2, 10.0},
			ShowersSum:        []float64{0.5, 1.0, 0.0},
			SnowfallSum:       []float64{0.0, 0.0, 1.5},
			Sunrise:           []string{fixedDate.Add(6*time.Hour + 0*time.Minute).Format(time.RFC3339), fixedDate.AddDate(0, 0, 1).Add(5*time.Hour + 59*time.Minute).Format(time.RFC3339), fixedDate.AddDate(0, 0, 2).Add(5*time.Hour + 58*time.Minute).Format(time.RFC3339)},
			Sunset:            []string{fixedDate.Add(19*time.Hour + 0*time.Minute).Format(time.RFC3339), fixedDate.AddDate(0, 0, 1).Add(19*time.Hour + 1*time.Minute).Format(time.RFC3339), fixedDate.AddDate(0, 0, 2).Add(19*time.Hour + 2*time.Minute).Format(time.RFC3339)},
			WindSpeed10MMax:   []float64{12.3, 15.0, 18.8},
		},
		Reason: "",
		Error:  false,
	}

	tests := []struct {
		name          string
		weatherData   *WeatherData
		location      string
		dayIndex      int
		expectError   bool
		expectedEntry *data.Entry // Expected entry details (Date.Time, Summary, Metadata)
	}{
		{
			name:        "Create entry for Day 0",
			weatherData: wd,
			location:    mockLocation,
			dayIndex:    0,
			expectError: false,
			expectedEntry: &data.Entry{
				Date:    data.HumanTime{Time: fixedDate}, // time.Parse("2006-01-02") gives UTC
				Summary: fmt.Sprintf("Weather for %s in %s", fixedDate.Format("Monday"), mockLocation),
				Metadata: map[string]any{
					"Latitude":         40.7128,
					"Longitude":        -74.006,
					"Sunrise":          fixedDate.Add(6 * time.Hour).Format(time.RFC3339),
					"Sunset":           fixedDate.Add(19 * time.Hour).Format(time.RFC3339),
					"Mean temperature": "8.0 °C",
					"Max temperature":  "15.1 °C",
					"Min temperature":  "2.5 °C",
					"Rain sum":         "0 mm",
					"Showers sum":      "0 mm",
					"Snowfall sum":     "0 cm",
					"Windspeed max":    "12.3 km/h",
				},
			},
		},
		{
			name:        "Create entry for Day 1",
			weatherData: wd,
			location:    mockLocation,
			dayIndex:    1,
			expectError: false,
			expectedEntry: &data.Entry{
				Date:    data.HumanTime{Time: fixedDate.AddDate(0, 0, 1)},
				Summary: fmt.Sprintf("Weather for %s in %s", fixedDate.AddDate(0, 0, 1).Format("Monday"), mockLocation),
				Metadata: map[string]any{
					"Latitude":         40.7128,
					"Longitude":        -74.006,
					"Sunrise":          fixedDate.AddDate(0, 0, 1).Add(5*time.Hour + 59*time.Minute).Format(time.RFC3339),
					"Sunset":           fixedDate.AddDate(0, 0, 1).Add(19*time.Hour + 1*time.Minute).Format(time.RFC3339),
					"Mean temperature": "9.5 °C",
					"Max temperature":  "16.0 °C",
					"Min temperature":  "3.0 °C",
					"Rain sum":         "5 mm",
					"Showers sum":      "1 mm",
					"Snowfall sum":     "0 cm",
					"Windspeed max":    "15.0 km/h",
				},
			},
		},
		{
			name:        "Create entry for Day 2 (includes snowfall)",
			weatherData: wd,
			location:    mockLocation,
			dayIndex:    2,
			expectError: false,
			expectedEntry: &data.Entry{
				Date:    data.HumanTime{Time: fixedDate.AddDate(0, 0, 2)},
				Summary: fmt.Sprintf("Weather for %s in %s", fixedDate.AddDate(0, 0, 2).Format("Monday"), mockLocation),
				Metadata: map[string]any{
					"Latitude":         40.7128,
					"Longitude":        -74.006,
					"Sunrise":          fixedDate.AddDate(0, 0, 2).Add(5*time.Hour + 58*time.Minute).Format(time.RFC3339),
					"Sunset":           fixedDate.AddDate(0, 0, 2).Add(19*time.Hour + 2*time.Minute).Format(time.RFC3339),
					"Mean temperature": "10.0 °C",
					"Max temperature":  "17.5 °C",
					"Min temperature":  "4.1 °C", // Tests single decimal point
					"Rain sum":         "10 mm",  // Tests integer formatting
					"Showers sum":      "0 mm",
					"Snowfall sum":     "2 cm", // Tests float formatting with .0
					"Windspeed max":    "18.8 km/h",
				},
			},
		},
		{
			name:        "Invalid date string in WeatherData",
			weatherData: &WeatherData{Daily: Daily{Time: []string{"invalid-date"}}},
			location:    mockLocation,
			dayIndex:    0,
			expectError: true,
		},
		{
			name:        "Index out of bounds",
			weatherData: wd,
			location:    mockLocation,
			dayIndex:    len(wd.Daily.Time) + 1, // Index past the end
			expectError: true,                   // Indexing slice out of bounds will cause panic/runtime error before function logic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a closure to wrap the potentially panicking call
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectError || !strings.Contains(fmt.Sprintf("%v", r), "index out of range") {
						t.Errorf("Test panicked unexpectedly: %v", r)
					}
				}
			}()

			// Call the function under test
			entry, err := newEventFromOpenMeteo(tt.weatherData, tt.location, tt.dayIndex)

			if tt.expectError && err == nil {
				// If we expected an error but got none, check if the error was an out-of-bounds panic
				if tt.dayIndex < 0 || tt.dayIndex >= len(tt.weatherData.Daily.Time) {
					// This case is handled by the defer/recover block above
				} else {
					t.Errorf("Expected an error but got none")
				}
			} else if !tt.expectError && err != nil {
				t.Errorf("Did not expect an error but got: %v", err)
			} else if !tt.expectError && err == nil {
				// Success case: Compare the generated entry
				require.NotNil(t, entry, "Expected non-nil entry on success")

				// Compare relevant fields
				assert.True(t, tt.expectedEntry.Date.Time.Equal(entry.Date.Time), "Date mismatch")
				assert.Equal(t, tt.expectedEntry.Summary, entry.Summary, "Summary mismatch")

				// Metadata comparison requires deep equal
				assert.Equal(t, tt.expectedEntry.Metadata, entry.Metadata, "Metadata mismatch")

				// Check default/generated fields are not set by this function
				assert.Equal(t, uint64(0), entry.ID)
				assert.Equal(t, "", entry.RemoteID)
				assert.Equal(t, uint64(0), entry.SourceID)
				// DateString is populated AfterFind, not here
				assert.Nil(t, entry.Source)
			}
		})
	}
}

// TestGetWeatherData tests the top-level GetWeatherData function.
func TestGetWeatherData(t *testing.T) {
	mockLocation := "London"
	mockDays := 5 // Number of days returned by mock API
	mockWeatherData := createMockWeatherData(mockDays, 1)

	tests := []struct {
		name               string
		location           string
		mockGetWeatherRes  *WeatherData
		mockGetWeatherErr  error
		expectError        bool
		expectedEntryCount int // Expected number of entries created
	}{
		{
			name:               "Successful weather data fetch and entry creation",
			location:           mockLocation,
			mockGetWeatherRes:  mockWeatherData,
			mockGetWeatherErr:  nil,
			expectError:        false,
			expectedEntryCount: mockDays, // Should create one entry per day returned
		},
		{
			name:               "GetWeather returns error",
			location:           mockLocation,
			mockGetWeatherRes:  nil,
			mockGetWeatherErr:  errors.New("weather fetch error"),
			expectError:        true,
			expectedEntryCount: 0,
		},
		{
			name:               "GetWeather returns WeatherData with empty Daily.Time",
			location:           mockLocation,
			mockGetWeatherRes:  &WeatherData{Daily: Daily{Time: []string{}}}, // Empty time slice
			mockGetWeatherErr:  nil,
			expectError:        false,
			expectedEntryCount: 0, // Loop range len(Time) will be 0
		},
		{
			name:               "GetWeather returns WeatherData with invalid date string (simulated)",
			location:           mockLocation,
			mockGetWeatherRes:  &WeatherData{Daily: Daily{Time: []string{"invalid-date"}}}, // newEventFromOpenMeteo will return error
			mockGetWeatherErr:  nil,
			expectError:        false, // newEventFromOpenMeteo errors are logged, not returned by GetWeatherData
			expectedEntryCount: 1,     // One attempt will be made, but the entry might be zero-valued or skipped depending on newEventFromOpenMeteo's internal error handling (it logs and returns err). GetWeatherData appends it regardless.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Patch GetWeather
			patchGetWeather := gomonkey.ApplyFunc(getWeatherData, func(loc string) (*WeatherData, error) {
				assert.Equal(t, tt.location, loc, "GetWeather called with incorrect location")
				return tt.mockGetWeatherRes, tt.mockGetWeatherErr
			})
			defer patchGetWeather.Reset()

			// Although newEventFromOpenMeteo is tested separately, its errors are logged
			// and processing continues in GetWeatherData. We can patch it here
			// to control its behavior specifically for this test if needed,
			// but for basic success/failure, patching GetWeather is sufficient.
			// Let's add a patch for newEventFromOpenMeteo to simulate the invalid date case
			// and ensure GetWeatherData handles it (logs, doesn't return error).
			if tt.name == "GetWeather returns WeatherData with invalid date string (simulated)" {
				patchNewEvent := gomonkey.ApplyFunc(newEventFromOpenMeteo, func(wd *WeatherData, location string, day int) (*data.Entry, error) {
					// Simulate error only for this specific test case
					return &data.Entry{}, errors.New("simulated newEventFromOpenMeteo error")
				})
				defer patchNewEvent.Reset()
			}

			// Call the function under test
			entries, err := GetWeatherData(tt.location)

			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				assert.Nil(t, entries, "Expected nil entries on error")
			} else {
				assert.NoError(t, err, "Did not expect an error")
				assert.Len(t, entries, tt.expectedEntryCount, "Entry count mismatch")

				// In the success case, we could optionally verify the content of the entries
				// by calling newEventFromOpenMeteo directly with the mocked data and comparing
				// against the entries slice, but this adds complexity and newEventFromOpenMeteo
				// is already tested. Checking the count is a good enough integration test here.
			}
		})
	}
}

func init() {
	// Initialize a logger for tests if needed, though log.Printf is less testable
	// We don't need to mock log.Printf for these tests, just be aware it happens.
}
