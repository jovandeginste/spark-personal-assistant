package twizzit

import (
	"log/slog"
	"net/url"
	"os"
	"testing"
)

func TestDebugGetSubscriptions(t *testing.T) {
	// Setup logger to stdout
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Config
	config := Config{
		PHPSessionID: "fake-session-id",
	}

	// Create instance
	twizzit := New(config, logger)

	// URL and Form Data for GetSubscriptions
	u := "https://app.twizzit.com/v2/ajax/forms"
	form := url.Values{}
	form.Add("archive", "false")
	form.Add("searchString", "")

	// Call makeRequest directly
	res, _, err := twizzit.makeRequest("POST", u, form)
	if err != nil {
		if err.Error() == "twizzit session expired" {
			t.Skipf("skipping test, session expired")
		}
		t.Fatalf("makeRequest failed: %v", err)
	}

	if res.IsError {
		t.Logf("Result is error: %v", res.Content)
	} else {
		for _, c := range res.Content {
			if tc, ok := c.(interface{ GetText() string }); ok { // basic assertion
				t.Logf("Response Content: %s", tc.GetText())
			} else {
				// fallback using reflection or just printing
				t.Logf("Response Content (raw): %v", c)
			}
		}
	}
}

func TestDebugGetNotifications(t *testing.T) {
	// Setup logger to stdout
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Config
	config := Config{
		PHPSessionID: "fake-session-id",
	}

	// Create instance
	twizzit := New(config, logger)

	// URL for GetNotifications
	u := "https://app.twizzit.com/v2/ajax/my/notifications?amount=20&offset=0"

	res, _, err := twizzit.makeRequest("GET", u)
	if err != nil {
		if err.Error() == "twizzit session expired" {
			t.Skipf("skipping test, session expired")
		}
		t.Fatalf("makeRequest failed: %v", err)
	}

	if res.IsError {
		t.Errorf("Result is error: %v", res.Content)
	} else {
		t.Logf("GetNotifications success")
	}
}

func TestDebugGetSubscriptionForms(t *testing.T) {
	// Setup logger to stdout
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Config
	config := Config{
		PHPSessionID: "fake-session-id",
	}

	// Create instance
	twizzit := New(config, logger)

	// URL for GetSubscriptionForms
	u := "https://app.twizzit.com/v2/ajax/forms"
	form := url.Values{}
	form.Add("archive", "false")
	form.Add("searchString", "")

	res, _, err := twizzit.makeRequest("POST", u, form)
	if err != nil {
		if err.Error() == "twizzit session expired" {
			t.Skipf("skipping test, session expired")
		}
		t.Fatalf("makeRequest failed: %v", err)
	}

	if res.IsError {
		t.Errorf("Result is error: %v", res.Content)
	} else {
		// Log response for inspection
		/*
			for _, c := range res.Content {
				if tc, ok := c.(interface{ GetText() string }); ok {
					t.Logf("Response Content: %s", tc.GetText())
				}
			}
		*/
		t.Logf("GetSubscriptionForms success")
	}
}
