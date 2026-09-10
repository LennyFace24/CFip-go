package core

import (
	"context"
	"math"
	"testing"
	"time"
)

// probeWithFailures 返回前 failFirst 次失败、其余成功的探测函数
func probeWithFailures(failFirst int, latency float64, colo string) ProbeLatency {
	count := 0
	return func(IP) ProbeResult {
		count++
		if count <= failFirst {
			return ProbeResult{Latency: -1}
		}
		return ProbeResult{Latency: latency, Colo: colo}
	}
}

func assertFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: want %v, got %v", name, want, got)
	}
}

func TestProbeRepeatedAllSuccess(t *testing.T) {
	got := ProbeRepeated(context.Background(), IP{IP: "1.1.1.1"}, 3, 0, probeWithFailures(0, 0.1, "HKG"))
	if got.Total != 3 || got.Success != 3 {
		t.Fatalf("want 3/3, got %d/%d", got.Success, got.Total)
	}
	assertFloat(t, "AvgLatency", got.AvgLatency, 0.1)
	assertFloat(t, "LossRate", got.LossRate, 0)
	if got.Colo != "HKG" {
		t.Errorf("want colo HKG, got %q", got.Colo)
	}
}

func TestProbeRepeatedAllFailed(t *testing.T) {
	got := ProbeRepeated(context.Background(), IP{IP: "1.1.1.1"}, 2, 0, probeWithFailures(99, 0, ""))
	if got.Total != 2 || got.Success != 0 {
		t.Fatalf("want 0/2, got %d/%d", got.Success, got.Total)
	}
	assertFloat(t, "AvgLatency", got.AvgLatency, -1)
	assertFloat(t, "LossRate", got.LossRate, 1)
}

func TestProbeRepeatedPartialFailure(t *testing.T) {
	// 4 次中前 1 次失败 → 丢包率 0.25，均值只算成功的 3 次
	got := ProbeRepeated(context.Background(), IP{IP: "1.1.1.1"}, 4, 0, probeWithFailures(1, 0.2, "NRT"))
	if got.Total != 4 || got.Success != 3 {
		t.Fatalf("want 3/4, got %d/%d", got.Success, got.Total)
	}
	assertFloat(t, "AvgLatency", got.AvgLatency, 0.2)
	assertFloat(t, "LossRate", got.LossRate, 0.25)
	if got.Colo != "NRT" {
		t.Errorf("want colo NRT, got %q", got.Colo)
	}
}

func TestProbeRepeatedCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := ProbeRepeated(ctx, IP{IP: "1.1.1.1"}, 5, 0, probeWithFailures(0, 0.1, ""))
	if got.Total != 0 {
		t.Fatalf("want 0 samples after cancel, got %d", got.Total)
	}
	assertFloat(t, "AvgLatency", got.AvgLatency, -1)
}

func TestProbeRepeatedTimesLessThanOne(t *testing.T) {
	got := ProbeRepeated(context.Background(), IP{IP: "1.1.1.1"}, 0, 0, probeWithFailures(0, 0.1, ""))
	if got.Total != 1 {
		t.Fatalf("want 1 sample, got %d", got.Total)
	}
}

func TestProbeRepeatedRespectsGap(t *testing.T) {
	// 3 次采样、间隔 20ms，总耗时不应低于 40ms
	start := time.Now()
	ProbeRepeated(context.Background(), IP{IP: "1.1.1.1"}, 3, 20*time.Millisecond, probeWithFailures(0, 0.01, ""))
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("gap not respected: elapsed %v", elapsed)
	}
}
