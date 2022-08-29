package consistenthash

import (
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
)

type Hash func(data []byte) uint32

type Map struct {
	sync.Mutex
	// 哈希函数
	hash Hash
	// 虚拟节点倍数
	replicas int
	// 原子地存取 keys 和 hashMap
	values atomic.Value // values
}

type values struct {
	// 哈希环
	keys []int
	// 虚拟节点与真是节点的映射
	hashMap map[int]string
}

func NeeMap(replicas int, hashFunc Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     hashFunc,
	}
	m.values.Store(&values{
		hashMap: make(map[int]string),
	})
	return m
}

// 添加节点
func (m *Map) Add(keys ...string) {
	m.Lock()
	defer m.Unlock()
	newValues := m.loadValues()
	//newValues := m.copyValue()
	for _, key := range keys {
		// 对每个 key(节点) 创建 m.replicas 个虚拟节点
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			newValues.keys = append(newValues.keys, hash)
			newValues.hashMap[hash] = key
		}
	}
	sort.Ints(newValues.keys)
	m.values.Store(newValues)
}

func (m *Map) Get(key string) string {
	values := m.loadValues()
	if len(values.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	idx := sort.Search(len(values.keys), func(i int) bool {
		return values.keys[i] >= hash
	})
	// idx == len(m.keys), 说明应选择 m.keys[0],
	// 因为 m.keys 是一个环形结构，用取余数的方式来处理这种情况
	return values.hashMap[values.keys[idx%len(values.keys)]]
}

func (m *Map) Remove(key string) {
	m.Lock()
	defer m.Unlock()

}

func (m *Map) loadValues() *values {
	return m.values.Load().(*values)
}

func (m *Map) copyValue() *values {
	oldValues := m.loadValues()
	newValues := &values{
		keys:    make([]int, len(oldValues.keys)),
		hashMap: make(map[int]string),
	}
	copy(newValues.keys, oldValues.keys)
	for k, v := range oldValues.hashMap {
		newValues.hashMap[k] = v
	}
	return newValues
}
