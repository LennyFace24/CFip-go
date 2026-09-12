package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/LennyFace24/CFip-go/src/config"
	"github.com/LennyFace24/CFip-go/src/core"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// logFileName 日志固定文件名：每次测速覆盖上一次，避免日志无限堆积。
const logFileName = "speed-latest.txt"

// LogDir 返回日志目录（位于配置目录下），不存在时自动创建。
func LogDir() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}
	return logDir, nil
}

func isQualified(r SpeedResult, latencyLimit int) bool {
	return r.Latency >= 0 && r.Latency <= float64(latencyLimit)
}

// resultRank 排序权重：达标 → 超标 → 机房不符 → 失败
func resultRank(r SpeedResult, cfg *config.Config, whitelist []string) int {
	switch {
	case r.Latency < 0:
		return 3
	case !core.ColoAllowed(r.Colo, whitelist):
		return 2
	case isQualified(r, cfg.Latency):
		return 0
	default:
		return 1
	}
}

// WriteSpeedLog 把一次测速结果写入固定的 speed-latest.txt（覆盖上一次）。
// 内容按「达标 → 超标 → 机房不符 → 失败」排序，同组内延迟升序。
func WriteSpeedLog(rows []SpeedResult, summary SpeedSummary, cfg *config.Config) (string, error) {
	dir, err := LogDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, logFileName)

	whitelist := core.SplitColos(cfg.Colo)
	sorted := make([]SpeedResult, len(rows))
	copy(sorted, rows)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri, rj := resultRank(sorted[i], cfg, whitelist), resultRank(sorted[j], cfg, whitelist)
		if ri != rj {
			return ri < rj
		}
		if sorted[i].Latency < 0 || sorted[j].Latency < 0 {
			return sorted[i].Latency > sorted[j].Latency
		}
		return sorted[i].Latency < sorted[j].Latency
	})

	coloNote := strings.Join(whitelist, " ")
	if coloNote == "" {
		coloNote = "不过滤"
	}

	var b strings.Builder
	b.WriteString("CFip 测速日志\n")
	fmt.Fprintf(&b, "生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "延迟上限: %d ms    目标数量: %d    并发: %d    超时: %d ms\n",
		cfg.Latency, cfg.Number, cfg.Concurrency, cfg.Timeout)
	fmt.Fprintf(&b, "机房白名单: %s\n", coloNote)
	fmt.Fprintf(&b, "探测: %d    达标: %d    超标: %d    失败: %d    机房不符: %d    耗时: %d ms    %s\n",
		summary.Total, summary.Qualified, summary.OverLimit, summary.Failed, summary.Excluded,
		summary.Elapsed, map[bool]string{true: "提前结束", false: "全部跑完"}[summary.Stopped])
	b.WriteString("\n")

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "IP\t延迟(ms)\t状态\t机房\t来源")
	for _, r := range sorted {
		latency := "-"
		state := "失败"
		switch {
		case !core.ColoAllowed(r.Colo, whitelist):
			state = "机房不符"
		case r.Latency >= 0:
			latency = fmt.Sprintf("%.1f", r.Latency)
			state = "超标"
			if isQualified(r, cfg.Latency) {
				state = "达标"
			}
		}
		colo := r.Colo
		if colo == "" {
			colo = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.IP, latency, state, colo, r.Source)
	}
	if err := w.Flush(); err != nil {
		return "", err
	}

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// LogService 提供日志目录访问、导出与打开能力。
type LogService struct {
	app *application.App
}

func (s *LogService) ServiceName() string { return "LogService" }

// Dir 返回日志目录路径
func (s *LogService) Dir() (string, error) {
	return LogDir()
}

// Export 弹出系统保存对话框，把内容写入用户指定的文件，返回实际路径。
func (s *LogService) Export(content string, filename string) (string, error) {
	if s.app == nil {
		return "", errors.New("应用尚未初始化")
	}
	dir, err := LogDir()
	if err != nil {
		return "", err
	}
	if filename == "" {
		filename = fmt.Sprintf("cfip-%s.txt", time.Now().Format("20060102-150405"))
	}
	path, err := s.app.Dialog.SaveFile().
		SetMessage("导出测速日志").
		SetDirectory(dir).
		SetFilename(filename).
		SetButtonText("保存").
		AddFilter("文本文件 (*.txt)", "*.txt").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", errors.New("已取消导出")
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// OpenDir 用系统文件管理器打开日志目录
func (s *LogService) OpenDir() error {
	if s.app == nil {
		return errors.New("应用尚未初始化")
	}
	dir, err := LogDir()
	if err != nil {
		return err
	}
	return s.app.Browser.OpenFile(dir)
}
