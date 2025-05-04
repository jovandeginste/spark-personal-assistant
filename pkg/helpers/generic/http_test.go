package generic

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/awterman/monkey"
	"github.com/stretchr/testify/assert"
)

// mockReadCloser is a test helper to implement io.ReadCloser and track calls.
type mockReadCloser struct {
	io.Reader
	closeCalled bool
	closeErr    error // Error to return from Close()
}

// Close records that it was called and returns the predefined error.
func (m *mockReadCloser) Close() error {
	m.closeCalled = true
	return m.closeErr
}

// newMockReadCloser creates a mockReadCloser with a static data buffer.
func newMockReadCloser(data []byte, closeErr error) *mockReadCloser {
	return &mockReadCloser{
		Reader:   bytes.NewReader(data),
		closeErr: closeErr,
	}
}

// errorReader is a test helper to implement io.Reader and return a specific error.
type errorReader struct {
	err error // Error to return from Read()
}

// Read returns the predefined error.
func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

// newErrorReadCloser creates a mockReadCloser that returns a read error.
func newErrorReadCloser(readErr, closeErr error) *mockReadCloser {
	return &mockReadCloser{
		Reader:   &errorReader{err: readErr},
		closeErr: closeErr,
	}
}

func Test_getBody(t *testing.T) {
	mockURL := "http://example.com/test"
	mockBodyContent := []byte("mock response body")

	tests := []struct {
		name              string
		remoteURL         string
		mockResponse      *http.Response
		mockErr           error
		mockReadErr       error // Error to simulate during io.ReadAll
		mockCloseErr      error // Error to simulate during Body.Close
		expectError       bool
		expectedErr       error // Specific error to match if expectError is true
		expectedBody      []byte
		expectCloseCalled bool
	}{
		{
			name:              "Successful GET request",
			remoteURL:         mockURL,
			mockResponse:      &http.Response{StatusCode: http.StatusOK, Body: newMockReadCloser(mockBodyContent, nil)},
			mockErr:           nil,
			mockReadErr:       nil,
			mockCloseErr:      nil,
			expectError:       false,
			expectedBody:      mockBodyContent,
			expectCloseCalled: true,
		},
		{
			name:              "Successful GET request with empty body",
			remoteURL:         mockURL,
			mockResponse:      &http.Response{StatusCode: http.StatusOK, Body: newMockReadCloser([]byte{}, nil)},
			mockErr:           nil,
			mockReadErr:       nil,
			mockCloseErr:      nil,
			expectError:       false,
			expectedBody:      []byte{},
			expectCloseCalled: true,
		},
		{
			name:              "http.Client.Do returns error",
			remoteURL:         mockURL,
			mockResponse:      nil, // Response is nil on error
			mockErr:           errors.New("network error"),
			mockReadErr:       nil, // Not applicable
			mockCloseErr:      nil, // Not applicable
			expectError:       true,
			expectedErr:       errors.New("network error"),
			expectedBody:      nil,   // Body is nil on error
			expectCloseCalled: false, // Close should not be called if Do fails
		},
		{
			name:              "io.ReadAll returns error",
			remoteURL:         mockURL,
			mockResponse:      &http.Response{StatusCode: http.StatusOK, Body: newErrorReadCloser(errors.New("read error"), nil)},
			mockErr:           nil,
			mockReadErr:       errors.New("read error"),
			mockCloseErr:      nil,
			expectError:       true,
			expectedErr:       errors.New("read error"),
			expectedBody:      nil,  // Body might be partially read or nil on error, depends on io.ReadAll internal state
			expectCloseCalled: true, // Close should still be called by defer
		},
		{
			name:              "Body.Close returns error (error is ignored by getBody)",
			remoteURL:         mockURL,
			mockResponse:      &http.Response{StatusCode: http.StatusOK, Body: newMockReadCloser(mockBodyContent, errors.New("close error"))},
			mockErr:           nil,
			mockReadErr:       nil,
			mockCloseErr:      errors.New("close error"),
			expectError:       false, // getBody does not check the close error
			expectedBody:      mockBodyContent,
			expectCloseCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use monkey.PatchInstanceMethod to patch the Do method on http.DefaultClient
			// We need to store the request passed to the patched method to assert headers
			var capturedReq *http.Request
			var mockBody *mockReadCloser // To check if Close was called

			// Determine which mock body to use based on the test case
			if tt.mockResponse != nil && tt.mockResponse.Body != nil {
				// Cast the mock response body to our mockReadCloser type
				// Note: This assumes the test setup correctly provides a mockReadCloser
				// in tt.mockResponse.Body when needed.
				if rc, ok := tt.mockResponse.Body.(*mockReadCloser); ok {
					mockBody = rc
				}
			}

			patch := monkey.Method(nil, http.DefaultClient, http.DefaultClient.Do, func(req *http.Request) (*http.Response, error) {
				// Capture the request for assertion later
				capturedReq = req
				// Return the mock response and error defined in the test case
				return tt.mockResponse, tt.mockErr
			})
			defer patch.Reset()

			// Call the function under test
			body, err := getBody(tt.remoteURL)

			// Assert error expectations
			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err, "Error mismatch")
				}
			} else {
				assert.NoError(t, err, "Did not expect an error")
				assert.Equal(t, tt.expectedBody, body, "Returned body mismatch")
			}

			// Assert that the correct URL was requested
			if capturedReq != nil {
				assert.Equal(t, tt.remoteURL, capturedReq.URL.String(), "Requested URL mismatch")
			} else if !tt.expectError || tt.mockErr == nil {
				// If no error was expected, or the error happened *after* the Do call (like ReadAll),
				// the request should have been captured. If Do itself failed, capturedReq will be nil.
				assert.Nil(t, capturedReq, "Request should be nil if http.Client.Do returned an error")
			}

			// Assert that the User-Agent header was set
			if capturedReq != nil {
				assert.Equal(t, "Spark", capturedReq.Header.Get("User-Agent"), "User-Agent header not set correctly")
			}

			// Assert that Close was called on the response body if applicable
			if mockBody != nil {
				assert.Equal(t, tt.expectCloseCalled, mockBody.closeCalled, "Body.Close() call mismatch")
				// Note: We don't check the return value of Close here, as getBody ignores it.
			}
		})
	}
}

// TestGetBodyVariable ensures the exported variable GetBody points to the internal getBody function.
func TestGetBodyVariable(t *testing.T) {
	// Use reflection to check if the exported variable points to the internal function.
	v := reflect.ValueOf(GetBody)
	f := reflect.ValueOf(getBody)

	assert.Equal(t, f.Pointer(), v.Pointer(), "Exported variable GetBody does not point to internal function getBody")
}
