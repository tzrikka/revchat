package cache

// Clear removes all items from the cache.
func (c *Cache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]Item[T])
}

// Keys returns a slice of all unexpired keys in the cache.
// The order of keys is not guaranteed.
func (c *Cache[T]) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.data))
	for k, item := range c.data {
		if !item.Expired() {
			keys = append(keys, k)
		}
	}
	return keys
}

// Items returns a copy of all the unexpired [Item]s in the cache.
func (c *Cache[T]) Items() map[string]Item[T] {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := make(map[string]Item[T], len(c.data))
	for k, item := range c.data {
		if !item.Expired() {
			m[k] = item
		}
	}
	return m
}

// ItemCount returns the number of unexpired items in the cache.
// This is different from [Len], which returns the total number
// of items, including expired-but-still-present ones.
func (c *Cache[T]) ItemCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, item := range c.data {
		if !item.Expired() {
			count++
		}
	}
	return count
}

// Len returns the total number of items in the cache,
// including expired-but-still-present ones.
func (c *Cache[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.data)
}
