package core

import (
	"math"
	"testing"
)

func newTestBandit() *UCBBandit {
	return NewUCBBandit([]string{"1.1.1.1", "2.2.2.2", "3.3.3.3"})
}

func TestUCBSelectPrefersUnobserved(t *testing.T) {
	u := newTestBandit()

	// 1.1.1.1 已经测得极好，但 2.2.2.2 还没探过 → 应优先未测的
	u.Observe("1.1.1.1", 0.001, true)
	u.Observe("1.1.1.1", 0.001, true)

	got := u.Select(1)
	if len(got) != 1 || got[0] != "2.2.2.2" {
		t.Fatalf("want 2.2.2.2（未探测优先）, got %v", got)
	}
}

func TestUCBSelectAllUnobservedWhenTotalZero(t *testing.T) {
	u := newTestBandit()

	got := u.Select(3)
	if len(got) != 3 {
		t.Fatalf("want 3, got %v", got)
	}
	// 全未观察时按候选原顺序返回（稳定排序）
	for i, want := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		if got[i] != want {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
}

func TestUCBSelectExploresRarelySampled(t *testing.T) {
	u := NewUCBBandit([]string{"1.1.1.1", "2.2.2.2"})

	// 两者延迟相同，但 2.2.2.2 样本更少 → 探索项更大 → 应当优先
	for i := 0; i < 5; i++ {
		u.Observe("1.1.1.1", 0.1, true)
	}
	u.Observe("2.2.2.2", 0.1, true)

	got := u.Select(1)
	if len(got) != 1 || got[0] != "2.2.2.2" {
		t.Fatalf("want 2.2.2.2（样本少应优先）, got %v", got)
	}
}

func TestUCBSelectPrefersFasterWhenSamplesEqual(t *testing.T) {
	u := NewUCBBandit([]string{"1.1.1.1", "2.2.2.2"})

	for i := 0; i < 5; i++ {
		u.Observe("1.1.1.1", 0.3, true)
		u.Observe("2.2.2.2", 0.05, true)
	}

	got := u.Select(1)
	if len(got) != 1 || got[0] != "2.2.2.2" {
		t.Fatalf("want 2.2.2.2（延迟更低）, got %v", got)
	}
}

func TestUCBSelectRespectsCount(t *testing.T) {
	u := newTestBandit()

	if got := u.Select(0); got != nil {
		t.Fatalf("n=0 应返回 nil, got %v", got)
	}
	if got := u.Select(10); len(got) != 3 {
		t.Fatalf("n 超过候选数时应截断, got %v", got)
	}

	got := u.Select(2)
	if len(got) != 2 || got[0] == got[1] {
		t.Fatalf("应返回 2 个互不相同的候选, got %v", got)
	}
}

func TestUCBObserveFirstSampleSetsMean(t *testing.T) {
	u := newTestBandit()
	u.Observe("1.1.1.1", 0.25, true)

	mean, count, ok := u.Stats("1.1.1.1")
	if !ok {
		t.Fatal("候选应存在")
	}
	if mean != 0.25 {
		t.Errorf("首个样本应直接赋值, got %v", mean)
	}
	if count != 1 {
		t.Errorf("count want 1, got %d", count)
	}
}

func TestUCBObserveDiscountsOldSamples(t *testing.T) {
	u := newTestBandit()

	for i := 0; i < 5; i++ {
		u.Observe("1.1.1.1", 0.1, true)
	}
	meanBefore, _, _ := u.Stats("1.1.1.1")
	if math.Abs(meanBefore-0.1) > 1e-9 {
		t.Fatalf("同值样本的均值应保持不变, got %v", meanBefore)
	}

	// 折算：0.1*0.9 + 0.5*0.1 = 0.14
	u.Observe("1.1.1.1", 0.5, true)
	meanAfter, _, _ := u.Stats("1.1.1.1")
	if meanAfter < 0.13 || meanAfter > 0.15 {
		t.Fatalf("新样本应小幅拉动均值, got %v", meanAfter)
	}
}

func TestUCBObserveFailureUsesPenalty(t *testing.T) {
	u := newTestBandit()
	u.Observe("1.1.1.1", -1, false)

	mean, count, _ := u.Stats("1.1.1.1")
	if mean != ucbFailPenalty {
		t.Errorf("失败应按惩罚值计入, got %v", mean)
	}
	if count != 1 {
		t.Errorf("失败也应计入样本数, got %d", count)
	}
}

func TestUCBObserveUnknownIP(t *testing.T) {
	u := newTestBandit()
	u.Observe("9.9.9.9", 0.1, true) // 不应 panic

	if u.Total() != 0 {
		t.Errorf("未知候选不应计入总数, got %d", u.Total())
	}
}

func TestUCBCountsAreCapped(t *testing.T) {
	u := newTestBandit()

	for i := 0; i < ucbMaxSamples+10; i++ {
		u.Observe("1.1.1.1", 0.1, true)
	}

	_, count, _ := u.Stats("1.1.1.1")
	if count != ucbMaxSamples {
		t.Fatalf("样本数应封顶在 %d, got %d", ucbMaxSamples, count)
	}
}

func TestUCBEmptyCandidates(t *testing.T) {
	u := NewUCBBandit(nil)
	if got := u.Select(3); got != nil {
		t.Fatalf("无候选时应返回 nil, got %v", got)
	}
}
