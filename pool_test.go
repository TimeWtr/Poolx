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
	"bytes"
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/semaphore"
)

func TestNewPool_String(t *testing.T) {
	p, err := NewPool[string](10, func() string {
		return ""
	}, WithMetrics[string]())
	assert.NoError(t, err)
	assert.NotNil(t, p)
	defer p.Close()

	sem := semaphore.NewWeighted(100)
	const total = 1000
	size := int64(500)
	for i := 0; i < total; i++ {
		err := sem.Acquire(context.Background(), 1)
		assert.NoError(t, err)
		go func() {
			defer sem.Release(1)
			obj, err := p.Get(size)
			assert.NoError(t, err)
			p.Put(obj, size)
		}()
	}

	time.Sleep(time.Second)
	a, r, _ := p.Stats()
	if p.metrics.totalGets.Load() != total {
		t.Fatalf("totalGets计数错误，期望%d，实际%d", total, p.metrics.totalGets.Load())
	}
	if a+r != total {
		t.Fatalf("统计不匹配: allocations(%d) + reuses(%d) != total(%d)", a, r, total)
	}
	t.Logf("totalGets计数: %d, allocations计数：%d, discards: %d, reused: %d",
		p.metrics.totalGets.Load(),
		p.metrics.allocations.Load(),
		p.metrics.discards.Load(),
		r)
}

func TestNewPool_Chan_Bytes(t *testing.T) {
	p, err := NewPool[chan []byte](10,
		func() chan []byte {
			return make(chan []byte, 100)
		},
		WithResetFn[chan []byte](func(c chan []byte) chan []byte {
			for len(c) > 0 {
				<-c
			}
			return c
		}),
		WithCloseFn[chan []byte](func(c chan []byte) {
			for len(c) > 0 {
				<-c
			}
			close(c)
		}),
		WithMetrics[chan []byte]())
	assert.NoError(t, err)
	assert.NotNil(t, p)
	defer p.Close()

	sem := semaphore.NewWeighted(100)
	const total = 1000
	size := int64(500)
	for i := 0; i < total; i++ {
		err := sem.Acquire(context.Background(), 1)
		assert.NoError(t, err)
		go func() {
			defer sem.Release(1)
			obj, err := p.Get(size)
			assert.NoError(t, err)
			p.Put(obj, size)
		}()
	}

	time.Sleep(time.Second)
	a, r, _ := p.Stats()
	if p.metrics.totalGets.Load() != total {
		t.Fatalf("totalGets计数错误，期望%d，实际%d", total, p.metrics.totalGets.Load())
	}
	if a+r != total {
		t.Fatalf("统计不匹配: allocations(%d) + reuses(%d) != total(%d)", a, r, total)
	}
	t.Logf("totalGets计数: %d, allocations计数：%d, discards: %d, reused: %d",
		p.metrics.totalGets.Load(),
		p.metrics.allocations.Load(),
		p.metrics.discards.Load(),
		r)
}

func TestNewPool_Struct(t *testing.T) {
	type Instance struct {
		Schema string        `json:"schema"`
		Addr   string        `json:"addr"`
		Port   int           `json:"port"`
		Ch     chan struct{} `json:"ch"`
	}

	p, err := NewPool[Instance](10,
		func() Instance {
			return Instance{
				Schema: "",
				Addr:   "",
				Port:   0,
				Ch:     make(chan struct{}, 10),
			}
		},
		WithResetFn[Instance](func(c Instance) Instance {
			c.Schema = ""
			c.Addr = ""
			c.Port = 0
			for {
				select {
				case <-c.Ch:
				default:
					return c
				}
			}
		}),
		WithCloseFn[Instance](func(i Instance) {
			for {
				select {
				case <-i.Ch:
				default:
					close(i.Ch)
					return
				}
			}
		}),
		WithMetrics[Instance]())
	assert.NoError(t, err)
	assert.NotNil(t, p)
	defer p.Close()

	sem := semaphore.NewWeighted(100)
	const total = 1000
	size := int64(1024 * 10)
	for i := 0; i < total; i++ {
		err := sem.Acquire(context.Background(), 1)
		assert.NoError(t, err)
		go func() {
			defer sem.Release(1)
			obj, err := p.Get(size)
			assert.NoError(t, err)
			p.Put(obj, size)
		}()
	}

	time.Sleep(time.Second)
	a, r, _ := p.Stats()
	if p.metrics.totalGets.Load() != total {
		t.Fatalf("totalGets计数错误，期望%d，实际%d", total, p.metrics.totalGets.Load())
	}
	if a+r != total {
		t.Fatalf("统计不匹配: allocations(%d) + reuses(%d) != total(%d)", a, r, total)
	}
	t.Logf("totalGets计数: %d, allocations计数：%d, discards: %d, reused: %d",
		p.metrics.totalGets.Load(),
		p.metrics.allocations.Load(),
		p.metrics.discards.Load(),
		r)
}

func TestNewPool_Struct_Multi_Size(t *testing.T) {
	type Instance struct {
		Schema string        `json:"schema"`
		Addr   string        `json:"addr"`
		Port   int           `json:"port"`
		Ch     chan struct{} `json:"ch"`
	}

	p, err := NewPool[Instance](10,
		func() Instance {
			return Instance{
				Schema: "",
				Addr:   "",
				Port:   0,
				Ch:     make(chan struct{}, 10),
			}
		},
		WithResetFn[Instance](func(c Instance) Instance {
			c.Schema = ""
			c.Addr = ""
			c.Port = 0
			for {
				select {
				case <-c.Ch:
				default:
					return c
				}
			}
		}),
		WithCloseFn[Instance](func(i Instance) {
			for {
				select {
				case <-i.Ch:
				default:
					close(i.Ch)
					return
				}
			}
		}),
		WithMetrics[Instance]())
	assert.NoError(t, err)
	assert.NotNil(t, p)
	defer p.Close()

	sem := semaphore.NewWeighted(100)
	const total = 1000
	for i := 0; i < total; i++ {
		err := sem.Acquire(context.Background(), 1)
		assert.NoError(t, err)
		go func() {
			defer sem.Release(1)
			size := rand.Int63n(1024 * 4)
			obj, err := p.Get(size)
			assert.NoError(t, err)
			p.Put(obj, size)
		}()
	}

	time.Sleep(time.Second)
	a, r, _ := p.Stats()
	if p.metrics.totalGets.Load() != total {
		t.Fatalf("totalGets计数错误，期望%d，实际%d", total, p.metrics.totalGets.Load())
	}
	if a+r != total {
		t.Fatalf("统计不匹配: allocations(%d) + reuses(%d) != total(%d)", a, r, total)
	}
	t.Logf("totalGets计数: %d, allocations计数：%d, discards: %d, reused: %d",
		p.metrics.totalGets.Load(),
		p.metrics.allocations.Load(),
		p.metrics.discards.Load(),
		r)
}

func BenchmarkPool_Struct(b *testing.B) {
	type Instance struct {
		Schema string
		Addr   string
		Port   int
		Ch     chan struct{}
	}

	// 初始化对象池（避免计时器包含初始化耗时）
	p, _ := NewPool[Instance](10,
		func() Instance {
			return Instance{
				Schema: "",
				Addr:   "",
				Port:   0,
			}
		},
		WithResetFn[Instance](func(c Instance) Instance {
			c.Schema = ""
			c.Addr = ""
			c.Port = 0

			for {
				select {
				case <-c.Ch:
				default:
					return c
				}
			}
		}),
		WithCloseFn[Instance](func(i Instance) {
			i.Schema = ""
			i.Addr = ""
			i.Port = 0

			for {
				select {
				case <-i.Ch:
				default:
					close(i.Ch)
					i.Ch = nil
					return
				}
			}
		}),
	)
	defer p.Close()

	b.ResetTimer()
	size := int64(1024 * 10)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj, _ := p.Get(size)
			p.Put(obj, size)
		}
	})
}

func BenchmarkPool_Big_Bytes(b *testing.B) {
	const size = 1 << 20 // 1MB
	p, _ := NewPool[*[]byte](10,
		func() *[]byte {
			buf := make([]byte, 0, size)
			return &buf
		},
		WithResetFn[*[]byte](func(c *[]byte) *[]byte {
			*c = (*c)[:0]
			return c
		}),
	)
	defer p.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj, _ := p.Get(size)
			p.Put(obj, size)
		}
	})
}

func BenchmarkPool_Bytes_Buffer(b *testing.B) {
	p, _ := NewPool[*bytes.Buffer](10,
		func() *bytes.Buffer {
			return bytes.NewBuffer([]byte{})
		},
		WithResetFn[*bytes.Buffer](func(c *bytes.Buffer) *bytes.Buffer {
			c.Reset()
			return c
		}),
	)
	defer p.Close()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		size := rand.Int63n(1024 * 4)
		for pb.Next() {
			obj, _ := p.Get(size)
			obj.WriteString("pool and test data!")
			p.Put(obj, size)
		}
	})
}

func BenchmarkWithoutPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf := &bytes.Buffer{}
		buf.WriteString("test")
	}
}

var pool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}

func BenchmarkWithPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf, _ := pool.Get().(*bytes.Buffer)
		buf.WriteString("test")
		buf.Reset()
		pool.Put(buf)
	}
}
