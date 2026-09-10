package core

import (
	"context"
	"testing"
	"time"
)

type probeSpec struct {
	failFirst int     // 前几次失败
	latency   float64 // 其余次数的延迟（秒）
	colo      string
}

// healthProbe 按 IP 返回预设的采样序列
func healthProbe(spec map[string]probeSpec) ProbeLatency {
	calls := make(map[string]int)
	return func(ip IP) ProbeResult {
		calls[ip.IP]++
		s, ok := spec[ip.IP]
		if !ok || calls[ip.IP] <= s.failFirst {
			return ProbeResult{Latency: -1}
		}
		return ProbeResult{Latency: s.latency, Colo: s.colo}
	}
}

func healthConfig() HealthConfig {
	return HealthConfig{
		Interval:        time.Second,
		Times:           4,
		Gap:             0,
		LossLimit:       0.1,
		LatencyLimit:    0.3,
		FailStreakLimit: 2,
	}
}

func TestHealthCheckKeepsHealthyNode(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")

	checker := NewHealthChecker(pool, healthConfig(), healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.1, colo: "HKG"},
	}))

	if evicted := checker.CheckOnce(context.Background()); len(evicted) != 0 {
		t.Fatalf("健康节点不应被淘汰, got %v", evicted)
	}
	node, _ := pool.Get("1.1.1.1")
	if node.LossRate != 0 || node.AvgLatency != 0.1 {
		t.Errorf("统计应被刷新, got loss=%v avg=%v", node.LossRate, node.AvgLatency)
	}
	if node.Colo != "HKG" {
		t.Errorf("机房应写入, got %q", node.Colo)
	}
}

func TestHealthCheckEvictsOnLossRate(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")

	// 4 次中前 2 次失败 → 丢包率 0.5，超过 0.1
	checker := NewHealthChecker(pool, healthConfig(), healthProbe(map[string]probeSpec{
		"1.1.1.1": {failFirst: 2, latency: 0.1, colo: "HKG"},
	}))

	evicted := checker.CheckOnce(context.Background())
	if len(evicted) != 1 {
		t.Fatalf("want 1 evicted, got %d", len(evicted))
	}
	if evicted[0].Reason != "丢包率超限" {
		t.Errorf("want 丢包率超限, got %q", evicted[0].Reason)
	}
	if pool.Contains("1.1.1.1") {
		t.Error("淘汰后不应还在池中")
	}
}

func TestHealthCheckEvictsOnLatency(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")

	// 延迟 0.5 秒，超过上限 0.3
	checker := NewHealthChecker(pool, healthConfig(), healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.5, colo: "HKG"},
	}))

	evicted := checker.CheckOnce(context.Background())
	if len(evicted) != 1 || evicted[0].Reason != "延迟超限" {
		t.Fatalf("want 延迟超限, got %v", evicted)
	}
}

func TestHealthCheckEvictsOnColoMismatch(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")

	cfg := healthConfig()
	cfg.ColoWhitelist = []string{"HKG"}
	checker := NewHealthChecker(pool, cfg, healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.1, colo: "LAX"},
	}))

	evicted := checker.CheckOnce(context.Background())
	if len(evicted) != 1 || evicted[0].Reason != "机房不符" {
		t.Fatalf("want 机房不符, got %v", evicted)
	}
}

func TestHealthCheckNeedsConsecutiveFailures(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")

	// 全部失败
	checker := NewHealthChecker(pool, healthConfig(), healthProbe(nil))

	if evicted := checker.CheckOnce(context.Background()); len(evicted) != 0 {
		t.Fatalf("第一次整轮失败不应淘汰（FailStreak=1）, got %v", evicted)
	}
	node, _ := pool.Get("1.1.1.1")
	if node.FailStreak != 1 {
		t.Fatalf("FailStreak want 1, got %d", node.FailStreak)
	}

	evicted := checker.CheckOnce(context.Background())
	if len(evicted) != 1 || evicted[0].Reason != "连续失败" {
		t.Fatalf("第二次应淘汰, got %v", evicted)
	}
}

func TestHealthCheckPromotesBackupAfterEviction(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 2, BackupSize: 1, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "") // 主选
	pool.TryAdd("2.2.2.2", 0.2, "") // 主选
	pool.TryAdd("3.3.3.3", 0.3, "") // 备用

	// 1.1.1.1 延迟超限，2.2.2.2 与 3.3.3.3 正常
	checker := NewHealthChecker(pool, healthConfig(), healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.9, colo: "HKG"},
		"2.2.2.2": {latency: 0.1, colo: "HKG"},
		"3.3.3.3": {latency: 0.2, colo: "HKG"},
	}))

	checker.CheckOnce(context.Background())

	primary, backup := pool.Snapshot()
	if len(primary) != 2 {
		t.Fatalf("主选空缺应由备用递补, got %d", len(primary))
	}
	if len(backup) != 0 {
		t.Fatalf("备用应被提空, got %d", len(backup))
	}
	if !pool.Contains("3.3.3.3") {
		t.Error("备用节点应已升入主选")
	}
}

func TestHealthCheckRespectsContext(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 3, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")
	pool.TryAdd("2.2.2.2", 0.1, "")
	pool.TryAdd("3.3.3.3", 0.1, "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checker := NewHealthChecker(pool, healthConfig(), healthProbe(nil))

	if evicted := checker.CheckOnce(ctx); len(evicted) != 0 {
		t.Fatalf("ctx 取消后不应产生淘汰, got %v", evicted)
	}
}

func TestHealthCheckRunStopsOnCancel(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")

	cfg := healthConfig()
	cfg.Interval = 10 * time.Millisecond

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		NewHealthChecker(pool, cfg, healthProbe(map[string]probeSpec{
			"1.1.1.1": {latency: 0.1, colo: "HKG"},
		})).Run(ctx, nil)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run 应在 ctx 取消后退出")
	}
}

func TestHealthCheckRunReportsEviction(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")

	cfg := healthConfig()
	cfg.Interval = 0 // 立即执行一次

	var got []Evicted
	NewHealthChecker(pool, cfg, healthProbe(map[string]probeSpec{
		"1.1.1.1": {failFirst: 3, latency: 0.1, colo: "HKG"},
	})).Run(context.Background(), func(evicted []Evicted) {
		got = evicted
	})

	if len(got) != 1 {
		t.Fatalf("want 1 reported eviction, got %d", len(got))
	}
}
