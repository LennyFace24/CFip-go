package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

var inputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

// newInput 创建筛选输入框
func newInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "输入 IP 子串筛选，Enter 确认，Esc 取消"
	ti.CharLimit = 64
	ti.Width = 40
	return ti
}

// inputLine 渲染输入框所在行
func inputLine(ti textinput.Model) string {
	return inputStyle.Render("筛选: " + ti.View())
}
