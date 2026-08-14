package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/LennyFace24/CFip-go/src/config"
	"github.com/LennyFace24/CFip-go/src/core"
)
// newTestModel 构造测试 Model（不启动测速流，probe 用 fake 避免真实网络/配置）
func newTestModel() *Model {
	cfg := &config.Config{Latency: 500, Concurrency: 2, Timeout: 500, Number: 10}
	ips := []core.IP{{IP: "1.1.1.1"}, {IP: "2.2.2.2"}}
	m := New(cfg, ips)
	m.probe = func(ip core.IP) float64 { return 1.0 }
	return m
}

func TestUpdateToggleSort(t *testing.T) {
	m := newTestModel()
	if m.sort != SortLatency {
		t.Fatalf("initial sort want SortLatency, got %v", m.sort)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(*Model)
	if m.sort != SortIP {
		t.Errorf("after s want SortIP, got %v", m.sort)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(*Model)
	if m.sort != SortStatus {
		t.Errorf("after second s want SortStatus, got %v", m.sort)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(*Model)
	if m.sort != SortLatency {
		t.Errorf("after third s want wrap to SortLatency, got %v", m.sort)
	}
}

func TestUpdateFilterFlow(t *testing.T) {
	m := newTestModel()
	// 注入一条结果便于验证过滤
	m.rows = []core.StreamResult{
		{IP: core.IP{IP: "104.16.1.1"}, Latency: 10},
		{IP: core.IP{IP: "1.1.1.1"}, Latency: -1},
	}
	m.setResults()

	// 按 / 激活输入框
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = next.(*Model)
	if !m.filtering {
		t.Fatal("after / want filtering=true")
	}

	// 输入 '1'，实时过滤
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = next.(*Model)
	if m.query != "1" {
		t.Errorf("after typing want query=1, got %q", m.query)
	}
	if len(m.table.Rows()) != 2 {
		t.Errorf("filter '1' want 2 rows (104.16.1.1, 1.1.1.1), got %d", len(m.table.Rows()))
	}

	// enter 确认
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*Model)
	if m.filtering {
		t.Error("after enter want filtering=false")
	}

	// 再按 / 输入不匹配词
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = next.(*Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}})
	m = next.(*Model)
	if len(m.table.Rows()) != 0 {
		t.Errorf("filter '9' want 0 rows, got %d", len(m.table.Rows()))
	}
	// esc 取消筛选，恢复全部
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(*Model)
	if m.filtering {
		t.Error("after esc want filtering=false")
	}
	if len(m.table.Rows()) != 2 {
		t.Errorf("after esc clear want 2 rows, got %d", len(m.table.Rows()))
	}
}

func TestUpdateRestart(t *testing.T) {
	m := newTestModel()
	m.rows = []core.StreamResult{{IP: core.IP{IP: "1.1.1.1"}, Latency: 10}}
	m.ok, m.fail, m.done = 1, 2, true

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("after r want non-nil cmd (restart subscription)")
	}
	if m.done {
		t.Error("after r want done=false")
	}
	if len(m.rows) != 0 || m.ok != 0 || m.fail != 0 {
		t.Errorf("after r want reset rows/ok/fail, got rows=%d ok=%d fail=%d", len(m.rows), m.ok, m.fail)
	}
	// 清理 restart 启动的测速流（probe 为 nil 会走默认探测，测试中 StreamLatency 已启动但无网络流量，cancel 终止）
	m.cancel()
}
