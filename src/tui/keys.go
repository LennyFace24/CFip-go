package tui

import "github.com/charmbracelet/lipgloss"

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

// helpLine 渲染底部帮助栏
func helpLine() string {
	return helpStyle.Render("↑/↓ 导航  s 排序  / 筛选  r 重测  q 退出")
}
