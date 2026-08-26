package weather

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// ForecastResponse represents the raw Open-Meteo API response format.
// ForecastResponse represents the raw Open-Meteo API response format.
type ForecastResponse struct {
	Latitude         float64           `json:"latitude"`
	Longitude        float64           `json:"longitude"`
	GenerationTimeMS float64           `json:"generationtime_ms"`
	UTCOffsetSeconds int64             `json:"utc_offset_seconds"`
	Timezone         string            `json:"timezone"`
	TimezoneAbbr     string            `json:"timezone_abbreviation"`
	Elevation        float64           `json:"elevation"`
	HourlyUnits      map[string]string `json:"hourly_units"`
	Hourly           map[string][]any  `json:"hourly"`
	DailyUnits       map[string]string `json:"daily_units"`
	Daily            map[string][]any  `json:"daily"`
}

// ConvertToCleanMap merges daily and hourly forecast results into a clean
// map[string]map[string]string where the first key is the local date or date+hour,
// the second key is the measurement name, and the value is formatted with its unit.
func ConvertToCleanMap(data []byte) (map[string]map[string]string, error) {
	var resp ForecastResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal forecast response: %w", err)
	}

	result := make(map[string]map[string]string)

	// Process daily forecast if present
	if len(resp.Daily) > 0 {
		times, ok := resp.Daily["time"]
		if ok {
			for i, tVal := range times {
				dateKey, err := parseTimeString(tVal)
				if err != nil {
					continue
				}

				if _, exists := result[dateKey]; !exists {
					result[dateKey] = make(map[string]string)
				}

				for measure, values := range resp.Daily {
					if measure == "time" || i >= len(values) {
						continue
					}
					unit := resp.DailyUnits[measure]
					formattedVal := formatValue(values[i], unit)
					result[dateKey][measure] = formattedVal
				}
			}
		}
	}

	// Process hourly forecast if present
	if len(resp.Hourly) > 0 {
		times, ok := resp.Hourly["time"]
		if ok {
			for i, tVal := range times {
				dateTimeKey, err := parseTimeString(tVal)
				if err != nil {
					continue
				}

				if _, exists := result[dateTimeKey]; !exists {
					result[dateTimeKey] = make(map[string]string)
				}

				for measure, values := range resp.Hourly {
					if measure == "time" || i >= len(values) {
						continue
					}
					unit := resp.HourlyUnits[measure]
					formattedVal := formatValue(values[i], unit)
					result[dateTimeKey][measure] = formattedVal
				}
			}
		}
	}

	return result, nil
}

func parseTimeString(val any) (string, error) {
	switch v := val.(type) {
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("unknown time string type: %T", val)
	}
}

func formatValue(val any, unit string) string {
	var strVal string
	switch v := val.(type) {
	case float64:
		// Format floats neatly, checking if it's an integer or has decimals
		if v == float64(int64(v)) {
			strVal = strconv.FormatInt(int64(v), 10)
		} else {
			strVal = fmt.Sprintf("%.1f", v)
		}
	case int:
		strVal = strconv.Itoa(v)
	case int64:
		strVal = strconv.FormatInt(v, 10)
	case string:
		strVal = v
	case nil:
		strVal = ""
	default:
		strVal = fmt.Sprintf("%v", v)
	}

	if unit == "" {
		return strVal
	}
	return fmt.Sprintf("%s %s", strVal, unit)
}
