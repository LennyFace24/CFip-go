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

// SpeedResult 单条测速结果。Latency 单位为毫秒，小于 0 表示请求失败。
type SpeedResult struct {
	Seq     int     // 自增序号，前端用它做 :key
	IP      string
	Latency float64
	Source  string
}

// SpeedSummary 一次测速结束后的汇总。
type SpeedSummary struct {
	Total     int   // 已探测数量
	Qualified int   // 达标：探测成功且延迟不超过上限
	OverLimit int   // 超标：探测成功但延迟超过上限
	Failed    int   // 请求失败
	Elapsed   int64 // 耗时（毫秒）
	Stopped   bool  // true = 提前结束（集满或手动停止）
	LogPath   string
}

// SpeedService 负责流式测速，结果通过事件逐条推给前端。
// 停止条件：达标数量达到 cfg.Number。
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
	limitMs := float64(cfg.Latency)
	seq, qualified, overLimit, failed := 0, 0, 0, 0
	stopped := false
	collected := make([]SpeedResult, 0, len(ips))

	for r := range core.StreamLatency(ctx, ips, cfg.Concurrency, nil) {
		seq++
		ms := r.Latency * 1000 // 秒 → 毫秒
		switch {
		case r.Latency < 0:
			failed++
		case ms <= limitMs:
			qualified++
		default:
			overLimit++
		}

		result := SpeedResult{Seq: seq, IP: r.IP.IP, Latency: ms, Source: r.Source}
		collected = append(collected, result)
		s.emit("speed:result", result)

		if qualified >= cfg.Number {
			stopped = true
			break
		}
	}

	summary := SpeedSummary{
		Total:     seq,
		Qualified: qualified,
		OverLimit: overLimit,
		Failed:    failed,
		Elapsed:   time.Since(start).Milliseconds(),
		Stopped:   stopped,
	}

	// 无论正常结束还是被停止，都留一份日志便于回溯（固定文件名，覆盖上一次）
	if path, err := WriteSpeedLog(collected, summary, cfg); err == nil {
		summary.LogPath = path
	}

	s.emit("speed:done", summary)
}

func (s *SpeedService) emit(name string, data any) {
	if s.app == nil {
		return
	}
	s.app.Event.Emit(name, data)
}
