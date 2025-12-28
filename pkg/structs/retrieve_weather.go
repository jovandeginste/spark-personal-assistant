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

	today := humantime.Today()
	for d := range 5 {
		e, err := newHourEventFromOpenMeteo(weatherData, location, today.Add(time.Duration(d*24)*time.Hour))
		if err != nil {
			log.Printf("Error: %s", err)
		} else {
			entries = append(entries, *e)
		}
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

func newHourEventFromOpenMeteo(wd *WeatherData, location string, date *humantime.HumanTime) (*Entry, error) {
	allHours := wd.Hourly

	e := &Entry{
		Date:    *date,
		Summary: fmt.Sprintf("Hourly weather for %s in %s", date.Format("Monday"), location),
	}

	type hw map[string]string

	hr := map[string]hw{}

	for hour, t := range allHours.Time {
		lt, err := time.ParseInLocation("2006-01-02T15:04", t, humantime.LocalTimezone)
		if err != nil {
			continue
		}

		d := &humantime.HumanTime{Time: lt}
		if !d.SameDate(date) {
			continue
		}

		if d.Hour() == 0 {
			d = d.Add(60 * time.Second)
		}

		data := hw{
			"temperature": fmt.Sprintf("%.1f %s", allHours.Temperature2M[hour], wd.HourlyUnits.Temperature2M),
			"rain":        fmt.Sprintf("%.0f %s", allHours.Rain[hour], wd.HourlyUnits.Rain),
			"showers":     fmt.Sprintf("%.0f %s", allHours.Showers[hour], wd.HourlyUnits.Showers),
			"snowfall":    fmt.Sprintf("%.0f %s", allHours.Snowfall[hour], wd.HourlyUnits.Snowfall),
			"windspeed":   fmt.Sprintf("%.1f %s", allHours.WindSpeed10M[hour], wd.HourlyUnits.WindSpeed10M),
		}

		hr[d.FormatDate()] = data
	}

	j, err := json.MarshalIndent(hr, "", "  ")
	if err != nil {
		return nil, err
	}

	e.SetMetadata("hourly", string(j))

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
