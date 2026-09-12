package core

import (
	"math"
	"sort"
	"sync"
)

const (
	// ucbExplore 探索系数：越大越倾向探测样本少的候选
	ucbExplore = 1.0

	// ucbDiscount 指数折扣因子：越小越只看近期样本，用于适应网络变化
	ucbDiscount = 0.9

	// ucbMaxSamples 单个候选的有效样本上限。
	// 封顶后探索项不再衰减，否则长期运行后所有候选的探索项趋近于 0，
	// 打分退化为纯均值比较，失去主动探索未测候选的能力。
	ucbMaxSamples = 20

	// ucbFailPenalty 探测失败的等效延迟（秒），按较差样本计入
	ucbFailPenalty = 1.0
)

// UCBBandit 用 UCB 打分决定候选 IP 的探测顺序。
//
// 与标准 UCB 的差异：
//   - 奖励取反：我们最小化延迟，打分 = 均值 - 探索项，取最小者
//   - 样本数封顶，配合指数折扣，使旧样本随时间失效（网络会变）
//   - 支持一次取出多个候选（Top-K），而不只是最优的那一个
type UCBBandit struct {
	mu     sync.Mutex
	ips    []string
	mean   []float64 // 折扣加权平均延迟（秒）
	counts []int32   // 有效样本数，封顶 ucbMaxSamples
	index  map[string]int
	total  int64 // 总观察次数，用于 ln(t)
}

func NewUCBBandit(ips []string) *UCBBandit {
	u := &UCBBandit{
		ips:    append([]string(nil), ips...),
		mean:   make([]float64, len(ips)),
		counts: make([]int32, len(ips)),
		index:  make(map[string]int, len(ips)),
	}
	for i, ip := range u.ips {
		u.index[ip] = i
	}
	return u
}

// Select 返回最值得探测的 n 个候选 IP：延迟低、或样本少的优先。
func (u *UCBBandit) Select(n int) []string {
	if n <= 0 {
		return nil
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if len(u.ips) == 0 {
		return nil
	}
	if n > len(u.ips) {
		n = len(u.ips)
	}

	lnTotal := math.Log(float64(u.total) + 1)
	type scored struct {
		index int
		score float64
	}

	scores := make([]scored, len(u.ips))
	for i := range u.ips {
		count := float64(u.counts[i])
		if count == 0 {
			scores[i] = scored{index: i, score: math.Inf(-1)} // 从未探测 → 最优先
			continue
		}
		bonus := ucbExplore * math.Sqrt(lnTotal/count)
		scores[i] = scored{index: i, score: u.mean[i] - bonus}
	}

	// 稳定排序：同分时保持候选原有顺序，保证结果可复现
	sort.SliceStable(scores, func(a, b int) bool {
		return scores[a].score < scores[b].score
	})

	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = u.ips[scores[i].index]
	}
	return out
}

// Observe 记录一次探测结果。ok 为 false 或延迟为负时按失败惩罚计入。
func (u *UCBBandit) Observe(ip string, latency float64, ok bool) {
	u.mu.Lock()
	defer u.mu.Unlock()

	idx, exists := u.index[ip]
	if !exists {
		return
	}

	sample := latency
	if !ok || sample < 0 {
		sample = ucbFailPenalty
	}

	if u.counts[idx] == 0 {
		u.mean[idx] = sample
	} else {
		u.mean[idx] = u.mean[idx]*ucbDiscount + sample*(1-ucbDiscount)
	}
	if u.counts[idx] < ucbMaxSamples {
		u.counts[idx]++
	}
	u.total++
}

// Stats 返回某个候选的当前统计，供测试与界面展示。
func (u *UCBBandit) Stats(ip string) (mean float64, count int32, ok bool) {
	u.mu.Lock()
	defer u.mu.Unlock()

	idx, exists := u.index[ip]
	if !exists {
		return 0, 0, false
	}
	return u.mean[idx], u.counts[idx], true
}

// Total 返回累计观察次数
func (u *UCBBandit) Total() int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.total
}
