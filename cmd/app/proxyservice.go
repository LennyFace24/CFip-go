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

const (
	// failStreakLimit 连续「整轮失败」多少次才算超限。
	// 不进配置：它只影响淘汰的保守程度，用户无需感知。
	failStreakLimit = 3

	// maxEvictionLog 保留的最近淘汰记录条数
	maxEvictionLog = 20

	// scanBatch 每批从候选队列取多少个 IP 去探测
	scanBatch = 200

	// maxScanRounds 候选耗尽后最多再循环几轮，避免无休止重扫
	maxScanRounds = 2

	// poolUpdateThrottle 扫描阶段推送快照的最小间隔
	poolUpdateThrottle = 200 * time.Millisecond

	// statsRefreshInterval 运行期的定期快照间隔，刷新活跃连接数等实时指标
	statsRefreshInterval = 2 * time.Second

	// listenReadyTimeout 等待监听绑定完成的时长上限
	listenReadyTimeout = time.Second
)

// PoolNode 池中的一个节点视图。Latency 单位为毫秒，取最近一轮采样结果。
type PoolNode struct {
	IP         string
	Colo       string
	Latency    float64 // 最近一轮采样的均值；该轮全失败时保留上一次的值
	LossRate   float64
	Samples    int
	FailStreak int
	Isolated   bool
	UpdatedAt  int64 // Unix 毫秒
}

// EvictionRecord 一条淘汰记录
type EvictionRecord struct {
	IP     string
	Reason string
	Time   int64 // Unix 毫秒
}

// 代理所处的阶段
const (
	phaseIdle     = "idle"     // 未启动
	phaseScanning = "scanning" // 首轮扫描候选 IP 中
	phaseReady    = "ready"    // 扫描完成，池已就绪
)

// PoolSnapshot 池的完整快照，前端据此整体刷新界面
type PoolSnapshot struct {
	Running       bool
	Phase         string
	ScanTotal     int // 候选 IP 总数
	ScanDone      int // 本轮扫描已探测的数量
	PrimaryTarget int
	BackupTarget  int
	ListenAddr    string // 本地 SOCKS5 监听地址；空串表示未监听
	ListenError   string
	ActiveConns   int
	Primary       []PoolNode
	Backup        []PoolNode
	Evictions     []EvictionRecord
}

// candidateQueue 候选 IP 队列，按顺序取用，耗尽后可回到队首重新扫。
type candidateQueue struct {
	mu    sync.Mutex
	items []core.IP
	next  int
}

func newCandidateQueue(items []core.IP) *candidateQueue {
	return &candidateQueue{items: items}
}

// take 取下一批候选；已取完返回 nil。
func (q *candidateQueue) take(size int) []core.IP {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.next >= len(q.items) {
		return nil
	}
	end := min(q.next+size, len(q.items))
	batch := q.items[q.next:end]
	q.next = end
	return batch
}

// rewind 回到队首，用于候选耗尽后重新扫描。
func (q *candidateQueue) rewind() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.next = 0
}

// ProxyService 构建并长期维护 IP 池，并在池有节点后开启本地 SOCKS5 转发。
// 与 SpeedService（一次性测速）相互独立。
type ProxyService struct {
	app        *application.App
	mu         sync.Mutex
	pool       *core.Pool
	forwarder  *core.Forwarder
	checker    *core.HealthChecker
	candidates *candidateQueue
	cfg        *config.Config
	cancel     context.CancelFunc
	trigger    chan struct{}
	evicts     []EvictionRecord
	listenErr  string
	phase      string
	scanTotal  int
	scanDone   int
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
	s.checker = nil
	s.trigger = make(chan struct{}, 1)
	s.phase = phaseScanning
	s.scanTotal = len(ips)
	s.scanDone = 0
	s.cfg = cfg
	s.pool = core.NewPool(core.PoolConfig{
		PrimarySize: cfg.PrimarySize,
		BackupSize:  cfg.BackupSize,
		Cooldown:    time.Duration(cfg.Cooldown) * time.Second,
	})
	s.candidates = newCandidateQueue(ips)
	pool := s.pool
	candidates := s.candidates
	s.mu.Unlock()

	go s.run(ctx, pool, candidates, cfg)
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

// Recheck 立即对池中节点复测一轮，不等下一个周期。
func (s *ProxyService) Recheck() error {
	s.mu.Lock()
	trigger := s.trigger
	running := s.cancel != nil
	s.mu.Unlock()

	if !running || trigger == nil {
		return errors.New("IP 池未在运行")
	}
	select {
	case trigger <- struct{}{}:
	default: // 已有待处理的复测请求
	}
	return nil
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
		Phase:         s.phase,
		ScanTotal:     s.scanTotal,
		ScanDone:      s.scanDone,
		PrimaryTarget: primaryTarget(s.cfg),
		BackupTarget:  backupTarget(s.cfg),
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

func (s *ProxyService) run(ctx context.Context, pool *core.Pool, candidates *candidateQueue, cfg *config.Config) {
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.forwarder = nil
		s.checker = nil
		s.trigger = nil
		s.phase = phaseIdle
		s.mu.Unlock()
		s.emitUpdate()
	}()

	// 首轮：把候选扫一遍，填满池
	s.scanFromCursor(ctx, pool, candidates, cfg)
	if ctx.Err() != nil {
		return
	}

	s.mu.Lock()
	s.phase = phaseReady
	s.mu.Unlock()

	s.startForwarder(ctx, pool, cfg.ProxyListen)

	checker := core.NewHealthChecker(pool, healthConfigFrom(cfg), nil)
	trigger := make(chan struct{}, 1)

	s.mu.Lock()
	s.checker = checker
	s.trigger = trigger
	s.mu.Unlock()

	go s.emitPeriodically(ctx)

	s.emitUpdate()
	s.runHealthLoop(ctx, pool, candidates, checker, cfg, trigger)
}

// runHealthLoop 周期复测；收到 trigger 时立即复测一次。
// 使用 Timer 而非 Ticker：每轮结束后才计下一轮，避免检查耗时超过周期时堆积。
func (s *ProxyService) runHealthLoop(
	ctx context.Context,
	pool *core.Pool,
	candidates *candidateQueue,
	checker *core.HealthChecker,
	cfg *config.Config,
	trigger <-chan struct{},
) {
	interval := time.Duration(cfg.HealthInterval) * time.Second
	if interval <= 0 {
		interval = time.Minute
	}

	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-trigger:
		}
		s.checkAndRefill(ctx, pool, candidates, checker, cfg)
		timer.Reset(interval)
	}
}

// checkAndRefill 执行一轮健康检查，记录淘汰并在池出现空缺时补测。
func (s *ProxyService) checkAndRefill(
	ctx context.Context,
	pool *core.Pool,
	candidates *candidateQueue,
	checker *core.HealthChecker,
	cfg *config.Config,
) {
	evicted := checker.CheckOnce(ctx)
	if len(evicted) > 0 {
		now := time.Now().UnixMilli()
		for _, item := range evicted {
			s.appendEviction(EvictionRecord{IP: item.Node.IP, Reason: item.Reason, Time: now})
		}
	}

	if pool.NeedsMore() && ctx.Err() == nil {
		s.refill(ctx, pool, candidates, cfg)
	}
	s.emitUpdate()
}

// scanFromCursor 从候选游标处继续扫描，直到候选耗尽或池不再缺节点。
func (s *ProxyService) scanFromCursor(ctx context.Context, pool *core.Pool, candidates *candidateQueue, cfg *config.Config) {
	lastEmit := time.Now()

	for ctx.Err() == nil && pool.NeedsMore() {
		batch := candidates.take(scanBatch)
		if len(batch) == 0 {
			return
		}
		s.probeAndFill(ctx, pool, batch, cfg)
		if time.Since(lastEmit) >= poolUpdateThrottle {
			s.emitUpdate()
			lastEmit = time.Now()
		}
	}
	s.emitUpdate()
}

// refill 淘汰后补位：先用没测过的候选，候选耗尽则回到队首重新扫。
func (s *ProxyService) refill(ctx context.Context, pool *core.Pool, candidates *candidateQueue, cfg *config.Config) {
	s.scanFromCursor(ctx, pool, candidates, cfg)
	for round := 0; round < maxScanRounds && ctx.Err() == nil && pool.NeedsMore(); round++ {
		candidates.rewind()
		s.scanFromCursor(ctx, pool, candidates, cfg)
	}
}

// probeAndFill 探测一批 IP，把达标且通过机房白名单的填入池中。
func (s *ProxyService) probeAndFill(ctx context.Context, pool *core.Pool, batch []core.IP, cfg *config.Config) {
	// 池满时提前退出，靠 cancel 释放仍在探测的协程
	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	limitSec := float64(cfg.Latency) / 1000
	whitelist := core.SplitColos(cfg.Colo)

	for r := range core.StreamLatency(scanCtx, batch, cfg.Concurrency, nil) {
		if ctx.Err() != nil {
			return
		}
		s.addScanDone(1)
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
	}
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

func (s *ProxyService) addScanDone(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanDone += delta
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

func healthConfigFrom(cfg *config.Config) core.HealthConfig {
	return core.HealthConfig{
		Interval:        time.Duration(cfg.HealthInterval) * time.Second,
		Times:           cfg.PingTimes,
		Gap:             time.Duration(cfg.PingGap) * time.Millisecond,
		LossLimit:       cfg.LossLimit,
		LatencyLimit:    float64(cfg.Latency) / 1000,
		FailStreakLimit: failStreakLimit,
		ColoWhitelist:   core.SplitColos(cfg.Colo),
	}
}

func primaryTarget(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	return cfg.PrimarySize
}

func backupTarget(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	return cfg.BackupSize
}

func toPoolNodes(nodes []core.Node) []PoolNode {
	out := make([]PoolNode, 0, len(nodes))
	for _, node := range nodes {
		latency := node.Latency
		if latency < 0 {
			latency = node.AvgLatency
		}
		out = append(out, PoolNode{
			IP:         node.IP,
			Colo:       node.Colo,
			Latency:    latency * 1000,
			LossRate:   node.LossRate,
			Samples:    node.Samples,
			FailStreak: node.FailStreak,
			Isolated:   node.Isolated,
			UpdatedAt:  node.UpdatedAt.UnixMilli(),
		})
	}
	return out
}
