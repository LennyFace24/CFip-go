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

// failStreakLimit 连续「整轮失败」多少次才淘汰节点。
// 不进配置：它只影响淘汰的保守程度，用户无需感知。
const failStreakLimit = 3

// maxEvictionLog 保留的最近淘汰记录条数
const maxEvictionLog = 20

// poolUpdateThrottle 扫描阶段推送快照的最小间隔，避免每个 IP 都触发一次刷新
const poolUpdateThrottle = 200 * time.Millisecond

// statsRefreshInterval 运行期的定期快照间隔，用于刷新活跃连接数等实时指标
const statsRefreshInterval = 2 * time.Second

// listenReadyTimeout 等待监听绑定完成的时长上限
const listenReadyTimeout = time.Second

// PoolNode 池中的一个节点视图。Latency 单位为毫秒。
type PoolNode struct {
	IP         string
	Colo       string
	Latency    float64 // < 0 表示该轮全部失败
	LossRate   float64
	Samples    int
	FailStreak int
	UpdatedAt  int64 // Unix 毫秒
}

// EvictionRecord 一条淘汰记录
type EvictionRecord struct {
	IP     string
	Reason string
	Time   int64 // Unix 毫秒
}

// PoolSnapshot 池的完整快照，前端据此整体刷新界面
type PoolSnapshot struct {
	Running       bool
	PrimaryTarget int
	BackupTarget  int
	ListenAddr    string // 本地 SOCKS5 监听地址；空串表示未监听
	ListenError   string
	ActiveConns   int
	Primary       []PoolNode
	Backup        []PoolNode
	Evictions     []EvictionRecord
}

// ProxyService 构建并长期维护 IP 池，并在池有节点后开启本地 SOCKS5 转发。
// 与 SpeedService（一次性测速）相互独立。
type ProxyService struct {
	app           *application.App
	mu            sync.Mutex
	pool          *core.Pool
	forwarder     *core.Forwarder
	cancel        context.CancelFunc
	evicts        []EvictionRecord
	listenErr     string
	primaryTarget int
	backupTarget  int
}

func (s *ProxyService) ServiceName() string { return "ProxyService" }

// StartPool 扫描达标节点填满池，随后开启 SOCKS5 转发与周期健康检查。
func (s *ProxyService) StartPool(text string) error {
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
		return errors.New("IP 池已在运行")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.evicts = nil
	s.listenErr = ""
	s.forwarder = nil
	s.primaryTarget = cfg.PrimarySize
	s.backupTarget = cfg.BackupSize
	s.pool = core.NewPool(core.PoolConfig{
		PrimarySize: cfg.PrimarySize,
		BackupSize:  cfg.BackupSize,
		Cooldown:    time.Duration(cfg.Cooldown) * time.Second,
	})
	pool := s.pool
	s.mu.Unlock()

	go s.run(ctx, pool, ips, cfg)
	return nil
}

// StopPool 停止扫描、转发与健康检查，池中节点保留以便查看。
func (s *ProxyService) StopPool() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.emitUpdate()
}

// Snapshot 返回当前池的快照
func (s *ProxyService) Snapshot() PoolSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pool == nil {
		return PoolSnapshot{}
	}
	primary, backup := s.pool.Snapshot()
	snapshot := PoolSnapshot{
		Running:       s.cancel != nil,
		PrimaryTarget: s.primaryTarget,
		BackupTarget:  s.backupTarget,
		ListenError:   s.listenErr,
		Primary:       toPoolNodes(primary),
		Backup:        toPoolNodes(backup),
		Evictions:     append([]EvictionRecord(nil), s.evicts...),
	}
	if s.forwarder != nil {
		snapshot.ListenAddr = s.forwarder.Addr()
		snapshot.ActiveConns = s.forwarder.ActiveConns()
	}
	return snapshot
}

func (s *ProxyService) run(ctx context.Context, pool *core.Pool, ips []core.IP, cfg *config.Config) {
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.forwarder = nil
		s.mu.Unlock()
		s.emitUpdate()
	}()

	s.scan(ctx, pool, ips, cfg)
	if ctx.Err() != nil {
		return
	}

	s.startForwarder(ctx, pool, cfg.ProxyListen)
	go s.emitPeriodically(ctx)

	checker := core.NewHealthChecker(pool, core.HealthConfig{
		Interval:        time.Duration(cfg.HealthInterval) * time.Second,
		Times:           cfg.PingTimes,
		Gap:             time.Duration(cfg.PingGap) * time.Millisecond,
		LossLimit:       cfg.LossLimit,
		LatencyLimit:    float64(cfg.Latency) / 1000,
		FailStreakLimit: failStreakLimit,
		ColoWhitelist:   core.SplitColos(cfg.Colo),
	}, nil)

	s.emitUpdate()
	checker.Run(ctx, func(evicted []core.Evicted) {
		now := time.Now().UnixMilli()
		for _, item := range evicted {
			s.appendEviction(EvictionRecord{IP: item.Node.IP, Reason: item.Reason, Time: now})
		}
		s.emitUpdate()
	})
}

// startForwarder 池中已有节点后开启本地 SOCKS5 监听。
// 监听失败不终止流程，原因记录在快照里由界面提示。
func (s *ProxyService) startForwarder(ctx context.Context, pool *core.Pool, addr string) {
	forwarder := core.NewForwarder(pool)

	s.mu.Lock()
	s.forwarder = forwarder
	s.mu.Unlock()

	go func() {
		if err := forwarder.Listen(ctx, addr); err != nil {
			s.mu.Lock()
			s.listenErr = err.Error()
			s.mu.Unlock()
			s.emitUpdate()
		}
	}()

	// 等绑定完成，让首次推送的快照就带上监听地址
	deadline := time.Now().Add(listenReadyTimeout)
	for time.Now().Before(deadline) && forwarder.Addr() == "" && ctx.Err() == nil {
		time.Sleep(10 * time.Millisecond)
	}
}

// scan 扫描候选 IP，把达标节点填入池中，池满即停。
func (s *ProxyService) scan(ctx context.Context, pool *core.Pool, ips []core.IP, cfg *config.Config) {
	limitSec := float64(cfg.Latency) / 1000
	whitelist := core.SplitColos(cfg.Colo)
	lastEmit := time.Now()

	for r := range core.StreamLatency(ctx, ips, cfg.Concurrency, nil) {
		if ctx.Err() != nil {
			return
		}
		if r.Latency < 0 || r.Latency > limitSec {
			continue
		}
		if !core.ColoAllowed(r.Colo, whitelist) {
			continue
		}
		if !pool.NeedsMore() {
			return
		}

		pool.TryAdd(r.IP.IP, r.Latency, r.Colo)
		if time.Since(lastEmit) >= poolUpdateThrottle {
			s.emitUpdate()
			lastEmit = time.Now()
		}
	}
	s.emitUpdate()
}

// emitPeriodically 运行期定期推送快照，刷新活跃连接数等随时在变的指标
func (s *ProxyService) emitPeriodically(ctx context.Context) {
	ticker := time.NewTicker(statsRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.emitUpdate()
		}
	}
}

func (s *ProxyService) appendEviction(record EvictionRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evicts = append(s.evicts, record)
	if len(s.evicts) > maxEvictionLog {
		s.evicts = s.evicts[len(s.evicts)-maxEvictionLog:]
	}
}

func (s *ProxyService) emitUpdate() {
	if s.app == nil {
		return
	}
	s.app.Event.Emit("pool:update", s.Snapshot())
}

func toPoolNodes(nodes []core.Node) []PoolNode {
	out := make([]PoolNode, 0, len(nodes))
	for _, node := range nodes {
		latency := -1.0
		if node.AvgLatency >= 0 {
			latency = node.AvgLatency * 1000
		}
		out = append(out, PoolNode{
			IP:         node.IP,
			Colo:       node.Colo,
			Latency:    latency,
			LossRate:   node.LossRate,
			Samples:    node.Samples,
			FailStreak: node.FailStreak,
			UpdatedAt:  node.UpdatedAt.UnixMilli(),
		})
	}
	return out
}
