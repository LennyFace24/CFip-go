package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/LennyFace24/CFip-go/src/config"
	"github.com/LennyFace24/CFip-go/src/core"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SpeedResult 单条测速结果。Latency 单位为毫秒，小于 0 表示失败。
type SpeedResult struct {
	Seq     int     // 自增序号，前端用它做 :key
	IP      string
	Latency float64
	Source  string
}

// SpeedSummary 一次测速结束后的汇总。
type SpeedSummary struct {
	Total    int   // 已探测数量
	Success  int
	Failed   int
	Elapsed  int64 // 耗时（毫秒）
	Stopped  bool  // true = 提前结束（集满或手动停止），false = 全部跑完
}

// SpeedService 负责流式测速，结果通过事件逐条推给前端。
type SpeedService struct {
	app    *application.App
	mu     sync.Mutex
	cancel context.CancelFunc
}

func (s *SpeedService) ServiceName() string { return "SpeedService" }

// Start 解析 IP 文本并开始测速。已在测速时返回错误。
func (s *SpeedService) Start(text string) error {
	ips, err := core.NewIPParser().ParseIP(text)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return errors.New("没有可测速的 IP，请先导入 ip.txt 或加载内置网段")
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return errors.New("测速正在进行中")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()

	go s.run(ctx, cancel, ips, cfg)
	return nil
}

// Stop 手动停止测速。
func (s *SpeedService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

// IsRunning 报告测速是否正在进行。
func (s *SpeedService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancel != nil
}

func (s *SpeedService) run(ctx context.Context, cancel context.CancelFunc, ips []core.IP, cfg *config.Config) {
	defer cancel()
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
	}()

	start := time.Now()
	seq, success, failed := 0, 0, 0
	stopped := false

	for r := range core.StreamLatency(ctx, ips, cfg.Concurrency, nil) {
		seq++
		if r.Latency >= 0 {
			success++
		} else {
			failed++
		}
		s.emit("speed:result", SpeedResult{
			Seq:     seq,
			IP:      r.IP.IP,
			Latency: r.Latency * 1000, // 秒 → 毫秒
			Source:  r.Source,
		})
		if success >= cfg.Number {
			stopped = true
			break
		}
	}

	s.emit("speed:done", SpeedSummary{
		Total:   seq,
		Success: success,
		Failed:  failed,
		Elapsed: time.Since(start).Milliseconds(),
		Stopped: stopped,
	})
}

func (s *SpeedService) emit(name string, data any) {
	if s.app == nil {
		return
	}
	s.app.Event.Emit(name, data)
}
