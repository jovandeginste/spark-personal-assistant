package twizzit

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (t *Twizzit) makeRequest(method, requestURL string, formData ...url.Values) (*sdk.CallToolResult, any, error) {
	t.Logger().Debug("making request to twizzit", "method", method, "url", requestURL)

	var body io.Reader
	if len(formData) > 0 {
		body = strings.NewReader(formData[0].Encode())
	}

	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		if t.config.Username != "" && t.config.Password != "" {
			t.Logger().Info("twizzit returned 401 Unauthorized, attempting login")
			if err := t.login(); err != nil {
				return nil, nil, fmt.Errorf("login failed after 401: %w", err)
			}
			// Retry request once
			return t.makeRequest(method, requestURL, formData...)
		}
		return nil, nil, errors.New("twizzit unauthorized")
	}

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		location, err := resp.Location()
		if err == nil {
			if location.String() == "https://app.twizzit.com/v2/login?expired=1" {
				if t.config.Username != "" && t.config.Password != "" {
					t.Logger().Info("twizzit session expired, attempting login")
					if err := t.login(); err != nil {
						return nil, nil, fmt.Errorf("twizzit session expired and login failed: %w", err)
					}
					// Retry request once
					return t.makeRequest(method, requestURL, formData...)
				}
				return nil, nil, errors.New("twizzit session expired")
			}
		}
	}

	respBody, err := io.ReadAll(resp.Body)

	return &sdk.CallToolResult{
		Content: []sdk.Content{
			&sdk.TextContent{
				Text: string(respBody),
			},
		},
	}, nil, err
}

func (t *Twizzit) login() error {
	// 1. Fetch login page to get PHPSESSID and token
	req, err := http.NewRequest(http.MethodGet, "https://app.twizzit.com/v2/login?locale=nl", nil)
	if err != nil {
		return err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	token, err := extractLoginToken(string(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to extract token from login page: %w", err)
	}

	// 2. Perform login POST
	form := url.Values{}
	form.Set("mobileDeviceType", "")
	form.Set("mobileDeviceId", "")
	form.Set("token", token)
	form.Set("username", t.config.Username)
	form.Set("password", t.config.Password)
	form.Set("authCode", "")

	req, err = http.NewRequest(http.MethodPost, "https://app.twizzit.com/v2/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check if login was successful (usually a redirect to dashboard or home)
	// Or check if we are NOT redirected back to login?
	// The original makeRequest will handle subsequent requests, here we just need to ensure the session is updated.
	// Since we use a cookiejar, cookies are updated automatically.

	// A successful login usually redirects.
	// If we get redirected back to login with error, that's a failure.
	// If we are redirected to /v2/home or similar, success.

	// Let's check the location if it's a redirect
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		loc, err := resp.Location()
		if err == nil {
			if strings.Contains(loc.String(), "login?error") {
				return errors.New("login failed: invalid credentials or other error")
			}

			// Just hit the dashboard to finalize the session
			dashboardURL := "https://app.twizzit.com/v2/dashboard"
			if t.config.OrganizationID != 0 {
				dashboardURL = fmt.Sprintf("%s?organization-id=%d", dashboardURL, t.config.OrganizationID)
			}
			t.Logger().Debug("calling dashboard to finalize login", "url", dashboardURL)
			req, err = http.NewRequest(http.MethodGet, dashboardURL, nil)
			if err != nil {
				t.Logger().Error("error calling dashboard url", "error", err)
				return fmt.Errorf("failed to create dashboard request: %w", err)
			}

			// Reuse the outer resp variable to avoid shadowing
			var dashResp *http.Response
			dashResp, err = t.client.Do(req)
			if err != nil {
				t.Logger().Error("error calling dashboard url", "error", err)
				return fmt.Errorf("failed to call dashboard: %w", err)
			}
			defer dashResp.Body.Close()
			// We don't need the body, just the session update via cookiejar
		}
	}

	return nil
}
