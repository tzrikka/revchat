// Package cache provides a simple in-memory concurrency-safe key-value store
// for strings, with optional expiration. Think of it as a basic and local
// version of Redis or https://github.com/patrickmn/go-cache.
package cache

import (
	"sync"
	"time"
)

// Cache is an in-memory concurrency-safe key-value store for strings, with optional expiration.
// Think of it as a basic and local version of Redis or https://github.com/patrickmn/go-cache.
type Cache struct {
	mu   sync.RWMutex
	data map[string]Item

	defaultExpiration time.Duration
}

// Item represents a single cache item, with its value and expiration time.
// An Expiration of zero means the item never expires.
type Item struct {
	Value      string
	Expiration time.Time
}

// Expired checks if the item is expired.
func (i *Item) Expired() bool {
	if i.Expiration.IsZero() {
		return false
	}
	return time.Now().After(i.Expiration)
}

const (
	DefaultCleanupInterval time.Duration = 61 * time.Minute // Prime number & slightly over an hour.
	DefaultExpiration      time.Duration = 0
	NoCleanup              time.Duration = 0 // For use in [New] only.
	NoExpiration           time.Duration = -1
	KeepTTL                time.Duration = -2 // For use in [Replace] only.
)

// New creates a new [Cache] instance.
func New(defaultExpiration, cleanupInterval time.Duration) *Cache {
	if defaultExpiration <= DefaultExpiration {
		defaultExpiration = NoExpiration
	}

	return &Cache{
		data:              make(map[string]Item),
		defaultExpiration: defaultExpiration,
	}
}

// Del removes a specified item from the cache. If the item does not exist, this is a no-op.
func (c *Cache) Del(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
}

// Get retrieves a value from the cache, and also returns a boolean indicating if it was
// found and not expired. It also handles lazy expiration (deleting expired items).
func (c *Cache) Get(key string) (string, bool) {
	item, ok := c.Item(key)
	return item.Value, ok
}

// Item retrieves a copy of an [Item] from the cache, and also returns a boolean indicating
// if it was found and not expired. It also handles lazy expiration (deleting expired items).
func (c *Cache) Item(key string) (Item, bool) {
	c.mu.RLock()
	item, ok := c.data[key]
	c.mu.RUnlock()

	if !ok {
		return Item{}, false
	}

	if item.Expired() {
		c.Del(key)
		return Item{}, false
	}

	return item, true
}

func (c *Cache) expirationTime(ttl time.Duration) time.Time {
	if ttl == DefaultExpiration {
		ttl = c.defaultExpiration
	}
	if ttl > DefaultExpiration {
		return time.Now().Add(ttl)
	}
	return time.Time{}
}

// Set adds a value to the cache with an optional Time-To-Live duration
// until it expires (see also [DefaultExpiration] and [NoExpiration]).
// Note that any negative TTL value is treated as [NoExpiration].
func (c *Cache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = Item{
		Value:      value,
		Expiration: c.expirationTime(ttl),
	}
}

// Add adds a value to the cache only if the key does not already exist
// or is expired. It returns true if the item was added, false otherwise.
// Note that any negative TTL value is treated as [NoExpiration].
func (c *Cache) Add(key, value string, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.data[key]; ok && !item.Expired() {
		return false
	}

	c.data[key] = Item{
		Value:      value,
		Expiration: c.expirationTime(ttl),
	}
	return true
}

// Replace updates a value in the cache only if the key already exists and is not expired.
// It returns true if the item was replaced, false otherwise. See also [KeepTTL], and
// note that any negative TTL value is treated as [NoExpiration].
func (c *Cache) Replace(key, value string, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.data[key]
	if !ok || item.Expired() {
		delete(c.data, key)
		return false
	}

	item.Value = value
	if ttl != KeepTTL {
		item.Expiration = c.expirationTime(ttl)
	}

	c.data[key] = item
	return true
}
