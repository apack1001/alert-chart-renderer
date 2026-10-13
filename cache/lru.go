package cache

import (
	"container/list"
	"sync"
	"time"
)

type cacheEntry struct {
	key       string
	value     []byte
	expiresAt time.Time
}

// LRUCache provides a thread-safe LRU cache with TTL expiration.
type LRUCache struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	evictList *list.List
}

// NewLRUCache initializes a cache with fixed capacity.
func NewLRUCache(capacity int) *LRUCache {
	if capacity <= 0 {
		capacity = 500
	}
	return &LRUCache{
		capacity:  capacity,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves an item by key if it exists and has not expired.
func (c *LRUCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, found := c.items[key]
	if !found {
		return nil, false
	}

	entry := elem.Value.(*cacheEntry)
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.removeElement(elem)
		return nil, false
	}

	c.evictList.MoveToFront(elem)
	return entry.value, true
}

// Set adds or updates an item with a given TTL.
func (c *LRUCache) Set(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	if elem, found := c.items[key]; found {
		c.evictList.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.value = value
		entry.expiresAt = expiresAt
		return
	}

	if c.evictList.Len() >= c.capacity {
		c.removeOldest()
	}

	entry := &cacheEntry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	elem := c.evictList.PushFront(entry)
	c.items[key] = elem
}

// Len returns current item count.
func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictList.Len()
}

func (c *LRUCache) removeElement(elem *list.Element) {
	c.evictList.Remove(elem)
	entry := elem.Value.(*cacheEntry)
	delete(c.items, entry.key)
}

func (c *LRUCache) removeOldest() {
	elem := c.evictList.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}
