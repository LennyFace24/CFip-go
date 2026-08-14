package tui

import (
	"context"

	"github.com/LennyFace24/CFip-go/src/config"
	"github.com/LennyFace24/CFip-go/src/core"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// latencyMsg 单条测速结果到达
type latencyMsg core.StreamResult

// streamDoneMsg 测速流结束（channel 关闭）
type streamDoneMsg struct{}

// Model TUI 根模型
type Model struct {
	cfg    *config.Config
	ips    []core.IP
	ctx    context.Context
	cancel context.CancelFunc
	ch     <-chan core.StreamResult
	probe  core.ProbeLatency

	rows    []core.StreamResult // 全部已到达结果（原始顺序）
	total   int                 // 总待测数
	ok      int                 // 成功数
	fail    int                 // 失败数
	started bool
	done    bool

	sort      SortMode
	query     string
	filtering bool
	input     textinput.Model

	table table.Model
}

func New(cfg *config.Config, ips []core.IP) *Model {
	return &Model{
		cfg:   cfg,
		ips:   ips,
		total: len(ips),
		sort:  SortLatency,
		input: newInput(),
		table: newTable(),
	}
}

// Init 启动测速流并返回首个订阅命令
func (m *Model) Init() tea.Cmd {
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.ch = core.StreamLatency(m.ctx, m.ips, m.cfg.Concurrency, m.probe)
	m.started = true
	return subscribe(m.ch)
}

// subscribe 从 channel 读一条结果并转成消息；channel 关闭返回 streamDoneMsg
func subscribe(ch <-chan core.StreamResult) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return latencyMsg(r)
	}
}

// Update 处理消息
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// 筛选输入框激活时，优先交给输入框
		if m.filtering {
			switch msg.String() {
			case "enter":
				m.query = m.input.Value()
				m.filtering = false
				m.setResults()
				return m, nil
			case "esc", "ctrl+c":
				m.filtering = false
				m.input.SetValue("")
				m.query = ""
				m.setResults()
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.query = m.input.Value() // 实时过滤
			m.setResults()
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case "s":
			m.sort = (m.sort + 1) % 3 // SortLatency→SortIP→SortStatus 循环
			m.setResults()
			return m, nil
		case "/":
			m.filtering = true
			m.input.Focus()
			return m, nil
		case "r":
			return m, m.restart()
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case latencyMsg:
		r := core.StreamResult(msg)
		m.rows = append(m.rows, r)
		if r.Latency >= 0 {
			m.ok++
			if m.ok >= m.cfg.Number && m.cancel != nil {
				m.cancel() // 集满即停
			}
		} else {
			m.fail++
		}
		m.setResults()
		return m, subscribe(m.ch)
	case streamDoneMsg:
		m.done = true
		m.setResults()
		return m, nil
	}
	return m, nil
}

// setResults 重算排序/筛选并刷新表格
func (m *Model) setResults() {
	filtered := FilterResults(m.rows, m.query)
	sorted := SortResults(filtered, m.sort)
	rows := make([]table.Row, len(sorted))
	for i, r := range sorted {
		rows[i] = rowFromResult(r)
	}
	m.table.SetRows(rows)
}

// restart 重置结果并重启测速流
func (m *Model) restart() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.ch = core.StreamLatency(m.ctx, m.ips, m.cfg.Concurrency, m.probe)
	m.rows = nil
	m.ok, m.fail = 0, 0
	m.done = false
	m.started = true
	m.setResults()
	return subscribe(m.ch)
}

// View 渲染整个界面
func (m *Model) View() string {
	var inputLineStr string
	if m.filtering {
		inputLineStr = inputLine(m.input) + "\n"
	}
	return m.statusLine() + "\n" + inputLineStr + baseStyle.Render(m.table.View()) + "\n" + helpLine()
}
