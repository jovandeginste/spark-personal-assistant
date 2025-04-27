package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
	"github.com/spf13/cobra"
)

var (
	omURL      = "https://api.open-meteo.com/v1/forecast"
	attributes = []string{
		"temperature_2m_min",
		"temperature_2m_max",
		"sunrise",
		"sunset",
		"rain_sum",
		"temperature_2m_mean",
		"snowfall_sum",
		"showers_sum",
		"wind_speed_10m_max",
	}
)

func (c *cli) weatherCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "weather2entry source location",
		Short:   "Convert open-meteo JSON to Spark entries",
		Example: "spark weather2entry weather-brussels Brussels",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			geocoder.SetClient(c.app.Logger(), "Spark")

			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			location := args[1]

			weatherData, err := getWeatherData(location)
			if err != nil {
				return err
			}

			entries := make(data.Entries, len(weatherData.Daily.Time))

			for day := range len(weatherData.Daily.Time) {
				e, err := newEventFromOpenMeteo(weatherData, location, day)
				if err != nil {
					log.Printf("Error: %s", err)
					continue
				}

				entries[day] = *e
			}

			c.app.FetchExistingEntries(entries)

			return c.app.ReplaceSourceEntries(src, entries)
		},
	}

	return cmd
}

func queryFor(location string) (url.Values, error) {
	addr, err := geocoder.SearchLocations(location)
	if err != nil {
		return nil, err
	}

	if len(addr) == 0 {
		return nil, fmt.Errorf("no location found for %q", location)
	}

	q := OpenMeteoParams{
		Latitude:  addr[0].Lat,
		Longitude: addr[0].Lon,
		Daily:     strings.Join(attributes, ","),
		Timezone:  "GMT+1",
		PastDays:  1,
	}

	return query.Values(q)
}

func getWeatherInfo(location string) ([]byte, error) {
	q, err := queryFor(location)
	if err != nil {
		return nil, err
	}

	return getBody(omURL + "?" + q.Encode())
}

func getWeatherData(location string) (*WeatherData, error) {
	var d WeatherData

	w, err := getWeatherInfo(location)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(w, &d); err != nil {
		return nil, err
	}

	if d.Error {
		return nil, fmt.Errorf("could not get forecast: %s", d.Reason)
	}

	return &d, nil
}

func newEventFromOpenMeteo(wd *WeatherData, location string, day int) (*data.Entry, error) {
	allDays := wd.Daily
	eDate := allDays.Time[day]

	parsedDate, err := time.Parse("2006-01-02", eDate)
	if err != nil {
		return nil, err
	}

	e := &data.Entry{
		Date:    data.HumanTime{Time: parsedDate},
		Summary: fmt.Sprintf("Weather for %s in %s", parsedDate.Format("Monday"), location),
	}

	e.SetMetadata("Sunrise", allDays.Sunrise[day])
	e.SetMetadata("Sunset", allDays.Sunset[day])
	e.SetMetadata("Mean temperature", fmt.Sprintf("%.1f %s", allDays.Temperature2MMean[day], wd.DailyUnits.Temperature2MMean))
	e.SetMetadata("Max temperature", fmt.Sprintf("%.1f %s", allDays.Temperature2MMax[day], wd.DailyUnits.Temperature2MMax))
	e.SetMetadata("Min temperature", fmt.Sprintf("%.1f %s", allDays.Temperature2MMin[day], wd.DailyUnits.Temperature2MMin))
	e.SetMetadata("Rain sum", fmt.Sprintf("%.0f %s", allDays.RainSum[day], wd.DailyUnits.RainSum))
	e.SetMetadata("Showers sum", fmt.Sprintf("%.0f %s", allDays.ShowersSum[day], wd.DailyUnits.ShowersSum))
	e.SetMetadata("Snowfall sum", fmt.Sprintf("%.0f %s", allDays.SnowfallSum[day], wd.DailyUnits.SnowfallSum))
	e.SetMetadata("Windspeed max", fmt.Sprintf("%.1f %s", allDays.WindSpeed10MMax[day], wd.DailyUnits.WindSpeed10MMax))
	e.SetMetadata("Latitude", wd.Latitude)
	e.SetMetadata("Longitude", wd.Longitude)

	return e, nil
}

type OpenMeteoParams struct {
	Latitude  string `url:"latitude"`
	Longitude string `url:"longitude"`
	Daily     string `url:"daily"`
	Timezone  string `url:"timezone"`
	PastDays  int    `url:"past_days"`
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
	Reason               string     `json:"reason"`
	Error                bool       `json:"error"`
}

type DailyUnits struct {
	RainSum           string `json:"rain_sum"`
	ShowersSum        string `json:"showers_sum"`
	SnowfallSum       string `json:"snowfall_sum"`
	Sunrise           string `json:"sunrise"`
	Sunset            string `json:"sunset"`
	Temperature2MMax  string `json:"temperature_2m_max"`
	Temperature2MMean string `json:"temperature_2m_mean"`
	Temperature2MMin  string `json:"temperature_2m_min"`
	Time              string `json:"time"`
	WindSpeed10MMax   string `json:"wind_speed_10m_max"`
}

type Daily struct {
	RainSum           []float64 `json:"rain_sum"`
	ShowersSum        []float64 `json:"showers_sum"`
	SnowfallSum       []float64 `json:"snowfall_sum"`
	Sunrise           []string  `json:"sunrise"`
	Sunset            []string  `json:"sunset"`
	Temperature2MMax  []float64 `json:"temperature_2m_max"`
	Temperature2MMean []float64 `json:"temperature_2m_mean"`
	Temperature2MMin  []float64 `json:"temperature_2m_min"`
	Time              []string  `json:"time"`
	WindSpeed10MMax   []float64 `json:"wind_speed_10m_max"`
}
