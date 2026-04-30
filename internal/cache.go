package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Cache struct {
	mu   sync.RWMutex
	data map[string]any
	path string
}

func NewCache(cacheDir string) (*Cache, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	c := &Cache{
		data: make(map[string]any),
		path: filepath.Join(cacheDir, "cache.json"),
	}

	if _, err := os.Stat(c.path); err == nil {
		bytes, err := os.ReadFile(c.path)
		if err != nil {
			return nil, err
		}
		if len(bytes) > 0 {
			if err := json.Unmarshal(bytes, &c.data); err != nil {
				return nil, err
			}
		}
	}
	return c, nil
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.data[key]
	return val, ok
}

func (c *Cache) Set(key string, value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = value
	return c.save()
}

func (c *Cache) save() error {
	bytes, err := json.MarshalIndent(c.data, "", "   ")
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
