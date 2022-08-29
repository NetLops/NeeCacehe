package neecache

import (
	"fmt"
	"log"
	"neecache/singleflight"
	"sync"
)

/**
如何从源头获取数据，应该是用户决定的事情
设计了一个回调函数(callback)，在缓存不存在时，调用这个函数，得到源数据
*/

// A Getter loads data for a key.
type Getter interface {
	Get(key string) ([]byte, error)
}

// A GetterFunc implements Getter with a function
type GetterFunc func(key string) ([]byte, error)

// Get implements Getter interface function
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// A Group is a cache namespace and associated data loaded spread over
// 可以认为是一个缓存的命名空间，拥有唯一的名称name
type Group struct {
	name      string
	getter    Getter // 缓存未命中时获取源数据的回调(callback)
	mainCache cache  // 实现并发缓存
	peers     PeerPicker
	// use singleflight.Group to make sure that
	// each key is only fetched once
	loader *singleflight.Group
}

// RegisterPeers register a PeerPicker for choosing remote peer
// 将实现了 PeerPicker 接口的HTTPPool 注入到 Group中
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// NewGroup create a new instance of Group
// 用来实例化Group，并且将group存储在全局变量groups中
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:   name,
		getter: getter,
		mainCache: cache{
			cacheBytes: cacheBytes,
		},
		loader: &singleflight.Group{},
	}
	groups[name] = g
	return g
}

// GetGroup returns the named group previously created with NewGroup
// or nil if there`s no such group.
// 用来特定名称的Group，这里使用了只读锁RLock()，因为不涉及任何冲突变量的写操作
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

// Get value for a key from cache
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	// 从mainCache 中查找缓存，如果存在则返回缓存值
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[NeeCache] hit")
		return v, nil
	}
	// 缓存不存在调用load，load调用getLocally(分布式场景下会调用getFromPeer从
	// 其他节点获取)，getLocally调用用户回调函数g.getter.Get() 获取源数据，并且将源数据
	// 添加到缓存mainCache中（通过 populateCache 方法）
	return g.load(key)
}

// 使用 PickPeer() 方法选择节点，若非本地节点，调用getFromPeer() 从远程获取，
// 若是本机节点或失败，则回退到 getLocally
func (g *Group) load(key string) (value ByteView, err error) {
	// each key is only fetched once (either locally or remotely)
	// regardless of the number of concurrent callers
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[NeeCache] Failed to get from peer", err)
			}
		}

		return g.getLocally(key)
	})

	//if g.peers != nil {
	//	if peer, ok := g.peers.PickPeer(key); ok {
	//		if value, err = g.getFromPeer(peer, key); err == nil {
	//			return value, nil
	//		}
	//		log.Println("[neeCache Failed ti get from peer]", err)
	//	}
	//}
	//// 用于定义的源数据中取
	//return g.getLocally(key)
	if err == nil {
		return viewi.(ByteView), nil
	}
	return
}

func (g *Group) getLocally(key string) (ByteView, error) {
	// 从用户定义的源数据中取
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

// 实现了PeerGetter接口的httpGetter 从访问远程节点，获取缓存值
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, nil
	}
	return ByteView{
		b: bytes,
	}, nil
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
