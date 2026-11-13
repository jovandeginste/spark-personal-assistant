package googlecontacts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/people/v1"
)

// getClient returns an HTTP client authorized for the Google People API.
// It requires a previously saved token; authorization is not interactive.
func getClient(cfg Config) (*http.Client, error) {
	oauthConfig, err := google.ConfigFromJSON([]byte(cfg.Init), people.ContactsReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse google client config: %w", err)
	}

	tok, err := tokenFromFile(cfg.TokenFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read google contacts token file %q: %w", cfg.TokenFile, err)
	}

	return oauthConfig.Client(context.Background(), tok), nil
}

// tokenFromFile retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)

	return tok, err
}
