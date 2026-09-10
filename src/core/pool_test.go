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

func TestPoolUpdateWritesSample(t *testing.T) {
	p := NewPool(testConfig())
	p.TryAdd("1.1.1.1", 0.1, "")

	sample := SampleResult{AvgLatency: 0.25, LossRate: 0.5, Success: 1, Total: 2, Colo: "SIN"}
	if !p.Update("1.1.1.1", sample) {
		t.Fatal("已存在节点应更新成功")
	}

	node := p.All()[0]
	if node.AvgLatency != 0.25 {
		t.Errorf("AvgLatency: want 0.25, got %v", node.AvgLatency)
	}
	if node.LossRate != 0.5 {
		t.Errorf("LossRate: want 0.5, got %v", node.LossRate)
	}
	if node.Colo != "SIN" {
		t.Errorf("Colo: want SIN, got %q", node.Colo)
	}

	failed := SampleResult{AvgLatency: -1, LossRate: 1, Success: 0, Total: 3}
	p.Update("1.1.1.1", failed)
	if node := p.All()[0]; node.FailStreak != 1 {
		t.Errorf("整轮失败后 FailStreak 应为 1, got %d", node.FailStreak)
	}

	p.Update("1.1.1.1", sample)
	if node := p.All()[0]; node.FailStreak != 0 {
		t.Errorf("成功一轮后 FailStreak 应清零, got %d", node.FailStreak)
	}
}

func TestPoolUpdateMissingNode(t *testing.T) {
	p := NewPool(testConfig())
	if p.Update("9.9.9.9", SampleResult{}) {
		t.Error("不存在的节点应返回 false")
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

func TestPoolSnapshotPutsFailedLast(t *testing.T) {
	p := NewPool(PoolConfig{PrimarySize: 3, BackupSize: 0, Cooldown: time.Second})
	p.TryAdd("1.1.1.1", 0.1, "")
	p.TryAdd("2.2.2.2", 0.2, "")
	p.TryAdd("3.3.3.3", 0.3, "")

	// 让延迟最低的节点整轮失败
	p.Update("1.1.1.1", SampleResult{AvgLatency: -1, LossRate: 1, Total: 3})

	primary, _ := p.Snapshot()
	if primary[len(primary)-1].IP != "1.1.1.1" {
		t.Fatalf("整轮失败的节点应排到最后, got %s", primary[len(primary)-1].IP)
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
	if !p.Contains("3.3.3.3") {
		t.Fatal("3.3.3.3 应仍在池中")
	}
	primary, backup := p.Snapshot()
	if len(primary) != 2 || len(backup) != 0 {
		t.Fatalf("提升后 want 2/0, got %d/%d", len(primary), len(backup))
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
