package core

import (
	"context"
	"time"
)

// minSamplesForEviction 累积多少轮检查后才允许判定，给新入池的节点留出保护期
const minSamplesForEviction = 3

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

type actionKind int

const (
	actionNone actionKind = iota
	actionRecover
	actionIsolate
	actionRemove
)

// HealthChecker 周期性复测池中节点：达标的解除隔离，超限的先隔离后移除。
type HealthChecker struct {
	pool  *Pool
	cfg   HealthConfig
	probe ProbeLatency
}

func NewHealthChecker(pool *Pool, cfg HealthConfig, probe ProbeLatency) *HealthChecker {
	return &HealthChecker{pool: pool, cfg: cfg, probe: probe}
}

// CheckOnce 对池中每个节点采样一次，按结果隔离、恢复或淘汰，返回淘汰明细。
//
// 超限的节点不会立即移除：先隔离观察，下一轮仍不达标才淘汰。
// 每次淘汰都要通过容量闸门，避免把可用节点清空。
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

		action, reason := h.decide(sample, current)
		switch action {
		case actionRecover:
			h.pool.Recover(node.IP)
		case actionIsolate:
			h.pool.Isolate(node.IP)
		case actionRemove:
			if !h.pool.TryEvict() {
				continue // 容量已触底，保留节点
			}
			evicted = append(evicted, Evicted{Node: current, Reason: reason})
			h.pool.Remove(node.IP)
		}
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
		evicted := h.CheckOnce(ctx)
		if onEvicted != nil && len(evicted) > 0 {
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

// decide 判定节点该采取什么动作。
func (h *HealthChecker) decide(sample SampleResult, node Node) (actionKind, string) {
	// 机房不符：直接移除，与样本数无关
	if sample.Success > 0 && !ColoAllowed(sample.Colo, h.cfg.ColoWhitelist) {
		return actionRemove, "机房不符"
	}

	// 样本不足：不判定，避免刚入池就被误杀
	if node.Samples < minSamplesForEviction {
		return actionNone, ""
	}

	reason := h.overLimitReason(sample, node)
	if reason == "" {
		return actionRecover, ""
	}
	if node.Isolated {
		return actionRemove, reason + "（隔离后仍不达标）"
	}
	return actionIsolate, reason
}

// overLimitReason 返回超限原因，未超限返回空串。
func (h *HealthChecker) overLimitReason(sample SampleResult, node Node) string {
	if sample.Success == 0 {
		if h.cfg.FailStreakLimit > 0 && node.FailStreak >= h.cfg.FailStreakLimit {
			return "连续无响应"
		}
		if h.cfg.LossLimit > 0 && node.LossRate > h.cfg.LossLimit {
			return "丢包率超限"
		}
		return ""
	}

	delayOver := h.cfg.LatencyLimit > 0 && node.AvgLatency > h.cfg.LatencyLimit
	lossOver := h.cfg.LossLimit > 0 && node.LossRate > h.cfg.LossLimit

	switch {
	case delayOver && lossOver:
		return "延迟与丢包率均超限"
	case delayOver:
		return "延迟超限"
	case lossOver:
		return "丢包率超限"
	default:
		return ""
	}
}
