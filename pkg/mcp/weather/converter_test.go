package weather

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertToCleanMap(t *testing.T) {
	jsonData := []byte(`{
  "latitude": 50.944,
  "longitude": 4.698,
  "generationtime_ms": 0.15151500701904297,
  "utc_offset_seconds": 7200,
  "timezone": "Europe/Brussels",
  "timezone_abbreviation": "GMT+2",
  "elevation": 13.0,
  "hourly_units": {
    "time": "iso8601",
    "temperature_2m": "°C",
    "rain": "mm",
    "wind_speed_10m": "km/h"
  },
  "hourly": {
    "time": [
      "2026-08-26T00:00",
      "2026-08-26T01:00"
    ],
    "temperature_2m": [
      16.6,
      15.9
    ],
    "rain": [
      0.00,
      0.5
    ],
    "wind_speed_10m": [
      9.4,
      8.0
    ]
  },
  "daily_units": {
    "time": "iso8601",
    "temperature_2m_max": "°C"
  },
  "daily": {
    "time": [ "2026-08-26" ],
    "temperature_2m_max": [ 26.0 ]
  }
}`)

	res, err := ConvertToCleanMap(jsonData)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	assert.Contains(t, res, "2026-08-26")
	assert.Contains(t, res, "2026-08-26T00:00")

	dailyMap := res["2026-08-26"]
	assert.Equal(t, "26 °C", dailyMap["temperature_2m_max"])

	hourlyMap := res["2026-08-26T00:00"]
	assert.Equal(t, "16.6 °C", hourlyMap["temperature_2m"])
	assert.Equal(t, "0 mm", hourlyMap["rain"])
	assert.Equal(t, "9.4 km/h", hourlyMap["wind_speed_10m"])
}
