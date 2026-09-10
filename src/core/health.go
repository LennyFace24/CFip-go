package core

import (
	"context"
	"time"
)

// HealthConfig 健康检查参数。延迟单位为秒，与 core 其余部分一致。
type HealthConfig struct {
	Interval        time.Duration // 检查周期，必须大于 0
	Times           int           // 每个节点每次检查的采样次数
	Gap             time.Duration // 采样间隔
	LossLimit       float64       // 丢包率上限，超过即淘汰
	LatencyLimit    float64       // 平均延迟上限（秒），超过即淘汰
	FailStreakLimit int           // 连续「整轮失败」次数上限
	ColoWhitelist   []string      // 机房白名单，为空表示不过滤
}

// Evicted 一条淘汰记录。Node 是淘汰时（已刷新统计）的节点快照。
type Evicted struct {
	Node   Node
	Reason string
}

// HealthChecker 周期性复测池中节点，淘汰劣化的并由备用递补。
type HealthChecker struct {
	pool  *Pool
	cfg   HealthConfig
	probe ProbeLatency
}

func NewHealthChecker(pool *Pool, cfg HealthConfig, probe ProbeLatency) *HealthChecker {
	return &HealthChecker{pool: pool, cfg: cfg, probe: probe}
}

// CheckOnce 对池中每个节点采样一次，刷新统计并淘汰劣化的，返回淘汰明细。
//
// 淘汰后主选出现的空缺由备用中延迟最低者递补。
func (h *HealthChecker) CheckOnce(ctx context.Context) []Evicted {
	var evicted []Evicted

	for _, node := range h.pool.All() {
		if ctx.Err() != nil {
			break
		}

		sample := ProbeRepeated(ctx, IP{IP: node.IP}, h.cfg.Times, h.cfg.Gap, h.probe)
		h.pool.Update(node.IP, sample)

		current, ok := h.pool.Get(node.IP)
		if !ok {
			continue
		}
		reason := h.reason(sample, current)
		if reason == "" {
			continue
		}

		evicted = append(evicted, Evicted{Node: current, Reason: reason})
		h.pool.Remove(node.IP)
	}

	for i := 0; i < len(evicted); i++ {
		if !h.pool.Promote() {
			break
		}
	}
	return evicted
}

// Run 按 Interval 周期执行检查，ctx 取消即退出。
// 每轮检查结束后才开始计下一轮，避免检查耗时超过周期时堆积。
func (h *HealthChecker) Run(ctx context.Context, onEvicted func([]Evicted)) {
	report := func() {
		if onEvicted == nil {
			return
		}
		if evicted := h.CheckOnce(ctx); len(evicted) > 0 {
			onEvicted(evicted)
		}
	}

	if h.cfg.Interval <= 0 {
		report()
		return
	}

	timer := time.NewTimer(h.cfg.Interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			report()
			timer.Reset(h.cfg.Interval)
		}
	}
}

// reason 判定节点是否应被淘汰，返回空串表示保留。
func (h *HealthChecker) reason(sample SampleResult, node Node) string {
	if h.cfg.FailStreakLimit > 0 && node.FailStreak >= h.cfg.FailStreakLimit {
		return "连续失败"
	}
	if sample.Success == 0 {
		// 整轮失败交给 FailStreak 判定，避免单次抖动就误杀
		return ""
	}
	if h.cfg.LossLimit > 0 && sample.LossRate > h.cfg.LossLimit {
		return "丢包率超限"
	}
	if h.cfg.LatencyLimit > 0 && sample.AvgLatency > h.cfg.LatencyLimit {
		return "延迟超限"
	}
	if !ColoAllowed(sample.Colo, h.cfg.ColoWhitelist) {
		return "机房不符"
	}
	return ""
}
