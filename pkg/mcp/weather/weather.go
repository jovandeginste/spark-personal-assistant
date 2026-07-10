package weather

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/go-querystring/query"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	defaultAPIURL   = "https://api.open-meteo.com/v1/forecast"
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

	// For testing
	searchLocationsFunc = geocoder.SearchLocations
)

type Config struct {
	APIURL string
}

type weatherParams struct {
	Location string `json:"location" jsonschema:"The location to get weather report for"`
	sparkmcp.DateRangeParams
}

type Module struct {
	sparkmcp.BaseModule
}

func New(config Config, logger *slog.Logger) *Module {
	m := &Module{}
	m.SetConfig(config)
	m.SetLogger(logger.With("module", "weather"))
	return m
}

func (m *Module) Register(server *mcp.Server) error {
	config := m.Config().(Config)
	logger := m.Logger()

	logger.Info("Registering MCP package")

	if config.APIURL == "" {
		config.APIURL = defaultAPIURL
	}

	tool := &mcp.Tool{
		Name:        "weather",
		Description: "Get weather report",
	}

	handler := func(ctx context.Context, request *mcp.CallToolRequest, params weatherParams) (*mcp.CallToolResult, any, error) {
		reqLogger := logger.With("handler", "weather")
		startDate, endDate := params.GetDateRange()
		reqLogger.Debug("Fetching weather info", "location", params.Location, "start_date", startDate, "end_date", endDate)
		result, err := m.getWeatherInfo(config.APIURL, params.Location, startDate, endDate)
		if err != nil {
			reqLogger.Error("Failed to get weather info", "location", params.Location, "error", err)
			return nil, nil, err
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Weather report for %s from %s to %s: %s", params.Location, startDate, endDate, result),
				},
			},
		}, nil, nil
	}

	mcp.AddTool(server, tool, handler)

	return nil
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.APIURL == "" {
		return errors.New("weather API URL is not configured")
	}
	return nil
}

func (m *Module) getWeatherInfo(apiURL, location, startDate, endDate string) ([]byte, error) {
	q, err := queryFor(location)
	if err != nil {
		return nil, err
	}

	q.StartDate = startDate
	q.EndDate = endDate

	p, err := query.Values(q)
	if err != nil {
		return nil, err
	}

	return generic.GetBody(apiURL + "?" + p.Encode())
}

func queryFor(location string) (*openMeteoParams, error) {
	addr, err := searchLocationsFunc(location)
	if err != nil {
		return nil, err
	}

	if len(addr) == 0 {
		return nil, fmt.Errorf("no location found for %q", location)
	}

	timezone := os.Getenv("TZ")
	if timezone == "" {
		timezone = "GMT"
	}

	q := openMeteoParams{
		Latitude:  addr[0].Lat,
		Longitude: addr[0].Lon,
		Daily:     strings.Join(attributesDaily, ","),
		Hourly:    strings.Join(attributesHourly, ","),
		Timezone:  timezone,
	}

	return &q, nil
}

type openMeteoParams struct {
	Latitude  string `url:"latitude"`
	Longitude string `url:"longitude"`
	Daily     string `url:"daily"`
	Hourly    string `url:"hourly"`
	Timezone  string `url:"timezone"`
	StartDate string `url:"start_date,omitempty"`
	EndDate   string `url:"end_date,omitempty"`
}
