package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // 绿
	failStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // 红
	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// statusLine 渲染顶部状态栏
func (m *Model) statusLine() string {
	progress := "等待开始"
	if m.done {
		progress = "已完成"
	} else if m.started {
		progress = "测速中"
	}
	status := fmt.Sprintf("%s  进度 %d/%d  成功 %s  失败 %s",
		progress, len(m.rows), m.total,
		okStyle.Render(fmt.Sprintf("%d", m.ok)),
		failStyle.Render(fmt.Sprintf("%d", m.fail)),
	)
	sortName := map[SortMode]string{SortLatency: "延迟", SortIP: "IP", SortStatus: "状态"}[m.sort]
	extra := dimStyle.Render(fmt.Sprintf("  排序: %s", sortName))
	if m.query != "" {
		extra += dimStyle.Render(fmt.Sprintf("  筛选: %q", m.query))
	}
	return status + extra
}
