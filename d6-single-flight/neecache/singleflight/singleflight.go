package singleflight

import (
	"sync"
)

// call 代表正在进行中，或已结束的请求。使用 sync.WaitGroup 锁避免重入
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// Group 是 singleflight 的主数据结构，管理不通 key 的请求(call)
type Group struct {
	mu sync.Mutex // protects m
	m  map[string]*call
}

// Do 接受两个参数，第一个参数是key， 第二个参数是一个甘薯fn,Do的所用就是，针对相同的key
// 无论Do被调用多少次，函数 fn  都只会被调用一次，等待 fn 调用结束了，返回返回值或错误
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	// 延迟加载
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock() // 0⃣ 非 fn 请求 在这段时间尽量都走到这儿
		c.wg.Wait()   // 1⃣ 如果 fn 请求正在进行中，则等待
		return c.val, c.err
	}
	c := new(call)
	c.wg.Add(1)  // 发起请求前加锁
	g.m[key] = c // 绑定c
	g.mu.Unlock()

	c.val, c.err = fn() // 在fn 运行完之前，其他协程 都能执行到
	c.wg.Done()         // 请求结束，此时值已经存完了，其他协程抢占成功后 抵达 1⃣ 阻塞

	g.mu.Lock()
	delete(g.m, key) // 更新 g.m
	g.mu.Unlock()

	return c.val, c.err // 返回结果
}
