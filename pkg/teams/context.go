package teams

import "sync"

// InMemoryContextCache is an in-memory implementation of ContextCache.
type InMemoryContextCache struct {
	mu    sync.RWMutex
	store map[string]string
}

// NewInMemoryContextCache creates a new InMemoryContextCache.
func NewInMemoryContextCache() *InMemoryContextCache {
	return &InMemoryContextCache{
		store: make(map[string]string),
	}
}

// Store sets a key-value pair in the cache.
func (c *InMemoryContextCache) Store(key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}

// Load retrieves a value by key. Returns the value and whether it was found.
func (c *InMemoryContextCache) Load(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.store[key]
	return val, ok
}

// Delete removes a key from the cache.
func (c *InMemoryContextCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}
