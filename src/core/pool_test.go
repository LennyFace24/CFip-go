package core

import (
	"testing"
	"time"
)

func testConfig() PoolConfig {
	return PoolConfig{PrimarySize: 2, BackupSize: 1, Cooldown: 30 * time.Millisecond}
}

func TestPoolTryAddFillsPrimaryThenBackup(t *testing.T) {
	p := NewPool(testConfig())

	if got := p.TryAdd("1.1.1.1", 0.1, "HKG"); got != AddedToPrimary {
		t.Fatalf("1.1.1.1: want AddedToPrimary, got %v", got)
	}
	if got := p.TryAdd("2.2.2.2", 0.2, "HKG"); got != AddedToPrimary {
		t.Fatalf("2.2.2.2: want AddedToPrimary, got %v", got)
	}
	if got := p.TryAdd("3.3.3.3", 0.3, "NRT"); got != AddedToBackup {
		t.Fatalf("3.3.3.3: want AddedToBackup, got %v", got)
	}
	if got := p.TryAdd("4.4.4.4", 0.4, "NRT"); got != QueueFull {
		t.Fatalf("4.4.4.4: want QueueFull, got %v", got)
	}
	if p.NeedsMore() {
		t.Error("池已满，NeedsMore 应为 false")
	}
}

func TestPoolTryAddDuplicate(t *testing.T) {
	p := NewPool(testConfig())
	p.TryAdd("1.1.1.1", 0.1, "HKG")

	if got := p.TryAdd("1.1.1.1", 0.1, "HKG"); got != AlreadyExists {
		t.Fatalf("want AlreadyExists, got %v", got)
	}
	if !p.Contains("1.1.1.1") {
		t.Error("1.1.1.1 应在池中")
	}
}

func TestPoolRemoveEntersCooldown(t *testing.T) {
	p := NewPool(testConfig())
	p.TryAdd("1.1.1.1", 0.1, "HKG")

	p.Remove("1.1.1.1")
	if p.Contains("1.1.1.1") {
		t.Fatal("移除后不应还在池中")
	}
	if got := p.TryAdd("1.1.1.1", 0.1, "HKG"); got != InCooldown {
		t.Fatalf("冷却期内应返回 InCooldown, got %v", got)
	}

	time.Sleep(40 * time.Millisecond)
	if got := p.TryAdd("1.1.1.1", 0.1, "HKG"); got != AddedToPrimary {
		t.Fatalf("冷却结束后应可重新入池, got %v", got)
	}
}

func TestPoolUpdateWritesFirstSampleDirectly(t *testing.T) {
	p := NewPool(testConfig())
	p.TryAdd("1.1.1.1", 0.1, "")

	sample := SampleResult{AvgLatency: 0.25, LossRate: 0.5, Success: 1, Total: 2, Colo: "SIN"}
	if !p.Update("1.1.1.1", sample) {
		t.Fatal("已存在节点应更新成功")
	}

	node, _ := p.Get("1.1.1.1")
	if node.AvgLatency != 0.25 {
		t.Errorf("首个样本应直接赋值, got %v", node.AvgLatency)
	}
	if node.LossRate != 0.5 {
		t.Errorf("LossRate want 0.5, got %v", node.LossRate)
	}
	if node.Colo != "SIN" {
		t.Errorf("Colo want SIN, got %q", node.Colo)
	}
	if node.Samples != 1 {
		t.Errorf("Samples want 1, got %d", node.Samples)
	}
}

func TestPoolUpdateSmoothsWithEWMA(t *testing.T) {
	p := NewPool(testConfig())
	p.TryAdd("1.1.1.1", 0.5, "")

	p.Update("1.1.1.1", SampleResult{AvgLatency: 0.5, Success: 5, Total: 5})
	p.Update("1.1.1.1", SampleResult{AvgLatency: 0.1, Success: 5, Total: 5})

	node, _ := p.Get("1.1.1.1")
	// α ≈ 0.095：单个样本只应小幅拉动均值
	if node.AvgLatency < 0.45 || node.AvgLatency > 0.47 {
		t.Errorf("EWMA 应小幅变动, got %v", node.AvgLatency)
	}
	if node.Samples != 2 {
		t.Errorf("Samples want 2, got %d", node.Samples)
	}
}

func TestPoolUpdateKeepsLatencyOnFailedRound(t *testing.T) {
	p := NewPool(testConfig())
	p.TryAdd("1.1.1.1", 0.1, "")

	p.Update("1.1.1.1", SampleResult{AvgLatency: 0.1, Success: 5, Total: 5})
	p.Update("1.1.1.1", SampleResult{AvgLatency: -1, LossRate: 1, Success: 0, Total: 5})

	node, _ := p.Get("1.1.1.1")
	if node.Latency != 0.1 {
		t.Errorf("整轮失败不应清空已有延迟, got %v", node.Latency)
	}
	if node.FailStreak != 1 {
		t.Errorf("FailStreak want 1, got %d", node.FailStreak)
	}
	if node.LossRate <= 0.09 || node.LossRate >= 0.11 {
		t.Errorf("丢包率应按 EWMA 上升, got %v", node.LossRate)
	}
}

func TestPoolUpdateMissingNode(t *testing.T) {
	p := NewPool(testConfig())
	if p.Update("9.9.9.9", SampleResult{}) {
		t.Error("不存在的节点应返回 false")
	}
}

func TestPoolIsolateAndRecover(t *testing.T) {
	p := NewPool(testConfig())
	p.TryAdd("1.1.1.1", 0.1, "")

	if !p.Isolate("1.1.1.1") {
		t.Fatal("首次隔离应成功")
	}
	if p.Isolate("1.1.1.1") {
		t.Error("重复隔离应返回 false")
	}
	if p.ActivePrimaryCount() != 0 {
		t.Errorf("隔离后不应计入可用, got %d", p.ActivePrimaryCount())
	}
	if !p.Contains("1.1.1.1") {
		t.Error("隔离的节点仍应留在池中")
	}

	if !p.Recover("1.1.1.1") {
		t.Fatal("解除隔离应成功")
	}
	if p.ActivePrimaryCount() != 1 {
		t.Errorf("恢复后应计入可用, got %d", p.ActivePrimaryCount())
	}
}

func TestPoolPickPrefersPrimaryAndFallsBack(t *testing.T) {
	p := NewPool(testConfig())

	if got := p.Pick(); got != nil {
		t.Fatalf("空池应返回 nil, got %v", got)
	}

	p.TryAdd("1.1.1.1", 0.3, "")
	p.TryAdd("2.2.2.2", 0.1, "")
	p.TryAdd("3.3.3.3", 0.2, "") // 备用

	for i := 0; i < 20; i++ {
		if got := p.Pick(); got == nil || got.IP == "3.3.3.3" {
			t.Fatalf("主选非空时应只从主选挑, got %v", got)
		}
	}

	p.Remove("1.1.1.1")
	p.Remove("2.2.2.2")
	if got := p.Pick(); got == nil || got.IP != "3.3.3.3" {
		t.Fatalf("主选空后应退到备用, got %v", got)
	}
}

func TestPoolPickSkipsIsolatedNode(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 2, BackupSize: 0, Cooldown: time.Second})
	p.TryAdd("1.1.1.1", 0.1, "")
	p.TryAdd("2.2.2.2", 0.2, "")
	p.Isolate("1.1.1.1")

	for i := 0; i < 10; i++ {
		if got := p.Pick(); got == nil || got.IP != "2.2.2.2" {
			t.Fatalf("应跳过隔离节点, got %v", got)
		}
	}

	// 全部隔离时退而选一个，避免完全没有出口
	p.Isolate("2.2.2.2")
	if got := p.Pick(); got == nil {
		t.Fatal("全部隔离时应兜底返回一个节点")
	}
}

func TestPoolTryEvictAllowsWhenPlentyActive(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 4, BackupSize: 1, Cooldown: time.Second})
	p.TryAdd("1.1.1.1", 0.1, "")
	p.TryAdd("2.2.2.2", 0.1, "")
	p.TryAdd("3.3.3.3", 0.1, "")
	p.TryAdd("4.4.4.4", 0.1, "")

	if !p.TryEvict() {
		t.Fatal("可用数充足时应直接放行")
	}
	if p.ActivePrimaryCount() != 4 {
		t.Error("放行不应改变池内容")
	}
}

func TestPoolTryEvictPromotesFromBackupWhenLow(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 4, BackupSize: 1, Cooldown: time.Second})
	p.TryAdd("1.1.1.1", 0.1, "")
	p.TryAdd("2.2.2.2", 0.1, "")
	p.TryAdd("3.3.3.3", 0.1, "")
	p.TryAdd("4.4.4.4", 0.1, "")
	p.TryAdd("5.5.5.5", 0.1, "") // 备用

	p.Isolate("1.1.1.1")
	p.Isolate("2.2.2.2")

	if !p.TryEvict() {
		t.Fatal("备用有可用节点时应允许淘汰")
	}
	if !p.Contains("5.5.5.5") {
		t.Fatal("备用节点应被提升")
	}
	if p.Contains("1.1.1.1") {
		t.Error("主选满员时应腾出一个隔离节点")
	}
}

func TestPoolTryEvictRejectedWithoutBackup(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 2, BackupSize: 0, Cooldown: time.Second})
	p.TryAdd("1.1.1.1", 0.1, "")
	p.TryAdd("2.2.2.2", 0.1, "")
	p.Isolate("1.1.1.1")

	if p.TryEvict() {
		t.Fatal("可用数触底且无备用时应拒绝淘汰")
	}
}

func TestPoolPromote(t *testing.T) {
	p := NewPool(testConfig())
	p.TryAdd("1.1.1.1", 0.1, "") // 主选
	p.TryAdd("2.2.2.2", 0.2, "") // 主选
	p.TryAdd("3.3.3.3", 0.3, "") // 备用

	if p.Promote() {
		t.Fatal("主选已满时不应提升")
	}

	p.Remove("1.1.1.1")
	if !p.Promote() {
		t.Fatal("主选有空缺时应提升成功")
	}
	primary, backup := p.Snapshot()
	if len(primary) != 2 || len(backup) != 0 {
		t.Fatalf("提升后 want 2/0, got %d/%d", len(primary), len(backup))
	}
}

func TestPoolPromotePicksBestBackup(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 2, Cooldown: time.Second})
	p.TryAdd("1.1.1.1", 0.5, "") // 主选
	p.TryAdd("2.2.2.2", 0.4, "") // 备用，较差
	p.TryAdd("3.3.3.3", 0.1, "") // 备用，最优

	p.Remove("1.1.1.1")
	if !p.Promote() {
		t.Fatal("应提升成功")
	}

	primary, backup := p.Snapshot()
	if primary[0].IP != "3.3.3.3" {
		t.Fatalf("应提升最优的备用节点, got %s", primary[0].IP)
	}
	if len(backup) != 1 || backup[0].IP != "2.2.2.2" {
		t.Fatalf("剩余备用应为 2.2.2.2, got %v", backup)
	}
}

func TestPoolSnapshotSortedByLatency(t *testing.T) {
	p := NewPool(testConfig())
	p.TryAdd("1.1.1.1", 0.3, "")
	p.TryAdd("2.2.2.2", 0.1, "")
	p.TryAdd("3.3.3.3", 0.2, "")

	primary, _ := p.Snapshot()
	if primary[0].IP != "2.2.2.2" || primary[1].IP != "1.1.1.1" {
		t.Fatalf("主选应按延迟升序, got %s %s", primary[0].IP, primary[1].IP)
	}
}

func TestPoolSnapshotPutsIsolatedLast(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 3, BackupSize: 0, Cooldown: time.Second})
	p.TryAdd("1.1.1.1", 0.1, "")
	p.TryAdd("2.2.2.2", 0.2, "")
	p.TryAdd("3.3.3.3", 0.3, "")
	p.Isolate("1.1.1.1")

	primary, _ := p.Snapshot()
	if primary[len(primary)-1].IP != "1.1.1.1" {
		t.Fatalf("隔离节点应排到最后, got %s", primary[len(primary)-1].IP)
	}
}

func TestPoolReplaceWorstSwapsBetterNode(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 2, BackupSize: 0, Cooldown: time.Minute})
	p.TryAdd("1.1.1.1", 0.1, "")
	p.TryAdd("2.2.2.2", 0.5, "") // 最差

	replaced, ok := p.ReplaceWorst("3.3.3.3", 0.12, "HKG")
	if !ok {
		t.Fatal("更优的节点应替换成功")
	}
	if replaced != "2.2.2.2" {
		t.Fatalf("应替换掉最差的 2.2.2.2, got %s", replaced)
	}
	if !p.Contains("3.3.3.3") || p.Contains("2.2.2.2") {
		t.Fatal("池内容应为 1.1.1.1 + 3.3.3.3")
	}
	// 被替换者进冷却，避免下一轮又被捞回来
	if got := p.TryAdd("2.2.2.2", 0.5, ""); got != InCooldown {
		t.Fatalf("被替换的节点应进入冷却, got %v", got)
	}
}

func TestPoolReplaceWorstRejectsWhenNotBetter(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 2, BackupSize: 0, Cooldown: time.Minute})
	p.TryAdd("1.1.1.1", 0.1, "")
	p.TryAdd("2.2.2.2", 0.2, "")

	// 延迟差不足 20ms 且丢包相同 → 视为等价，不替换
	if _, ok := p.ReplaceWorst("3.3.3.3", 0.19, ""); ok {
		t.Fatal("差异不显著时不应替换")
	}
	if p.Contains("3.3.3.3") {
		t.Error("未替换时不应把新节点放入池中")
	}
}

func TestPoolReplaceWorstPrefersIsolated(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 2, BackupSize: 0, Cooldown: time.Minute})
	p.TryAdd("1.1.1.1", 0.05, "")
	p.TryAdd("2.2.2.2", 0.06, "")
	p.Isolate("2.2.2.2")

	// 隔离节点不承载流量，应优先被换掉，哪怕它的延迟更好
	replaced, ok := p.ReplaceWorst("3.3.3.3", 0.1, "")
	if !ok || replaced != "2.2.2.2" {
		t.Fatalf("应优先替换隔离节点, got %q ok=%v", replaced, ok)
	}
}

func TestPoolReplaceWorstSkipsKnownAndCooling(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 2, BackupSize: 0, Cooldown: time.Minute})
	p.TryAdd("1.1.1.1", 0.1, "")
	p.TryAdd("2.2.2.2", 0.5, "")

	if _, ok := p.ReplaceWorst("1.1.1.1", 0.01, ""); ok {
		t.Error("已在池中的候选不应触发替换")
	}

	p.Remove("1.1.1.1") // 进冷却
	if _, ok := p.ReplaceWorst("1.1.1.1", 0.01, ""); ok {
		t.Error("冷却中的候选不应触发替换")
	}
}

func TestCompareEvictionThresholds(t *testing.T) {
	base := Node{AvgLatency: 0.2, LossRate: 0}

	// 延迟差不足 20ms → 不比延迟，比丢包
	near := Node{AvgLatency: 0.205, LossRate: 0.05}
	if CompareEviction(near, base) <= 0 {
		t.Error("延迟接近时应按丢包判定，丢包高者更差")
	}

	// 延迟差超过 20ms → 延迟高者更差
	slow := Node{AvgLatency: 0.3, LossRate: 0}
	if CompareEviction(slow, base) <= 0 {
		t.Error("延迟差显著时，延迟高者更差")
	}

	// 隔离节点最差
	isolated := Node{AvgLatency: 0.05, Isolated: true}
	if CompareEviction(isolated, base) <= 0 {
		t.Error("隔离节点应被视为最差")
	}

	// 各项都在阈值内 → 视为等价
	same := Node{AvgLatency: 0.205, LossRate: 0.005}
	if CompareEviction(same, base) != 0 {
		t.Error("差异都在阈值内时应返回 0")
	}
}

func TestMinActiveTarget(t *testing.T) {
	cases := map[int]int{1: 1, 2: 1, 3: 2, 4: 2, 10: 5}
	for size, want := range cases {
		if got := minActiveTarget(size); got != want {
			t.Errorf("minActiveTarget(%d) = %d, want %d", size, got, want)
		}
	}
}
