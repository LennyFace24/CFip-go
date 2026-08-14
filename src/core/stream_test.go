package core

import (
	"context"
	"sort"
	"testing"
)

// fakeProbe: 2.2.2.2 失败(-1)，其余成功(12.5ms)
func fakeProbe(ip IP) float64 {
	if ip.IP == "2.2.2.2" {
		return -1
	}
	return 12.5
}

func TestStreamLatencyEmitsAllResults(t *testing.T) {
	ips := []IP{
		{IP: "1.1.1.1", isCIDR: false},
		{IP: "2.2.2.2", isCIDR: true},
		{IP: "3.3.3.3", isCIDR: true},
	}
	ctx := context.Background()
	ch := StreamLatency(ctx, ips, 2, fakeProbe)

	var got []StreamResult
	for r := range ch {
		got = append(got, r)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 results, got %d", len(got))
	}
	// 按 IP 排序后断言，规避并发顺序
	sort.Slice(got, func(i, j int) bool { return got[i].IP.IP < got[j].IP.IP })
	if got[0].Latency != 12.5 || got[0].Source != "单IP" {
		t.Errorf("1.1.1.1: want latency 12.5 source 单IP, got %v %q", got[0].Latency, got[0].Source)
	}
	if got[1].Latency != -1 || got[1].Source != "网段采样" {
		t.Errorf("2.2.2.2: want latency -1 source 网段采样, got %v %q", got[1].Latency, got[1].Source)
	}
	if got[2].Latency != 12.5 {
		t.Errorf("3.3.3.3: want latency 12.5, got %v", got[2].Latency)
	}
}

func TestStreamLatencyStopsOnCancel(t *testing.T) {
	ips := make([]IP, 100)
	for i := range ips {
		ips[i] = IP{IP: "1.1.1.1", isCIDR: false}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	ch := StreamLatency(ctx, ips, 4, fakeProbe)

	count := 0
	for range ch {
		count++
	}
	if count != 0 {
		t.Fatalf("want 0 results after cancel, got %d", count)
	}
}
