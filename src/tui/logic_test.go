package tui

import (
	"testing"

	"github.com/LennyFace24/CFip-go/src/core"
)

func mkResult(ip string, latency float64) core.StreamResult {
	return core.StreamResult{IP: core.IP{IP: ip}, Latency: latency}
}

func TestSortLatency(t *testing.T) {
	rows := []core.StreamResult{
		mkResult("3.3.3.3", 30),
		mkResult("1.1.1.1", -1),
		mkResult("2.2.2.2", 10),
	}
	got := SortResults(rows, SortLatency)
	wantIPs := []string{"2.2.2.2", "3.3.3.3", "1.1.1.1"}
	for i, ip := range wantIPs {
		if got[i].IP.IP != ip {
			t.Errorf("pos %d: want %s, got %s", i, ip, got[i].IP.IP)
		}
	}
}

func TestSortIP(t *testing.T) {
	rows := []core.StreamResult{
		mkResult("104.16.9.1", 30),
		mkResult("104.16.1.1", -1),
		mkResult("104.16.2.1", 10),
	}
	got := SortResults(rows, SortIP)
	wantIPs := []string{"104.16.1.1", "104.16.2.1", "104.16.9.1"}
	for i, ip := range wantIPs {
		if got[i].IP.IP != ip {
			t.Errorf("pos %d: want %s, got %s", i, ip, got[i].IP.IP)
		}
	}
}

func TestSortStatus(t *testing.T) {
	rows := []core.StreamResult{
		mkResult("3.3.3.3", 30),
		mkResult("1.1.1.1", -1),
		mkResult("2.2.2.2", 10),
	}
	got := SortResults(rows, SortStatus)
	if got[0].IP.IP != "3.3.3.3" || got[1].IP.IP != "2.2.2.2" || got[2].IP.IP != "1.1.1.1" {
		t.Errorf("status sort wrong: %v %v %v", got[0].IP.IP, got[1].IP.IP, got[2].IP.IP)
	}
}

func TestFilterResults(t *testing.T) {
	rows := []core.StreamResult{
		mkResult("104.16.1.1", 30),
		mkResult("104.24.2.2", -1),
		mkResult("1.1.1.1", 10),
	}
	got := FilterResults(rows, "104.16")
	if len(got) != 1 || got[0].IP.IP != "104.16.1.1" {
		t.Fatalf("want only 104.16.1.1, got %v", got)
	}
	if len(FilterResults(rows, "")) != 3 {
		t.Error("empty query must return all")
	}
	if len(FilterResults(rows, "1.1.1.1")) != 1 {
		t.Error("full IP match must work")
	}
}
