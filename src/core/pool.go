package core

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

const (
	// pickWindow 从延迟最低的若干个节点里随机挑选，避免所有流量都打向同一个 IP
	pickWindow = 3

	// sampleWindow EWMA 的样本窗口：样本数达到该值后不再增长
	sampleWindow = 20
	// ewmaAlpha 指数加权移动平均的衰减因子，与 sampleWindow 对应
	ewmaAlpha = 2.0 / (sampleWindow + 1)

	// compareDelayThreshold 判定「延迟差异显著」的阈值（秒），20ms
	compareDelayThreshold = 0.02
	// compareLossThreshold 判定「丢包差异显著」的阈值
	compareLossThreshold = 0.01
)

// PoolConfig IP 池的容量与冷却策略。
type PoolConfig struct {
	PrimarySize int
	BackupSize  int
	Cooldown    time.Duration
}

// Node IP 池中的一个节点。延迟单位与 core 其他部分一致，为秒。
type Node struct {
	IP         string
	Colo       string
	Latency    float64 // 最近一轮采样的均值；该轮全失败为 -1
	AvgLatency float64 // EWMA 延迟，淘汰判定用
	LossRate   float64 // EWMA 丢包率，淘汰判定用
	Samples    int     // 已累积的检查轮数，封顶 sampleWindow
	FailStreak int     // 连续「整轮失败」的次数
	Isolated   bool    // 隔离观察中：仍在池内，但不参与流量分发
	UpdatedAt  time.Time
}

// AddResult 是 TryAdd 的结果。
type AddResult int

const (
	AddedToPrimary AddResult = iota
	AddedToBackup
	QueueFull
	AlreadyExists
	InCooldown
)

// Pool 维护主选、备用与冷却三段节点。
//
// 主选承载流量，备用在主选出现空缺时按「更优者优先」递补；
// 被淘汰的节点进入冷却，冷却期内不参与补位，避免反复进出。
type Pool struct {
	mu      sync.RWMutex
	cfg     PoolConfig
	primary []*Node
	backup  []*Node
	cooling map[string]time.Time
}

func NewPool(cfg PoolConfig) *Pool {
	return &Pool{cfg: cfg, cooling: make(map[string]time.Time)}
}

// TryAdd 尝试把节点放入池中：主选未满进主选，否则进备用，都满返回 QueueFull。
func (p *Pool) TryAdd(ip string, latency float64, colo string) AddResult {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cleanCoolingLocked()

	if _, cooling := p.cooling[ip]; cooling {
		return InCooldown
	}
	if p.findLocked(ip) != nil {
		return AlreadyExists
	}
	if len(p.primary) < p.cfg.PrimarySize {
		p.primary = append(p.primary, newNode(ip, latency, colo))
		return AddedToPrimary
	}
	if len(p.backup) < p.cfg.BackupSize {
		p.backup = append(p.backup, newNode(ip, latency, colo))
		return AddedToBackup
	}
	return QueueFull
}

// Remove 把节点从主选/备用移除并放入冷却名单。
func (p *Pool) Remove(ip string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	removeFrom(&p.primary, ip)
	removeFrom(&p.backup, ip)
	p.cooling[ip] = time.Now().Add(p.cfg.Cooldown)
}

// Update 用一轮采样结果刷新节点统计，节点不存在返回 false。
//
// 延迟与丢包都按 EWMA 累积：单次波动只会小幅改变结果，
// 避免偶发超时把节点直接推过阈值。
func (p *Pool) Update(ip string, sample SampleResult) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	node := p.findLocked(ip)
	if node == nil {
		return false
	}

	first := node.Samples == 0
	node.Samples = min(node.Samples+1, sampleWindow)

	if sample.Success == 0 {
		node.FailStreak++
		node.LossRate = ewma(node.LossRate, 1, first)
	} else {
		node.FailStreak = 0
		node.Latency = sample.AvgLatency
		node.AvgLatency = ewma(node.AvgLatency, sample.AvgLatency, first)
		node.LossRate = ewma(node.LossRate, sample.LossRate, first)
	}
	if sample.Colo != "" {
		node.Colo = sample.Colo
	}
	node.UpdatedAt = time.Now()
	return true
}

// Isolate 把节点标记为隔离。返回 false 表示节点不存在或已在隔离中。
func (p *Pool) Isolate(ip string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	node := p.findLocked(ip)
	if node == nil || node.Isolated {
		return false
	}
	node.Isolated = true
	return true
}

// Recover 解除隔离并清零连续失败计数。
func (p *Pool) Recover(ip string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	node := p.findLocked(ip)
	if node == nil {
		return false
	}
	node.Isolated = false
	node.FailStreak = 0
	return true
}

// TryEvict 申请一次淘汰许可。
//
// 主选中「可用」节点的数量不允许跌破目标值的一半（向上取整）；一旦触底，
// 必须能从备用提升一个可用节点才放行，备用也空则拒绝淘汰——
// 宁可保留一个劣化节点，也不让可用容量归零。
func (p *Pool) TryEvict() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.activePrimaryLocked() > minActiveTarget(p.cfg.PrimarySize) {
		return true
	}

	best := bestOf(p.backup)
	if best == nil {
		return false
	}
	removeFrom(&p.backup, best.IP)
	best.Isolated = false
	best.FailStreak = 0

	// 主选已达容量上限时，先腾出一个不可用的位置，保持总量守恒
	if len(p.primary) >= p.cfg.PrimarySize {
		for i, node := range p.primary {
			if node.Isolated {
				p.cooling[node.IP] = time.Now().Add(p.cfg.Cooldown)
				p.primary = append(p.primary[:i], p.primary[i+1:]...)
				break
			}
		}
	}
	p.primary = append(p.primary, best)
	return true
}

// minActiveTarget 主选中必须保有的可用节点数量下限（目标值的一半，向上取整）
func minActiveTarget(primarySize int) int {
	return (primarySize + 1) / 2
}

// Promote 把备用中最优的节点提升到主选；主选已满或无备用时返回 false。
func (p *Pool) Promote() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.primary) >= p.cfg.PrimarySize {
		return false
	}
	best := bestOf(p.backup)
	if best == nil {
		return false
	}
	removeFrom(&p.backup, best.IP)
	best.Isolated = false
	p.primary = append(p.primary, best)
	return true
}

// Pick 选出一个可用节点：主选优先，主选无可用的时退到备用；
// 两边都只剩隔离节点时，退而选一个隔离节点，避免完全没有出口。
func (p *Pool) Pick() *Node {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if node := pickFrom(p.primary, false); node != nil {
		return node
	}
	if node := pickFrom(p.backup, false); node != nil {
		return node
	}
	if node := pickFrom(p.primary, true); node != nil {
		return node
	}
	return pickFrom(p.backup, true)
}

// Snapshot 返回按「更优者在前」的主选与备用副本。
func (p *Pool) Snapshot() (primary []Node, backup []Node) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return cloneSorted(p.primary), cloneSorted(p.backup)
}

// All 返回主选与备用的全部节点副本，供健康检查遍历。
func (p *Pool) All() []Node {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := make([]Node, 0, len(p.primary)+len(p.backup))
	for _, node := range p.primary {
		out = append(out, *node)
	}
	for _, node := range p.backup {
		out = append(out, *node)
	}
	return out
}

// Get 返回节点副本，不存在返回 false。
func (p *Pool) Get(ip string) (Node, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if node := p.findLocked(ip); node != nil {
		return *node, true
	}
	return Node{}, false
}

func (p *Pool) Contains(ip string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.findLocked(ip) != nil
}

// NeedsMore 报告主选或备用是否还有空缺。
func (p *Pool) NeedsMore() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.primary) < p.cfg.PrimarySize || len(p.backup) < p.cfg.BackupSize
}

// ActivePrimaryCount 返回主选中未隔离的节点数量。
func (p *Pool) ActivePrimaryCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.activePrimaryLocked()
}

func (p *Pool) activePrimaryLocked() int {
	count := 0
	for _, node := range p.primary {
		if !node.Isolated {
			count++
		}
	}
	return count
}

func (p *Pool) findLocked(ip string) *Node {
	for _, node := range p.primary {
		if node.IP == ip {
			return node
		}
	}
	for _, node := range p.backup {
		if node.IP == ip {
			return node
		}
	}
	return nil
}

func (p *Pool) cleanCoolingLocked() {
	now := time.Now()
	for ip, deadline := range p.cooling {
		if now.After(deadline) {
			delete(p.cooling, ip)
		}
	}
}

func newNode(ip string, latency float64, colo string) *Node {
	return &Node{
		IP:         ip,
		Colo:       colo,
		Latency:    latency,
		AvgLatency: latency,
		UpdatedAt:  time.Now(),
	}
}

func removeFrom(nodes *[]*Node, ip string) {
	for i, node := range *nodes {
		if node.IP == ip {
			*nodes = append((*nodes)[:i], (*nodes)[i+1:]...)
			return
		}
	}
}

// ewma 指数加权移动平均；首个样本直接赋值，避免从 0 缓慢爬升
func ewma(current, sample float64, first bool) float64 {
	if first {
		return sample
	}
	return current*(1-ewmaAlpha) + sample*ewmaAlpha
}

// effectiveLatency 用于排序：整轮失败的节点排到最后
func effectiveLatency(node Node) float64 {
	if node.Isolated {
		return math.MaxFloat64
	}
	if node.AvgLatency < 0 {
		return math.MaxFloat64 / 2
	}
	return node.AvgLatency
}

// CompareEviction 比较两个节点的优劣，返回值 > 0 表示 a 比 b 更应该被淘汰。
//
// 延迟差超过阈值才算显著，否则退到丢包比较——避免在噪声区间内频繁切换。
func CompareEviction(a, b Node) int {
	delayA, delayB := effectiveLatency(a), effectiveLatency(b)
	if diff := delayA - delayB; math.Abs(diff) > compareDelayThreshold {
		return sign(diff)
	}
	if diff := a.LossRate - b.LossRate; math.Abs(diff) > compareLossThreshold {
		return sign(diff)
	}
	return 0
}

func sign(v float64) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// bestOf 取一组节点中最优的一个（CompareEviction 最小者）
func bestOf(nodes []*Node) *Node {
	var best *Node
	for _, node := range nodes {
		if best == nil || CompareEviction(*node, *best) < 0 {
			best = node
		}
	}
	return best
}

// pickFrom 从延迟最低的若干个可用节点里随机挑一个。
// includeIsolated 为 true 时也接受隔离节点（兜底出口）。
func pickFrom(nodes []*Node, includeIsolated bool) *Node {
	candidates := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Isolated && !includeIsolated {
			continue
		}
		candidates = append(candidates, node)
	}
	if len(candidates) == 0 {
		return nil
	}

	sorted := make([]*Node, len(candidates))
	copy(sorted, candidates)
	sortNodes(sorted)

	limit := pickWindow
	if len(sorted) < limit {
		limit = len(sorted)
	}
	return sorted[rand.Intn(limit)]
}

func sortNodes(nodes []*Node) {
	// 插入排序：池子很小，且需要稳定的「更优在前」顺序
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && CompareEviction(*nodes[j-1], *nodes[j]) > 0; j-- {
			nodes[j-1], nodes[j] = nodes[j], nodes[j-1]
		}
	}
}

func cloneSorted(nodes []*Node) []Node {
	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, *node)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && CompareEviction(out[j-1], out[j]) > 0; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
