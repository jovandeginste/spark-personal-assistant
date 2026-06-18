package recycleapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
)

const (
	consumer = "recycleapp.be"
	baseURL  = "https://api.fostplus.be/recyclecms/public/v1"
)

var (
	collectionsURL, _   = url.JoinPath(baseURL, "collections")
	streetsURL, _       = url.JoinPath(baseURL, "streets")
	zipcodesURL, _      = url.JoinPath(baseURL, "zipcodes")
	organizationsURL, _ = url.JoinPath(baseURL, "organizations/")
)

type Client struct {
	HTTPClient *http.Client
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		HTTPClient: httpClient,
	}
}

func (c *Client) getJSON(ctx context.Context, fullURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("x-consumer", consumer)

	r, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

func (c *Client) GetOrganization(ctx context.Context, zipcode string) (*Organization, error) {
	fullURL := organizationsURL + zipcode

	slog.Info("Fetching URL", "url", fullURL)

	var result Organization

	if err := c.getJSON(ctx, fullURL, &result); err != nil {
		return nil, err
	}

	if result.Name == "" {
		return nil, fmt.Errorf("%w: %s", ErrOrganizationNoResult, zipcode)
	}

	slog.Info("Organization detail", "name", result.Name)

	return &result, nil
}

func (c *Client) GetStreetID(ctx context.Context, zipcode, street string) (string, error) {
	v := url.Values{}

	v.Set("q", street)
	v.Set("zipcodes", zipcode)

	slog.Info("Fetching streets", "url", streetsURL, "values", v)

	fullURL := streetsURL + "?" + v.Encode()

	var result StreetResponse

	if err := c.getJSON(ctx, fullURL, &result); err != nil {
		return "", err
	}

	if len(result.Items) == 0 {
		return "", fmt.Errorf("%w: zipcode=%#v street=%#v", ErrStreetNoResult, zipcode, street)
	}

	if result.Items[0].ID == "" {
		return "", fmt.Errorf("%w: zipcode=%#v street=%#v", ErrStreetNoResult, zipcode, street)
	}

	slog.Info("Street ID details", "id", result.Items[0].ID)

	return result.Items[0].ID, nil
}

func (c *Client) GetZipcodeID(ctx context.Context, zipcode int) (string, error) {
	v := url.Values{}

	v.Set("q", strconv.Itoa(zipcode))

	slog.Info("Fetching zipcodes", "url", zipcodesURL, "values", v)

	fullURL := zipcodesURL + "?" + v.Encode()

	var result ZipResponse

	if err := c.getJSON(ctx, fullURL, &result); err != nil {
		return "", err
	}

	if len(result.Items) == 0 {
		return "", fmt.Errorf("%w: %d", ErrZipcodeNoResult, zipcode)
	}

	if result.Items[0].ID == "" {
		return "", fmt.Errorf("%w: %d", ErrZipcodeNoResult, zipcode)
	}

	slog.Info("Zipcode ID details", "id", result.Items[0].ID)

	return result.Items[0].ID, nil
}

type CollectionsParams struct {
	ZipcodeID   string
	StreetID    string
	HouseNumber int
	FromDate    string
	UntilDate   string
	Lang        string
}

func (c *Client) GetCollections(ctx context.Context, params CollectionsParams) (RecycleInfo, error) {
	v := url.Values{}
	v.Set("zipcodeId", params.ZipcodeID)
	v.Set("streetId", params.StreetID)
	v.Set("houseNumber", strconv.Itoa(params.HouseNumber))
	v.Set("fromDate", params.FromDate)
	v.Set("untilDate", params.UntilDate)
	v.Set("size", "200")

	slog.Info("Fetching collections", "url", collectionsURL, "values", v)

	fullURL := collectionsURL + "?" + v.Encode()

	var result RecycleInfo
	if err := c.getJSON(ctx, fullURL, &result); err != nil {
		return RecycleInfo{}, err
	}

	result.Lang = params.Lang
	return result, nil
}
