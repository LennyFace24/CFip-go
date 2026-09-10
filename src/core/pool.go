package core

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// pickWindow 从延迟最低的若干个节点里随机挑选，避免所有流量都打向同一个 IP
const pickWindow = 3

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
	Latency    float64 // 最近一次采样均值；该轮全失败为 -1
	AvgLatency float64 // 判定用的平均延迟
	LossRate   float64 // 丢包率，健康检查写入
	Samples    int     // 最近一次采样的总次数
	FailStreak int     // 连续「整轮失败」的次数
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
// 主选承载流量，备用在主选出现空缺时升级；被淘汰的节点进入冷却，
// 冷却期内不参与补位——否则一次偶发失败就会导致节点反复进出。
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

// Update 用一次采样结果刷新节点统计，节点不存在返回 false。
func (p *Pool) Update(ip string, sample SampleResult) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	node := p.findLocked(ip)
	if node == nil {
		return false
	}
	node.Latency = sample.AvgLatency
	node.AvgLatency = sample.AvgLatency
	node.LossRate = sample.LossRate
	node.Samples = sample.Total
	if sample.Colo != "" {
		node.Colo = sample.Colo
	}
	if sample.Success == 0 {
		node.FailStreak++
	} else {
		node.FailStreak = 0
	}
	node.UpdatedAt = time.Now()
	return true
}

// Pick 选出一个可用节点：主选优先，主选为空时退到备用。
func (p *Pool) Pick() *Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if node := pickFrom(p.primary); node != nil {
		return node
	}
	return pickFrom(p.backup)
}

// Promote 把备用中延迟最低的节点提升到主选；主选已满或无备用时返回 false。
func (p *Pool) Promote() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.primary) >= p.cfg.PrimarySize {
		return false
	}
	best := pickFrom(p.backup)
	if best == nil {
		return false
	}
	removeFrom(&p.backup, best.IP)
	p.primary = append(p.primary, best)
	return true
}

// Snapshot 返回按延迟升序的主选与备用副本。
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

// effectiveLatency 用于排序：整轮失败的节点排到最后
func effectiveLatency(node *Node) float64 {
	if node.AvgLatency < 0 {
		return math.MaxFloat64
	}
	return node.AvgLatency
}

func pickFrom(nodes []*Node) *Node {
	if len(nodes) == 0 {
		return nil
	}
	sorted := make([]*Node, len(nodes))
	copy(sorted, nodes)
	sort.SliceStable(sorted, func(i, j int) bool {
		return effectiveLatency(sorted[i]) < effectiveLatency(sorted[j])
	})

	limit := pickWindow
	if len(sorted) < limit {
		limit = len(sorted)
	}
	return sorted[rand.Intn(limit)]
}

func cloneSorted(nodes []*Node) []Node {
	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, *node)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return effectiveLatency(&out[i]) < effectiveLatency(&out[j])
	})
	return out
}
