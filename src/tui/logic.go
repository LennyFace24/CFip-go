package tui

import (
	"sort"
	"strings"

	"github.com/LennyFace24/CFip-go/src/core"
)

// SortMode 表格排序方式
type SortMode int

const (
	SortLatency SortMode = iota // 延迟升序，成功在前
	SortIP                      // IP 字典序
	SortStatus                  // 成功在前
)

// SortResults 按 mode 排序 rows，不修改原切片
func SortResults(rows []core.StreamResult, mode SortMode) []core.StreamResult {
	out := make([]core.StreamResult, len(rows))
	copy(out, rows)
	switch mode {
	case SortIP:
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].IP.IP < out[j].IP.IP
		})
	case SortStatus:
		sort.SliceStable(out, func(i, j int) bool {
			okI, okJ := out[i].Latency >= 0, out[j].Latency >= 0
			return okI && !okJ
		})
	default: // SortLatency
		sort.SliceStable(out, func(i, j int) bool {
			okI, okJ := out[i].Latency >= 0, out[j].Latency >= 0
			if okI != okJ {
				return okI // 成功在前
			}
			if !okI {
				return false // 都是失败，保持原序
			}
			return out[i].Latency < out[j].Latency
		})
	}
	return out
}

// FilterResults 按 IP 子串过滤（大小写不敏感）；空 query 返回全部
func FilterResults(rows []core.StreamResult, query string) []core.StreamResult {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return rows
	}
	var out []core.StreamResult
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.IP.IP), query) {
			out = append(out, r)
		}
	}
	return out
}
