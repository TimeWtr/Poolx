# Poolx
基于Go的sync.Pool进行二次封装，是sync.Pool的增强型。

## Features
- 范型支持；
- 缓存预热，初始化时预创建最大数量的30%对象放入Pool中；
- 引用计数统计(活跃对象)；
- 增强监控能力，统计对象分配总数、对象复用总数、对象丢弃总数；
- 对象池容量控制，防止无限增长；
- 对象池容量动态扩缩容(待实现)；

## 🚀 Performance
### 1. 结构体嵌套Channel的对象
- **硬件平台**：Apple M4芯片（ARM64架构）
- **软件环境**：Go 1.23
- **测试参数**：单核运行/5轮基准测试取平均
- **测试命令**：`go test -bench="^BenchmarkPool_Struct$" -cpu=1 -count=5 -benchmem`
- **复用对象**: 
```go
 type Instance struct {
   Schema string
   Addr   string
   Port   int
   Ch     chan struct{}
 }
```
#### 📊 测试结果
| 迭代轮次   | 迭代次数       | 单次耗时 (ns/op) | 内存分配 (B/op) | 分配次数 (allocs/op) |
|--------|---------------|------------------|----------------|---------------------|
| 1      | 9,956,271     | 116.9            | 48             | 1                   |
 | 2      | 10,571,440    | 116.0            | 48             | 1                   |
 | 3      | 10,378,336    | 116.5            | 48             | 1                   |
 | 4      | 10,328,274    | 117.3            | 48             | 1                   |
 |      5 | 10,304,106    | 117.1            | 48             | 1                   |
> 💡 测试数据显示单核场景下稳定吞吐量达近​1000万次/秒

### 2. 1MB大小的bytes对象
- **硬件平台**：Apple M4芯片（ARM64架构）
- **软件环境**：Go 1.23
- **测试参数**：单核运行/5轮基准测试取平均
- **测试命令**：`go test -bench="^BenchmarkPool_Big_Bytes$" -cpu=1 -count=5 -benchmem`

#### 📊 测试结果
| 测试轮次 | 迭代次数      | 单次耗时 (ns/op) | 内存分配 (B/op) | 分配次数 (allocs/op) |
|----------|---------------|------------------|-----------------|---------------------|
| 1        | 31,847,030    | 37.85            | 0               | 0                   |
| 2        | 31,781,343    | 37.78            | 0               | 0                   |
| 3        | 31,750,651    | 37.92            | 0               | 0                   |
| 4        | 31,526,954    | 37.85            | 0               | 0                   |
| 5        | 31,675,222    | 37.92            | 0               | 0                   |
> 💡 测试数据显示单核场景下稳定吞吐量达近​3100多万次/秒

## Installation
```bash
go get github.com/TimeWtr/Poolx
```
## Usage
```go
package main

import (
	"time"

	"github.com/TimeWtr/Poolx"
)

func main() {
	pool, err := poolx.NewPool[chan []byte](
		10,
		func() chan []byte {
			return make(chan []byte, 20)
		},
		poolx.WithResetFn[chan []byte](func(ch chan []byte) chan []byte {
			for {
				select {
				case <-ch:
				default:
					return ch
				}
			}
		}),
		poolx.WithCloseFn[chan []byte](func(ch chan []byte) {
			for {
				select {
				case <-ch:
				default:
					close(ch)
					return
				}
			}
		}), poolx.WithMetrics[chan []byte]())
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	obj, err := pool.Get()
	if err != nil {
		panic(err)
	}
	time.Sleep(1 * time.Second)
	pool.Put(obj)
}

```