package core

import (
	"context"
	"math"
	"sync"
)

// StreamResult 流式测速的单条结果；Latency 为 -1 表示失败
type StreamResult struct {
	IP      IP
	Latency float64
	Source  string // "单IP" 或 "网段采样"
}

// ProbeLatency 探测函数签名，便于测试注入
type ProbeLatency func(ip IP) float64

// StreamLatency 并发探测 ips 中的每个 IP，逐条向返回的 channel 发送结果。
// ctx 取消时停止发送并关闭 channel。probe 为 nil 时使用默认 HTTP 探测。
func StreamLatency(ctx context.Context, ips []IP, concurrency int, probe ProbeLatency) <-chan StreamResult {
	if probe == nil {
		client := newClient()
		probe = func(ip IP) float64 {
			return request(ip, client)
		}
	}
	results := make(chan StreamResult, 10)
	n := int(math.Min(float64(len(ips)), float64(concurrency)))
	if n <= 0 {
		close(results)
		return results
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		start := i * len(ips) / n
		end := len(ips) * (i + 1) / n
		go func(ips []IP) {
			defer wg.Done()
			for _, ip := range ips {
				if ctx.Err() != nil {
					return // 已取消，不再发起新探测
				}
				source := "单IP"
				if ip.isCIDR {
					source = "网段采样"
				}
				r := StreamResult{IP: ip, Latency: probe(ip), Source: source}
				select {
				case <-ctx.Done():
					return
				case results <- r:
				}
			}
		}(ips[start:end])
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	return results
}
