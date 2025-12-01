package cache

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]Item)
}

// Keys returns a slice of all unexpired keys in the cache.
// The order of keys is not guaranteed.
func (c *Cache) Keys() []string {
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
func (c *Cache) Items() map[string]Item {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := make(map[string]Item, len(c.data))
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
func (c *Cache) ItemCount() int {
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
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.data)
}
