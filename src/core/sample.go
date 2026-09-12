package core

import (
	"context"
	"time"
)

// SampleResult 单个 IP 多次采样的汇总。
type SampleResult struct {
	AvgLatency float64 // 平均延迟（秒），仅统计成功轮次；无成功轮次时为 -1
	LossRate   float64 // 丢包率 = 失败次数 / 总次数
	Success    int
	Total      int
	Colo       string // 任一轮拿到的机房代码，可能为空
}

// ProbeRepeated 对同一个 IP 连续采样 times 次，相邻两次间隔 gap。
//
// 失败轮次不计入均值，只计入 Total 与 LossRate——一次超时不代表节点慢，
// 但必须体现在丢包率里，否则「偶发不通」的节点会被误判为可用。
//
// 只要有一轮取到机房代码就记录，避免单次响应头缺失导致机房误判。
// ctx 取消时立即返回已采到的部分。times <= 0 按 1 处理。
func ProbeRepeated(
	ctx context.Context,
	ip IP,
	times int,
	gap time.Duration,
	probe ProbeLatency,
) SampleResult {
	if times <= 0 {
		times = 1
	}
	if probe == nil {
		client := newClient()
		probe = func(target IP) ProbeResult {
			return request(target, client)
		}
	}

	var result SampleResult
	var totalLatency float64

	for i := 0; i < times; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return finalize(result, totalLatency)
			case <-time.After(gap):
			}
		}
		if ctx.Err() != nil {
			return finalize(result, totalLatency)
		}

		r := probe(ip)
		result.Total++
		if r.Latency < 0 {
			continue
		}
		result.Success++
		totalLatency += r.Latency
		if result.Colo == "" {
			result.Colo = r.Colo
		}
	}

	return finalize(result, totalLatency)
}

func finalize(result SampleResult, totalLatency float64) SampleResult {
	if result.Success == 0 {
		result.AvgLatency = -1
	} else {
		result.AvgLatency = totalLatency / float64(result.Success)
	}
	if result.Total > 0 {
		result.LossRate = float64(result.Total-result.Success) / float64(result.Total)
	}
	return result
}
