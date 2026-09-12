package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/LennyFace24/CFip-go/src/config"
)

type Latency struct {
	IP      IP
	Latency float64
}

// ProbeResult 单次探测结果。Latency 单位为秒，小于 0 表示失败。
type ProbeResult struct {
	Latency float64
	Colo    string // 机房代码，取自 cf-ray；空串表示未取到
}

// 测速目标：明文 HTTP + HEAD，只取响应头，不下载正文。
//
// 用 HTTP 而非 HTTPS 是为了省掉 TLS 握手，单次探测成本约三分之一；
// 排序只关心链路质量，两种协议的排序结果一致。改 probeScheme 即可切回 https。
//
// 注意 Cloudflare 对 /cdn-cgi/trace 的 HEAD 返回 404，属于预期行为：
// 我们只取响应头里的 cf-ray 与往返耗时，不校验状态码。
const (
	probeScheme   = "http"
	probeHostname = "cp.cloudflare.com"
	probePath     = "/cdn-cgi/trace"
	// 伪装成浏览器，避免被 Cloudflare 的 bot 规则拦下
	probeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// RequestAndChooseGoodAndGetLatency 探测全部 IP，返回「达标」的延迟列表。
//
// 达标 = 探测成功且延迟不超过 cfg.Latency（毫秒）；
// 请求失败与延迟超标都不计入。集满 cfg.Number 条达标结果即提前停止。
// 并发调度与 StreamLatency 共用一套实现。
func RequestAndChooseGoodAndGetLatency(ips []IP) []Latency {
	// 监控器ctx用来通知协程停止请求，达标数量已足够
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // 保底释放，避免 context 泄漏

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println("加载配置文件出错:", err)
		panic(err)
	}
	limitSec := float64(cfg.Latency) / 1000

	var latencies []Latency
	for r := range StreamLatency(ctx, ips, cfg.Concurrency, nil) {
		if r.Latency < 0 || r.Latency > limitSec {
			continue
		}
		latencies = append(latencies, Latency{IP: r.IP, Latency: r.Latency})
		if len(latencies) >= cfg.Number {
			cancel()
			break
		}
	}
	return latencies
}

// request 探测单个 IP，返回耗时（秒）与机房代码；失败时 Latency 为 -1。
func request(ip IP, client *http.Client) ProbeResult {
	req, err := http.NewRequest(http.MethodHead, probeScheme+"://"+ip.IP+probePath, nil)
	if err != nil {
		return ProbeResult{Latency: -1}
	}
	// Host 决定 Cloudflare 用哪个站点来响应，连接地址仍是 ip 本身
	req.Host = probeHostname
	req.Header.Set("User-Agent", probeUserAgent)

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return ProbeResult{Latency: -1}
	}
	defer resp.Body.Close()

	return ProbeResult{
		Latency: time.Since(start).Seconds(),
		Colo:    ParseColo(resp.Header.Get("cf-ray")),
	}
}

func newClient() *http.Client {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // 切回 https 时跳过证书校验
		DisableKeepAlives: true,                                  // 不池化, 用完断开
	}
	return &http.Client{
		Timeout:   time.Duration(cfg.Timeout) * time.Millisecond, // 设置超时时间
		Transport: transport,
	}
}
