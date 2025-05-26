# Poolx
基于Go的sync.Pool进行二次封装，是sync.Pool的增强型。

## Features
- 范型支持；
- 对象大小分级存储，多级别Pool，1KB、2KB、4KB、>4KB，不同的Size对应
各自的Pool中，减少内存碎片及写入冲突；
- 多级缓存预热，初始化时1KB、2KB的缓冲池以30%比例的对象数量进行缓存预热处理，
4KB的缓冲池以15%比例的对象数量进行缓存预热处理，超过4KB的缓冲池不进行对象预热
处理，防止对象内存泄漏问题；
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

### 3. 基于bytes.Buffer的对照组压测
- **硬件平台**：Apple M4芯片（ARM64架构）
- **软件环境**：Go 1.23
- **测试参数**：单核运行/5轮基准测试取平均
#### 3.1 不使用Pool的数据压测
| 迭代次数    | 单次耗时 (ns/op) | 内存分配 (B/op) | 分配次数 (allocs/op) |
|-------------|------------------|-----------------|---------------------|
| 74,381,559  | 15.99            | 64              | 1                   |

#### 3.2 使用原生sync.Pool的数据压测
| 迭代次数      | 单次耗时 (ns/op) | 内存分配 (B/op) | 分配次数 (allocs/op) |
|---------------|------------------|-----------------|---------------------|
| 169,577,250   | 6.864            | 0               | 0                   |

#### 3.3 使用增强型的Poolx的数据压测
| 迭代次数        | 单次耗时 (ns/op) | 内存分配 (B/op) | 分配次数 (allocs/op) |
|-------------|--------------|-----------------|---------------------|
|  117731210 | 10.02        | 0               | 0                   |
#### 性能结论
1. **高效内存复用**  
   通过对象池技术实现：
 - **100% 内存复用率**：`0 B/op` + `0 allocs/op`
 - **GC 压力消除**：相比非池化方案（64 B/op），完全避免垃圾回收压力

2. **稳定低延迟**
 - 对象池获取/归还操作的确定性
 - 封装层未引入显著性能损耗

3. **吞吐量表现**  
   平均每秒处理 **1.17 亿次操作**，适用于：
 - 高频网络数据包处理（如 HTTP 请求解析）
 - 实时日志处理系统
 - 内存数据库操作

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
		// Pool允许的最大对象数量
		10,  
		// New对象方法，必须
		func() chan []byte { 
			return make(chan []byte, 20)
		},
		// 对象的重置方法，用于放回Pool前进行重置，可选
		poolx.WithResetFn[chan []byte](func(ch chan []byte) chan []byte {
			for {
				select {
				case <-ch:
				default:
					return ch
				}
			}
		}),
		// 对象的关闭方法，用于在Pool关闭前/对象丢弃前的资源释放方法，可选
		poolx.WithCloseFn[chan []byte](func(ch chan []byte) {
			for {
				select {
				case <-ch:
				default:
					close(ch)
					return
				}
			}
		}), 
		// 开启指标监控，可选
		poolx.WithMetrics[chan []byte]())
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	
	size := int64(1024*10)
	obj, err := pool.Get(size)
	if err != nil {
		panic(err)
	}
	time.Sleep(1 * time.Second)
	pool.Put(obj, size)
}

```