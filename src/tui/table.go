package tui

import (
	"fmt"

	"github.com/LennyFace24/CFip-go/src/core"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

// newTable 创建带列定义的空表格
func newTable() table.Model {
	columns := []table.Column{
		{Title: "IP", Width: 16},
		{Title: "延迟(ms)", Width: 12},
		{Title: "状态", Width: 8},
		{Title: "来源", Width: 12},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithHeight(10),
		table.WithFocused(true),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	t.SetStyles(s)
	return t
}

// rowFromResult 把单条结果转为表格行
func rowFromResult(r core.StreamResult) table.Row {
	lat := "-"
	status := "失败"
	if r.Latency >= 0 {
		lat = fmt.Sprintf("%.3f", r.Latency)
		status = "成功"
	}
	return table.Row{r.IP.IP, lat, status, r.Source}
}
