package lru

import "container/list"

// 约定front 是队尾

// Cache is LRU cache. It is not safe for concurrent access.
type Cache struct {
	maxBytes int64 // 允许使用的最大内存
	nbytes   int64 // 当前已使用的内存
	ll       *list.List
	cache    map[string]*list.Element
	// optional and executed when an entry is purged
	OnEvicted func(key string, value Value)
}

type entry struct {
	key   string
	value Value
}

// Value use Len to count how many bytes it takes
type Value interface {
	Len() int
}

// New is the constructor of cache
func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

// Get look ups a key`s value
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

// RemoveOldest removes the oldest item
func (c *Cache) RemoveOldest() {
	// c.ll.Back() 取到队首节点，从链表中删除
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		// 从字典中c.cache 删除该机诶单的影射关系
		delete(c.cache, kv.key)
		// 更新当前所用的内存c.nbytes
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		// 如果回调函数OnEvicted 不为nil，则调用回调函数
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Add adds a value to the cache.
func (c *Cache) Add(key string, value Value) {
	// 键存在，更新对应节点的值，并将该节点移到队尾部
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else { // 不存在，添加新节点，并字典中添加key和节点的映射关系
		ele := c.ll.PushFront(&entry{key: key, value: value})
		c.cache[key] = ele
		// 更新c.nbytes 超过最大值 c.maxBytes，则移除最小访问的节点
		c.nbytes += int64(len(key)) + int64(value.Len())
	}

	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}

// Len the number of cache entries
func (c *Cache) Len() int {
	return c.ll.Len()
}
