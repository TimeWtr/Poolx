// Copyright 2025 TimeWtr
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package poolx

import (
	"sync"
	"sync/atomic"
)

type Options[T any] func(p *Pool[T])

// WithResetFn 配置对象的重置方法，用于对象不再使用时放回Pool之间对复杂对象进行重置。
func WithResetFn[T any](resetFn func(T) T) Options[T] {
	return func(o *Pool[T]) {
		o.resetFn = resetFn
	}
}

// WithCloseFn 配置对象的关闭方法，用于对象池结束使用时，对资源的释放操作，
// 比如对象是channel，CloseFn就应该Close()通道
func WithCloseFn[T any](closeFn func(T)) Options[T] {
	return func(o *Pool[T]) {
		o.closeFn = closeFn
	}
}

// WithMetrics 开启指标数据监控
func WithMetrics[T any]() Options[T] {
	return func(o *Pool[T]) {
		o.enableMetrics = true
	}
}

// Metrics 监控数据
type Metrics struct {
	allocations atomic.Int64 // 总共分配的对象计数
	discards    atomic.Int64 // 因为对象池达到最大容量而被丢弃的对象数量
	totalGets   atomic.Int64 // 总共获取的对象数量
}

type Pool[T any] struct {
	p              atomic.Pointer[[PoolCount]*sync.Pool] // 以官方包的pool为基础封装
	capacity       atomic.Int32                          // 池的容量限制
	currentCounter atomic.Int32                          // 当前活跃的对象数量，即被从Pool中取出还未放回的对象
	newFn          func() T                              // 初始化的对象的方法
	resetFn        func(T) T                             // 重置对象的方法
	closeFn        func(T)                               // 关闭对象(释放资源)的方法
	enableMetrics  bool                                  // 是否开启监控，默认不开启
	metrics        Metrics                               // 指标数据
	status         atomic.Int32                          // 标识Pool状态
}

func NewPool[T any](capacity int32, newFn func() T, opts ...Options[T]) (*Pool[T], error) {
	p := &Pool[T]{
		metrics: Metrics{},
		newFn:   newFn,
		status:  atomic.Int32{},
	}

	for _, opt := range opts {
		opt(p)
	}

	p.status.Store(Running)
	var pools [4]*sync.Pool
	for i := range pools {
		pools[i] = &sync.Pool{
			New: func() interface{} {
				t := newFn()
				if p.enableMetrics {
					p.metrics.allocations.Add(1)
				}

				return t
			},
		}

		// 冷启动时预分配容量固定比例的对象数量，减少首次请求时的延迟抖动
		// 1KB和2KB的缓冲池预热30%比例的对象
		var scale float64
		if i <= _2KBIndex {
			scale = 0.3
		} else if i == _4KBIndex {
			// 4KB的缓冲池预热15%比例的对象
			scale = 0.15
		} else {
			// 超过KB以上的缓冲池不进行预热处理
			continue
		}
		preloadSize := float64(capacity) * scale
		for j := 0; j < int(preloadSize); j++ {
			obj, _ := pools[i].Get().(T)
			pools[i].Put(obj)
		}
	}
	p.p.Store(&pools)
	p.capacity.Store(capacity)

	return p, nil
}

func (p *Pool[T]) Get(size int64) (t T, err error) {
	idx := p.selector(size)
	for {
		if p.status.Load() == Closed {
			return t, ErrPoolClosed
		}

		currentCounter := p.currentCounter.Load()
		if currentCounter >= p.capacity.Load() {
			return t, ErrCountOverCapacity
		}

		if p.currentCounter.CompareAndSwap(currentCounter, currentCounter+1) {
			// 再次检查池状态（防止关闭后操作）
			if p.status.Load() == Closed {
				p.currentCounter.Add(-1)
				return t, ErrPoolClosed
			}
			break
		}
	}

	t, ok := p.p.Load()[idx].Get().(T)
	if !ok {
		p.currentCounter.Add(-1)
		return t, ErrObjectType
	}

	p.metrics.totalGets.Add(1)
	return t, nil
}

func (p *Pool[T]) Put(t T, size int64) {
	idx := p.selector(size)
	p.currentCounter.Add(-1)
	if p.status.Load() == Closed {
		if p.closeFn != nil {
			p.closeFn(t)
		}

		return
	}

	currentCounter := p.currentCounter.Load()
	if currentCounter >= p.capacity.Load() {
		// 池中对象已经达到最大限制，直接丢弃对象
		if p.enableMetrics {
			p.metrics.discards.Add(1)
		}

		if p.closeFn != nil {
			p.closeFn(t)
		}

		return
	}

	if p.resetFn != nil {
		p.resetFn(t)
	}

	p.p.Load()[idx].Put(t)
}

func (p *Pool[T]) Close() {
	if !p.status.CompareAndSwap(Running, Closed) {
		return
	}

	var pools [PoolCount]*sync.Pool
	p.p.Store(&pools)
}

// selector 根据对象的大小确定写入哪个缓冲池，返回的是缓冲池列表的下标
func (p *Pool[T]) selector(size int64) (idx int) {
	switch {
	case size < _1KB:
		idx = 0
	case size < _2KB:
		idx = 1
	case size < _4KB:
		idx = 2
	default:
		idx = 3
	}

	return
}

// Stats 返回监控指标数据，allocations分配的对象总数，reuses复用对象总数，
// discards因超过最大容量而直接丢弃的对象总数
func (p *Pool[T]) Stats() (allocations, reuses, discards int64) {
	t := p.metrics.totalGets.Load()
	a := p.metrics.allocations.Load()
	d := p.metrics.discards.Load()
	return a, t - a, d
}

// DynamicCapacity 动态调整容量限制
func (p *Pool[T]) DynamicCapacity(capacity int32) bool {
	oldCapacity := p.capacity.Load()
	return p.capacity.CompareAndSwap(oldCapacity, capacity)
}
