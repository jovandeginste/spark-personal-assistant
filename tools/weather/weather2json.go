package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
)

func main() {
	if len(os.Args) < 2 {
		log.Println("Usage:", os.Args[0], "<file.json>")
		return
	}

	file := os.Args[1]

	r, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	var d WeatherData

	if err := json.Unmarshal(r, &d); err != nil {
		panic(err)
	}

	results := make([]*data.Entry, len(d.Daily.Time))

	for day := range len(d.Daily.Time) {
		e, err := newEvent(&d, day)
		if err != nil {
			log.Printf("Error: %s", err)
			continue
		}

		results[day] = e
	}

	out, err := json.Marshal(results)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(out))
}

func newEvent(wd *WeatherData, day int) (*data.Entry, error) {
	allDays := wd.Daily
	eDate := allDays.Time[day]

	parsedDate, err := time.Parse("2006-01-02", eDate)
	if err != nil {
		return nil, err
	}

	e := &data.Entry{
		Date:    parsedDate,
		Summary: fmt.Sprintf("Weather for %s", parsedDate.Format("Monday")),
		Metadata: map[string]any{
			"Sunrise":          allDays.Sunrise[day],
			"Sunset":           allDays.Sunset[day],
			"Mean temperature": fmt.Sprintf("%.1f %s", allDays.Temperature2MMean[day], wd.DailyUnits.Temperature2MMean),
			"Max temperature":  fmt.Sprintf("%.1f %s", allDays.Temperature2MMax[day], wd.DailyUnits.Temperature2MMean),
			"Rain sum":         fmt.Sprintf("%.0f %s", allDays.RainSum[day], wd.DailyUnits.RainSum),
		},
	}

	return e, nil
}

type WeatherData struct {
	Latitude             float64    `json:"latitude"`
	Longitude            float64    `json:"longitude"`
	GenerationtimeMs     float64    `json:"generationtime_ms"`
	UtcOffsetSeconds     int        `json:"utc_offset_seconds"`
	Timezone             string     `json:"timezone"`
	TimezoneAbbreviation string     `json:"timezone_abbreviation"`
	Elevation            float64    `json:"elevation"`
	DailyUnits           DailyUnits `json:"daily_units"`
	Daily                Daily      `json:"daily"`
}
type DailyUnits struct {
	Time              string `json:"time"`
	Temperature2MMax  string `json:"temperature_2m_max"`
	Sunrise           string `json:"sunrise"`
	Sunset            string `json:"sunset"`
	RainSum           string `json:"rain_sum"`
	Temperature2MMean string `json:"temperature_2m_mean"`
}
type Daily struct {
	Time              []string  `json:"time"`
	Temperature2MMax  []float64 `json:"temperature_2m_max"`
	Sunrise           []string  `json:"sunrise"`
	Sunset            []string  `json:"sunset"`
	RainSum           []float64 `json:"rain_sum"`
	Temperature2MMean []float64 `json:"temperature_2m_mean"`
}
