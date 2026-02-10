package caching

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	tempDir := t.TempDir()
	service, err := NewService(tempDir, time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.DirExists(t, tempDir)
}

func TestMemoryCache(t *testing.T) {
	tempDir := t.TempDir()
	service, _ := NewService(tempDir, time.Minute)

	// Test Set and Get
	key := "test-key"
	value := []byte("test-value")
	service.Set(key, value)

	retrieved, exists := service.Get(key)
	assert.True(t, exists)
	assert.Equal(t, value, retrieved)

	// Test non-existent key
	_, exists = service.Get("non-existent")
	assert.False(t, exists)
}

func TestFileCache(t *testing.T) {
	tempDir := t.TempDir()
	// Use a short TTL for testing expiration
	service, _ := NewService(tempDir, 100*time.Millisecond)

	key := "file-key.txt"
	content := []byte("file-content")

	// Helper to provide data
	dataProvider := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(content)), nil
	}

	// Test SetFile
	path, err := service.SetFile(key, dataProvider)
	require.NoError(t, err)
	assert.FileExists(t, path)

	// Verify content
	savedContent, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, savedContent)

	// Test GetFile (cache hit)
	path2, exists := service.GetFile(key)
	assert.True(t, exists)
	assert.Equal(t, path, path2)

	// Test Expiration
	time.Sleep(200 * time.Millisecond)
	_, exists = service.GetFile(key)
	assert.False(t, exists, "File should have expired")
}

func TestForceUpdateFile(t *testing.T) {
	tempDir := t.TempDir()
	service, _ := NewService(tempDir, time.Minute)

	key := "update-key"
	content1 := []byte("content-1")
	path, err := service.SetFile(key, func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(content1)), nil
	})
	require.NoError(t, err)

	content2 := []byte("content-2")
	path2, err := service.ForceUpdateFile(key, func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(content2)), nil
	})
	require.NoError(t, err)
	assert.Equal(t, path, path2)

	savedContent, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content2, savedContent)
}

func TestClear(t *testing.T) {
	tempDir := t.TempDir()
	service, _ := NewService(tempDir, time.Minute)

	// Set items in both caches
	service.Set("mem", []byte("val"))
	_, err := service.SetFile("file", func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte("val"))), nil
	})
	require.NoError(t, err)

	// Clear
	err = service.Clear()
	require.NoError(t, err)

	// Verify empty
	_, exists := service.Get("mem")
	assert.False(t, exists)

	// Since Clear removes the directory, check that or if files are gone
	// The implementation does os.RemoveAll(storageDir)
	assert.NoDirExists(t, tempDir)
}

func TestPathSafety(t *testing.T) {
	tempDir := t.TempDir()
	service, _ := NewService(tempDir, time.Minute)

	// Attempt directory traversal in key
	// Note: The current implementation hashes the key, so this acts more as a verification
	// that even with malicious keys, we get a safe filename within the storage dir.
	key := "../../etc/passwd"
	
	// We access unexported getFilePath using a trick or just inspect the result of SetFile
	// Since we are in the same package (caching), we can call getFilePath if we change package name to caching
	// But commonly tests are in caching_test package.
	// Let's assume this file is `package caching` based on the file creation.
	
	path := service.getFilePath(key)
	assert.NotEmpty(t, path)
	assert.Contains(t, path, tempDir)
	
	// Check that it's NOT pointing to /etc/passwd
	rel, err := filepath.Rel(tempDir, path)
	require.NoError(t, err)
	assert.False(t, filepath.IsAbs(rel))
	assert.NotContains(t, rel, "..")
}
