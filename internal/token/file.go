package token

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// FileCache stores tokens in a JSON file on disk.
type FileCache struct {
	Path string
}

type fileCacheEntry struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

type fileCacheData map[string]fileCacheEntry

func (c *FileCache) Get(_ context.Context, key string) (string, error) {
	data, err := c.readAll()
	if err != nil {
		return "", nil // treat read errors as cache miss
	}
	entry, ok := data[key]
	if !ok || time.Now().After(entry.Expires) {
		return "", nil
	}
	return entry.Token, nil
}

func (c *FileCache) Set(_ context.Context, key string, token string, ttl time.Duration) error {
	data, _ := c.readAll()
	if data == nil {
		data = make(fileCacheData)
	}
	// Remove expired entries
	now := time.Now()
	for k, v := range data {
		if now.After(v.Expires) {
			delete(data, k)
		}
	}

	data[key] = fileCacheEntry{
		Token:   token,
		Expires: now.Add(ttl),
	}

	if err := os.MkdirAll(filepath.Dir(c.Path), 0700); err != nil {
		return err
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(c.Path, raw, 0600)
}

func (c *FileCache) readAll() (fileCacheData, error) {
	raw, err := os.ReadFile(c.Path)
	if err != nil {
		return nil, err
	}
	var data fileCacheData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}
