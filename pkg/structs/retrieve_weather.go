package structs

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/jovandeginste/spark-personal-assistant/pkg/humantime"
	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
)

var (
	omURL           = "https://api.open-meteo.com/v1/forecast"
	attributesDaily = []string{
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
	attributesHourly = []string{
		"temperature_2m",
		"rain",
		"snowfall",
		"showers",
		"wind_speed_10m",
		"cloud_cover",
		"visibility",
	}
)

func GetWeatherData(location string) (Entries, error) {
	weatherData, err := getWeatherData(location)
	if err != nil {
		return nil, err
	}

	var entries Entries

	for hour := range len(weatherData.Hourly.Time) {
		e, err := newHourEventFromOpenMeteo(weatherData, location, hour)
		if err != nil {
			log.Printf("Error: %s", err)
			continue
		}
		if e == nil {
			continue
		}

		entries = append(entries, *e)
	}

	for day := range len(weatherData.Daily.Time) {
		e, err := newDayEventFromOpenMeteo(weatherData, location, day)
		if err != nil {
			log.Printf("Error: %s", err)
			continue
		}
		if e == nil {
			continue
		}

		entries = append(entries, *e)
	}

	return entries, nil
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
		Daily:     strings.Join(attributesDaily, ","),
		Hourly:    strings.Join(attributesHourly, ","),
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

	return generic.GetBody(omURL + "?" + q.Encode())
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

func newHourEventFromOpenMeteo(wd *WeatherData, location string, hour int) (*Entry, error) {
	allHours := wd.Hourly
	eDate := allHours.Time[hour]

	parsedDate, err := time.Parse("2006-01-02T15:04", eDate)
	if err != nil {
		return nil, err
	}

	parsedDate = parsedDate.In(humantime.LocalTimezone)

	if parsedDate.Before(time.Now().Add(-6 * time.Hour)) {
		return nil, nil
	}

	if parsedDate.After(time.Now().Add(30 * time.Hour)) {
		return nil, nil
	}

	e := &Entry{
		Date:    humantime.HumanTime{Time: parsedDate},
		DateEnd: humantime.HumanTime{Time: parsedDate.Add(1 * time.Hour)},
		Summary: fmt.Sprintf("Weather for %s at %s in %s", parsedDate.Format("Monday"), parsedDate.Format("15:04"), location),
	}

	e.SetMetadata("Temperature", fmt.Sprintf("%.1f %s", allHours.Temperature2M[hour], wd.HourlyUnits.Temperature2M))
	e.SetMetadata("Rain", fmt.Sprintf("%.0f %s", allHours.Rain[hour], wd.HourlyUnits.Rain))
	e.SetMetadata("Showers", fmt.Sprintf("%.0f %s", allHours.Showers[hour], wd.HourlyUnits.Showers))
	e.SetMetadata("Snowfall", fmt.Sprintf("%.0f %s", allHours.Snowfall[hour], wd.HourlyUnits.Snowfall))
	e.SetMetadata("Windspeed", fmt.Sprintf("%.1f %s", allHours.WindSpeed10M[hour], wd.HourlyUnits.WindSpeed10M))
	e.SetMetadata("Latitude", wd.Latitude)
	e.SetMetadata("Longitude", wd.Longitude)

	return e, nil
}

func newDayEventFromOpenMeteo(wd *WeatherData, location string, day int) (*Entry, error) {
	allDays := wd.Daily
	eDate := allDays.Time[day]

	parsedDate, err := time.Parse("2006-01-02", eDate)
	if err != nil {
		return nil, err
	}

	e := &Entry{
		Date:    humantime.HumanTime{Time: parsedDate},
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
	Hourly    string `url:"hourly"`
	Timezone  string `url:"timezone"`
	PastDays  int    `url:"past_days"`
}

type WeatherData struct {
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	GenerationtimeMs     float64 `json:"generationtime_ms"`
	UtcOffsetSeconds     int     `json:"utc_offset_seconds"`
	Timezone             string  `json:"timezone"`
	TimezoneAbbreviation string  `json:"timezone_abbreviation"`
	Elevation            float64 `json:"elevation"`
	DailyUnits           Units   `json:"daily_units"`
	HourlyUnits          Units   `json:"hourly_units"`
	Daily                Result  `json:"daily"`
	Hourly               Result  `json:"hourly"`
	Reason               string  `json:"reason"`
	Error                bool    `json:"error"`
}

type Units struct {
	CloudCover         string `json:"cloud_cover"`
	Rain               string `json:"rain"`
	RainSum            string `json:"rain_sum"`
	RelativeHumidity2M string `json:"relative_humidity_2m"`
	Showers            string `json:"showers"`
	ShowersSum         string `json:"showers_sum"`
	Snowfall           string `json:"snowfall"`
	SnowfallSum        string `json:"snowfall_sum"`
	Sunrise            string `json:"sunrise"`
	Sunset             string `json:"sunset"`
	Temperature2MMax   string `json:"temperature_2m_max"`
	Temperature2MMean  string `json:"temperature_2m_mean"`
	Temperature2MMin   string `json:"temperature_2m_min"`
	Temperature2M      string `json:"temperature_2m"`
	Time               string `json:"time"`
	Visibility         string `json:"visibility"`
	WindSpeed10MMax    string `json:"wind_speed_10m_max"`
	WindSpeed10M       string `json:"wind_speed_10m"`
}

type Result struct {
	CloudCover         []float64 `json:"cloud_cover"`
	Rain               []float64 `json:"rain"`
	RainSum            []float64 `json:"rain_sum"`
	RelativeHumidity2M []float64 `json:"relative_humidity_2m"`
	Showers            []float64 `json:"showers"`
	ShowersSum         []float64 `json:"showers_sum"`
	Snowfall           []float64 `json:"snowfall"`
	SnowfallSum        []float64 `json:"snowfall_sum"`
	Sunrise            []string  `json:"sunrise"`
	Sunset             []string  `json:"sunset"`
	Temperature2M      []float64 `json:"temperature_2m"`
	Temperature2MMax   []float64 `json:"temperature_2m_max"`
	Temperature2MMean  []float64 `json:"temperature_2m_mean"`
	Temperature2MMin   []float64 `json:"temperature_2m_min"`
	Time               []string  `json:"time"`
	Visibility         []float64 `json:"visibility"`
	WindSpeed10M       []float64 `json:"wind_speed_10m"`
	WindSpeed10MMax    []float64 `json:"wind_speed_10m_max"`
}
