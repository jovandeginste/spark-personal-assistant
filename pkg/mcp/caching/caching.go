package caching

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/TwiN/gocache/v2"
)

// Cache defines the interface for caching operations
type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte)
	GetFile(key string) (string, bool)
	SetFile(key string, dataProvider func() (io.ReadCloser, error)) (string, error)
	ForceUpdateFile(key string, dataProvider func() (io.ReadCloser, error)) (string, error)
}

// Service implements the Cache interface using both in-memory and file-based strategies
type Service struct {
	memoryCache *gocache.Cache
	storageDir  string
	ttl         time.Duration
}

// NewService creates a new caching service
func NewService(storageDir string, ttl time.Duration) (*Service, error) {
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Initialize memory cache with provided TTL and max size (e.g., 1000 items)
	// Using 1000 items as reasonable default for metadata/small items
	memCache := gocache.NewCache().WithEvictionPolicy(gocache.LeastRecentlyUsed).WithMaxSize(1000).WithDefaultTTL(ttl)
	memCache.StartJanitor()

	return &Service{
		memoryCache: memCache,
		storageDir:  storageDir,
		ttl:         ttl,
	}, nil
}

// Get retrieves an item from the in-memory cache
func (s *Service) Get(key string) ([]byte, bool) {
	val, exists := s.memoryCache.Get(key)
	if !exists {
		return nil, false
	}
	// Assert type to []byte
	bytes, ok := val.([]byte)

	return bytes, ok
}

// Set adds an item to the in-memory cache
func (s *Service) Set(key string, value []byte) {
	s.memoryCache.Set(key, value)
}

// GetFile retrieves the path to a cached file if it exists and hasn't expired
func (s *Service) GetFile(key string) (string, bool) {
	filename := s.getFilePath(key)

	info, err := os.Stat(filename)
	if err != nil {
		return "", false
	}

	if time.Since(info.ModTime()) > s.ttl {
		return "", false
	}

	return filename, true
}

// SetFile writes data to a file in the cache directory and returns the path
func (s *Service) SetFile(key string, dataProvider func() (io.ReadCloser, error)) (string, error) {
	filename := s.getFilePath(key)

	rc, err := dataProvider()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	out, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	if err != nil {
		return "", err
	}

	return filename, nil
}

// ForceUpdateFile forces an update of the cached file, bypassing TTL checks
func (s *Service) ForceUpdateFile(key string, dataProvider func() (io.ReadCloser, error)) (string, error) {
	// Directly call SetFile, which overwrites existing file
	return s.SetFile(key, dataProvider)
}

// getFilePath generates the full path for a cache file based on the key
func (s *Service) getFilePath(key string) string {
	// Use SHA256 of key for filesystem-safe filename
	hash := sha256.Sum256([]byte(key))
	// Preserve extension if the key looks like a file/url, otherwise just use hash
	ext := filepath.Ext(key)
	if ext == "" {
		// Default to .cache if no extension found
		ext = ".cache"
	}
	// Sanitize extension to be safe (basic check)
	if len(ext) > 10 {
		ext = ".cache"
	}

	filename := fmt.Sprintf("%x%s", hash, ext)

	return filepath.Join(s.storageDir, filename)
}
