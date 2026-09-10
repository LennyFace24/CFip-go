package core

import (
	"context"
	"strings"
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
		LossLimit:       0.25,
		LatencyLimit:    0.3,
		FailStreakLimit: 2,
	}
}

// newHealthPool 构造一个「2 主选 + 1 备用」的池
func newHealthPool() *Pool {
	pool := NewPool(PoolConfig{PrimarySize: 2, BackupSize: 1, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")
	pool.TryAdd("2.2.2.2", 0.1, "")
	pool.TryAdd("3.3.3.3", 0.1, "")
	return pool
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
	if node.Isolated {
		t.Error("健康节点不应被隔离")
	}
	if node.AvgLatency != 0.1 || node.Colo != "HKG" {
		t.Errorf("统计应被刷新, got avg=%v colo=%q", node.AvgLatency, node.Colo)
	}
}

func TestHealthCheckNeedsSamplesBeforeJudging(t *testing.T) {
	pool := newHealthPool()
	checker := NewHealthChecker(pool, healthConfig(), healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.9, colo: "HKG"}, // 明显超限
		"2.2.2.2": {latency: 0.1, colo: "HKG"},
		"3.3.3.3": {latency: 0.1, colo: "HKG"},
	}))

	// minSamplesForEviction = 3，前两轮只积累样本，不做判定
	for round := 1; round <= 2; round++ {
		checker.CheckOnce(context.Background())
		node, _ := pool.Get("1.1.1.1")
		if node.Isolated {
			t.Fatalf("第 %d 轮样本不足，不应隔离", round)
		}
		if node.Samples != round {
			t.Fatalf("第 %d 轮样本数应为 %d, got %d", round, round, node.Samples)
		}
	}
}

func TestHealthCheckIsolatesThenRemoves(t *testing.T) {
	pool := newHealthPool()
	checker := NewHealthChecker(pool, healthConfig(), healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.9, colo: "HKG"},
		"2.2.2.2": {latency: 0.1, colo: "HKG"},
		"3.3.3.3": {latency: 0.1, colo: "HKG"},
	}))
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		checker.CheckOnce(ctx)
	}

	// 第 3 轮：样本够了，首次超限 → 隔离而不是移除
	if evicted := checker.CheckOnce(ctx); len(evicted) != 0 {
		t.Fatalf("首次超限应只隔离, got %v", evicted)
	}
	node, _ := pool.Get("1.1.1.1")
	if !node.Isolated {
		t.Fatal("首次超限后应处于隔离状态")
	}
	if !pool.Contains("1.1.1.1") {
		t.Fatal("隔离的节点仍应留在池中")
	}

	// 第 4 轮：隔离后仍不达标 → 移除
	evicted := checker.CheckOnce(ctx)
	if len(evicted) != 1 {
		t.Fatalf("want 1 evicted, got %d", len(evicted))
	}
	if !strings.Contains(evicted[0].Reason, "延迟超限") {
		t.Errorf("原因应包含延迟超限, got %q", evicted[0].Reason)
	}
	if pool.Contains("1.1.1.1") {
		t.Error("淘汰后不应还在池中")
	}
}

func TestHealthCheckRecoversIsolatedNode(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")
	pool.Isolate("1.1.1.1")

	checker := NewHealthChecker(pool, healthConfig(), healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.1, colo: "HKG"},
	}))

	for i := 0; i < 3; i++ {
		checker.CheckOnce(context.Background())
	}

	node, _ := pool.Get("1.1.1.1")
	if node.Isolated {
		t.Error("恢复达标后应解除隔离")
	}
}

func TestHealthCheckRemovesOnColoMismatch(t *testing.T) {
	pool := newHealthPool()
	cfg := healthConfig()
	cfg.ColoWhitelist = []string{"HKG"}

	checker := NewHealthChecker(pool, cfg, healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.1, colo: "LAX"},
		"2.2.2.2": {latency: 0.1, colo: "HKG"},
		"3.3.3.3": {latency: 0.1, colo: "HKG"},
	}))

	// 机房不符不受样本门槛限制
	evicted := checker.CheckOnce(context.Background())
	if len(evicted) != 1 || evicted[0].Reason != "机房不符" {
		t.Fatalf("want 机房不符, got %v", evicted)
	}
}

func TestHealthCheckEvictionGateKeepsLastNodes(t *testing.T) {
	// 无备用：隔离一个后可用数触底，应拒绝淘汰
	pool := NewPool(PoolConfig{PrimarySize: 2, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")
	pool.TryAdd("2.2.2.2", 0.1, "")
	pool.Isolate("1.1.1.1")

	checker := NewHealthChecker(pool, healthConfig(), healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.9, colo: "HKG"},
		"2.2.2.2": {latency: 0.1, colo: "HKG"},
	}))

	var evicted []Evicted
	for i := 0; i < 5; i++ {
		evicted = checker.CheckOnce(context.Background())
	}
	if len(evicted) != 0 {
		t.Fatalf("容量触底且无备用时应拒绝淘汰, got %v", evicted)
	}
	if !pool.Contains("1.1.1.1") {
		t.Error("节点应被保留")
	}
}

func TestHealthCheckPromotesBackupAfterEviction(t *testing.T) {
	pool := newHealthPool()
	checker := NewHealthChecker(pool, healthConfig(), healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.9, colo: "HKG"},
		"2.2.2.2": {latency: 0.1, colo: "HKG"},
		"3.3.3.3": {latency: 0.1, colo: "HKG"},
	}))
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		checker.CheckOnce(ctx)
	}

	if pool.Contains("1.1.1.1") {
		t.Fatal("劣化节点应已被淘汰")
	}
	primary, backup := pool.Snapshot()
	if len(primary) != 2 {
		t.Fatalf("主选应保持满员, got %d", len(primary))
	}
	if len(backup) != 0 {
		t.Fatalf("备用应已被提升, got %d", len(backup))
	}
	if !pool.Contains("3.3.3.3") {
		t.Error("原备用节点应已进入主选")
	}
}

func TestHealthCheckRespectsContext(t *testing.T) {
	pool := newHealthPool()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checker := NewHealthChecker(pool, healthConfig(), healthProbe(nil))

	if evicted := checker.CheckOnce(ctx); len(evicted) != 0 {
		t.Fatalf("ctx 取消后不应产生淘汰, got %v", evicted)
	}
	node, _ := pool.Get("1.1.1.1")
	if node.Samples != 0 {
		t.Error("ctx 取消后不应发起采样")
	}
}

func TestHealthCheckRunInvokesCheck(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.1.1.1", 0.1, "")

	cfg := healthConfig()
	cfg.Interval = 0 // 立即执行一次

	NewHealthChecker(pool, cfg, healthProbe(map[string]probeSpec{
		"1.1.1.1": {latency: 0.1, colo: "HKG"},
	})).Run(context.Background(), nil)

	node, _ := pool.Get("1.1.1.1")
	if node.Samples != 1 {
		t.Fatalf("Run 应立即执行一次检查, Samples=%d", node.Samples)
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
