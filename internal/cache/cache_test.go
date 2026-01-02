package cache_test

import (
	"testing"
	"time"

	"github.com/tzrikka/revchat/internal/cache"
)

func TestCacheNoExpiration(t *testing.T) {
	c := cache.New[string](cache.NoExpiration, cache.NoCleanup)
	k, v := "key1", "val1"

	wantInt := 0
	if got := c.Len(); got != wantInt {
		t.Errorf("Cache.Len() = %d, want %d", got, wantInt)
	}
	if got := c.ItemCount(); got != wantInt {
		t.Errorf("Cache.ItemCount() = %d, want %d", got, wantInt)
	}

	c.Set(k, v, cache.DefaultExpiration)

	wantInt = 1
	if got := c.Len(); got != wantInt {
		t.Errorf("Cache.Len() = %d, want %d", got, wantInt)
	}
	if got := c.ItemCount(); got != wantInt {
		t.Errorf("Cache.ItemCount() = %d, want %d", got, wantInt)
	}

	if got, found := c.Get(k); !found || got != v {
		t.Errorf("Cache.Get() = %q, %v; want %q, true", got, found, v)
	}
	if got, found := c.Item(k); !found || got.Value != v || !got.Expiration.IsZero() {
		t.Errorf("Cache.Item() = {%q, %v}, %v; want {%q, zero value}, true", got.Value, got.Expiration, found, v)
	}

	c.Del(k)

	wantInt = 0
	if got := c.Len(); got != wantInt {
		t.Errorf("Cache.Len() = %d, want %d", got, wantInt)
	}
	if got := c.ItemCount(); got != wantInt {
		t.Errorf("Cache.ItemCount() = %d, want %d", got, wantInt)
	}

	if _, found := c.Get(k); found {
		t.Errorf("Cache.Get() found deleted key: %s", k)
	}
	if _, found := c.Item(k); found {
		t.Errorf("Cache.Item() found deleted key: %s", k)
	}
}

func TestCacheWithExpiration(t *testing.T) {
	c := cache.New[string](1*time.Nanosecond, cache.NoCleanup)
	k, v := "key1", "val1"
	c.Set(k, v, cache.DefaultExpiration)

	if got := c.Len(); got != 1 {
		t.Errorf("Cache.Len() = %d, want %d", got, 1)
	}

	if got := c.ItemCount(); got != 0 {
		t.Errorf("Cache.ItemCount() = %d, want %d", got, 0)
	}

	if _, found := c.Get(k); found {
		t.Errorf("Cache.Get() found expired key: %s", k)
	}
	if _, found := c.Item(k); found {
		t.Errorf("Cache.Item() found expired key: %s", k)
	}

	c.Set(k, v, 2*time.Nanosecond)

	if got := c.Len(); got != 1 {
		t.Errorf("Cache.Len() = %d, want %d", got, 1)
	}

	if got := c.ItemCount(); got != 0 {
		t.Errorf("Cache.ItemCount() = %d, want %d", got, 0)
	}

	if _, found := c.Get(k); found {
		t.Errorf("Cache.Get() found expired key: %s", k)
	}
	if _, found := c.Item(k); found {
		t.Errorf("Cache.Item() found expired key: %s", k)
	}
}

func TestCacheItemCopy(t *testing.T) {
	c := cache.New[string](cache.NoExpiration, cache.NoCleanup)
	k, v := "key1", "val1"
	c.Set(k, v, cache.DefaultExpiration)

	item1, found1 := c.Item(k)
	if !found1 {
		t.Fatalf("Cache.Item() did not find key: %s", k)
	}

	item1.Value = "val2"
	item1.Expiration = time.Now()

	item2, found2 := c.Item(k)
	if !found2 {
		t.Fatalf("Cache.Item() did not find key: %s", k)
	}

	if item2.Value != v {
		t.Errorf("Cache item was modified through copy: got %q, want %q", item2.Value, v)
	}
	if !item2.Expiration.IsZero() {
		t.Errorf("Cache item expiration was modified through copy: got %v, want zero value", item2.Expiration)
	}
}

func TestCacheItemsCopy(t *testing.T) {
	c := cache.New[string](cache.NoExpiration, cache.NoCleanup)
	k, v := "key1", "val1"
	c.Set(k, v, cache.DefaultExpiration)

	items1 := c.Items()
	if len(items1) != 1 || items1[k].Value != v {
		t.Fatalf("Cache.Items() did not find key: %s", k)
	}

	items1[k] = cache.Item[string]{
		Value:      "val2",
		Expiration: time.Now(),
	}

	items2 := c.Items()
	if len(items2) != 1 || items2[k].Value != v {
		t.Fatalf("Cache.Items() did not find key: %s", k)
	}

	if items2[k].Value != v {
		t.Errorf("Cache item was modified through copy: got %q, want %q", items2[k].Value, v)
	}
	if !items2[k].Expiration.IsZero() {
		t.Errorf("Cache item expiration was modified through copy: got %v, want zero value", items2[k].Expiration)
	}
}

func TestCacheAdd(t *testing.T) {
	c := cache.New[string](cache.NoExpiration, cache.NoCleanup)
	k, v := "key1", "val1"

	if added := c.Add(k, v, cache.DefaultExpiration); !added {
		t.Errorf("Cache.Add() = %v, want true", added)
	}

	if added := c.Add(k, v, cache.DefaultExpiration); added {
		t.Errorf("Cache.Add() = %v, want false", added)
	}

	if got, found := c.Get(k); !found || got != v {
		t.Errorf("Cache.Get() = %q, %v; want %q, true", got, found, v)
	}
}

func TestCacheReplace(t *testing.T) {
	c := cache.New[string](cache.NoExpiration, cache.NoCleanup)
	k, v1, v2 := "key1", "val1", "val2"

	if replaced := c.Replace(k, v1, cache.DefaultExpiration); replaced {
		t.Errorf("Cache.Replace() = %v; want false", replaced)
	}

	c.Set(k, v1, cache.DefaultExpiration)

	if replaced := c.Replace(k, v2, 1*time.Hour); !replaced {
		t.Errorf("Cache.Replace() = %v; want true", replaced)
	}
	if got, found := c.Get(k); !found || got != v2 {
		t.Errorf("Cache.Get() = %q, %v; want %q, true", got, found, v2)
	}
	if item, found := c.Item(k); !found || item.Expiration.IsZero() {
		t.Errorf("Cache.Item() expiration was not replaced: got %v, want non-zero", item.Expiration)
	}
}

func TestCacheReplaceKeepNoExpiration(t *testing.T) {
	c := cache.New[string](cache.NoExpiration, cache.NoCleanup)
	k, v := "key1", "val1"
	c.Set(k, v, cache.DefaultExpiration)

	if replaced := c.Replace(k, "val2", cache.KeepTTL); !replaced {
		t.Errorf("Cache.Replace() = %v, want true", replaced)
	}

	item, found := c.Item(k)
	if !found {
		t.Fatalf("Cache.Item() did not find key: %s", k)
	}

	if item.Value != "val2" {
		t.Errorf("Cache.Replace() did not update value: got %q, want %q", item.Value, "val2")
	}

	if !item.Expiration.IsZero() {
		t.Errorf("Cache.Replace() did not keep TTL: got %v, want zero value", item.Expiration)
	}
}

func TestCacheReplaceKeepExactTTL(t *testing.T) {
	c := cache.New[string](1*time.Hour, cache.NoCleanup)
	k, v1, v2 := "key1", "val1", "val2"

	c.Set(k, v1, 3*time.Hour)
	item1, found := c.Item(k)
	if !found {
		t.Fatalf("Cache.Item() = false (key %q not found), want true", k)
	}
	if item1.Value != v1 {
		t.Fatalf("Cache.Item().Value = %q, want %q", item1.Value, v1)
	}
	if item1.Expiration.IsZero() || time.Until(item1.Expiration) < 2*time.Hour || time.Until(item1.Expiration) >= 3*time.Hour {
		t.Fatalf("Cache.Item().Expiration = %v, want a little less than 3 hours", item1.Expiration)
	}

	time.Sleep(time.Millisecond) // Just for clarity, the test isn't sensitive to timing.

	if replaced := c.Replace(k, v2, cache.KeepTTL); !replaced {
		t.Fatal("Cache.Replace() = false, want true")
	}
	item2, found := c.Item(k)
	if !found {
		t.Fatalf("Cache.Item() = false (key %q not found), want true", k)
	}
	if item2.Value != v2 {
		t.Errorf("Cache.Replace() did not update value: got %q, want %q", item2.Value, v2)
	}
	if item2.Expiration != item1.Expiration {
		t.Errorf("Cache.Replace() did not keep exact TTL: got expiration %v, want %v", item2.Expiration, item1.Expiration)
	}
}
