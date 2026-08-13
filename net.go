package main

import (
	"context"
	"crypto/tls"
	"math"
	"net/http"
	"sync"
	"time"
)

type Latency struct {
	IP      IP
	Latency float64
}

func req_and_choose_good_and_get_latency(ips []IP) []Latency {
	// 监控器ctx用来通知协程停止请求，优选数量已达标
	ctx, cancel := context.WithCancel(context.Background())
	num := 10               // 优选ip数量
	var latencies []Latency // ip与延迟数组
	var n = 16              // 并发数

	results := make(chan Latency, 10) // 控制并发数的通道

	// 请求每个ip
	wg := sync.WaitGroup{}

	n = int(math.Min(float64(len(ips)), float64(n))) // 并发数不能超过ip数量

	client := newClient() // 创建一个共享的 HTTP 客户端
	for i := 0; i < n; i++ {
		wg.Add(1)
		start := i * len(ips) / n
		end := len(ips) * (i + 1) / n
		go func(ips []IP) {
			defer wg.Done()
			for _, ip := range ips {
				latency := request(ip, client)
				if latency < 0 {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case results <- Latency{IP: ip, Latency: latency}:
				}

			}
		}(ips[start:end])
	}
	// 监控发送者有没有发完并关闭通道
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		latencies = append(latencies, Latency{IP: r.IP, Latency: r.Latency})
		if len(latencies) >= num {
			// 不只要break，还要通知
			cancel()
			break
		}
	}
	return latencies
}

func request(ip IP, client *http.Client) float64 {

	start := time.Now()        // Start time
	var duration time.Duration // Variable to hold the duration
	req, _ := http.NewRequest(http.MethodHead, "https://"+ip.IP+"/cdn-cgi/trace", nil)

	req.Host = "cp.cloudflare.com" // 设置 Host 头为 Cloudflare 的域名

	if _, err := client.Do(req); err == nil {
		duration = time.Since(start) // Update end time on error
	} else {
		return -1
	}
	// Process the response to calculate latency
	return duration.Seconds() // Return latency in milliseconds
}

func newClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // 跳过证书校验
		DisableKeepAlives: true,                                  // 不池化, 用完断开
	}
	return &http.Client{
		Timeout:   500 * time.Millisecond, // 设置超时时间
		Transport: transport,
	}
}
