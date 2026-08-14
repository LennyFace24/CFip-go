package main

import (
	"fmt"
	"os"

	"github.com/LennyFace24/CFip-go/src/config"
	"github.com/LennyFace24/CFip-go/src/core"
	"github.com/LennyFace24/CFip-go/src/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println("加载配置文件出错:", err)
		os.Exit(1)
	}
	file, err := os.ReadFile("ip.txt")
	if err != nil {
		fmt.Println("读取 ip.txt 出错:", err)
		os.Exit(1)
	}
	ips, err := core.NewIPParser().ParseIP(string(file))
	if err != nil {
		fmt.Println("解析 IP 出错:", err)
		os.Exit(1)
	}

	m := tui.New(cfg, ips)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("TUI 运行出错:", err)
		os.Exit(1)
	}
}
