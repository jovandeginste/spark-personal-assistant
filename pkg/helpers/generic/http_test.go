package generic

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/awterman/monkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockReadCloser is a simple io.ReadCloser for testing read errors.
type MockReadCloser struct {
	Reader io.Reader
	Err    error // Error to return on Read
}

func (m *MockReadCloser) Read(p []byte) (n int, err error) {
	if m.Err != nil {
		return 0, m.Err
	}
	return m.Reader.Read(p)
}

func (m *MockReadCloser) Close() error {
	// Do nothing or track close if needed
	return nil
}

func TestGetBody_Success(t *testing.T) {
	testData := []byte("This is the body content.")
	mockReadCloser := io.NopCloser(bytes.NewReader(testData)) // Use io.NopCloser for simplicity

	// Patch ReadResource to return our mock reader
	patch := monkey.Func(nil, ReadResource, func(uri string) (io.ReadCloser, error) {
		assert.Equal(t, "http://example.com/resource", uri, "ReadResource called with incorrect URI")
		return mockReadCloser, nil
	})
	defer patch.Reset()

	// Call the function under test
	body, err := getBody("http://example.com/resource")

	// Assertions
	assert.NoError(t, err, "Expected no error")
	assert.Equal(t, testData, body, "Received body content mismatch")
}

func TestGetBody_ReadResourceError(t *testing.T) {
	mockError := errors.New("failed to open resource")

	// Patch ReadResource to return an error
	patch := monkey.Func(nil, ReadResource, func(uri string) (io.ReadCloser, error) {
		assert.Equal(t, "file:///path/to/resource", uri, "ReadResource called with incorrect URI")
		return nil, mockError
	})
	defer patch.Reset()

	// Call the function under test
	body, err := getBody("file:///path/to/resource")

	// Assertions
	assert.Error(t, err, "Expected an error")
	assert.Equal(t, mockError, err, "Error mismatch")
	assert.Nil(t, body, "Expected nil body on error")
}

func TestGetBody_ReadError(t *testing.T) {
	mockReadError := errors.New("simulated read error")
	// Create a reader that will error on the first read
	errorReader := &MockReadCloser{
		Reader: strings.NewReader("partial content"), // Content doesn't matter, error comes first
		Err:    mockReadError,
	}

	// Patch ReadResource to return our error-prone reader
	patch := monkey.Func(nil, ReadResource, func(uri string) (io.ReadCloser, error) {
		assert.Equal(t, "http://another.com/resource", uri, "ReadResource called with incorrect URI")
		return errorReader, nil
	})
	defer patch.Reset()

	// Call the function under test
	body, err := getBody("http://another.com/resource")

	// Assertions
	assert.Error(t, err, "Expected an error during read")
	assert.Equal(t, mockReadError, err, "Read error mismatch")
	assert.Empty(t, body, "Expected nil body on read error")

	// Note: io.ReadAll handles closing the reader internally, even on error.
	// We don't need to explicitly test closing here unless the Close method
	// has specific side effects we need to verify.
}

func TestGetBody_EmptyBody(t *testing.T) {
	testData := []byte("") // Empty body
	mockReadCloser := io.NopCloser(bytes.NewReader(testData))

	patch := monkey.Func(nil, ReadResource, func(uri string) (io.ReadCloser, error) {
		assert.Equal(t, "http://empty.com/resource", uri, "ReadResource called with incorrect URI")
		return mockReadCloser, nil
	})
	defer patch.Reset()

	body, err := getBody("http://empty.com/resource")

	assert.NoError(t, err, "Expected no error for empty body")
	require.NotNil(t, body, "Expected non-nil body slice for empty content")
	assert.Empty(t, body, "Expected empty body slice")
	assert.Equal(t, testData, body, "Received body content mismatch for empty body")
}

func TestGetBody_LargeBody(t *testing.T) {
	// Create a moderately large body
	largeData := bytes.Repeat([]byte("a"), 1024*10) // 10KB
	mockReadCloser := io.NopCloser(bytes.NewReader(largeData))

	patch := monkey.Func(nil, ReadResource, func(uri string) (io.ReadCloser, error) {
		assert.Equal(t, "http://large.com/resource", uri, "ReadResource called with incorrect URI")
		return mockReadCloser, nil
	})
	defer patch.Reset()

	body, err := getBody("http://large.com/resource")

	assert.NoError(t, err, "Expected no error for large body")
	assert.Equal(t, largeData, body, "Received body content mismatch for large body")
}
