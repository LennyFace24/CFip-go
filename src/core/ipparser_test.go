package core

import (
	"net/netip"
	"os"
	"testing"
)

// 1. 单 IP 解析: 分类正确, isCIDR=false
func TestParseSingleIP(t *testing.T) {
	parser := NewIPParser()
	ips, err := parser.ParseIP("1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 1 {
		t.Fatalf("期望 1 个, 实际 %d", len(ips))
	}
	if ips[0].IP != "1.1.1.1" {
		t.Errorf("期望 1.1.1.1, 实际 %s", ips[0].IP)
	}
	if ips[0].isCIDR {
		t.Error("单 IP 不应标记为 CIDR")
	}
}

// 2. 注释/空行/非法行应被跳过
func TestParseSkipInvalid(t *testing.T) {
	input := "1.1.1.1\n\n# 注释\n// 注释\nnot-an-ip\n8.8.8.8\n"
	parser := NewIPParser()
	ips, err := parser.ParseIP(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 2 {
		t.Fatalf("期望 2 个, 实际 %d: %v", len(ips), ips)
	}
}

// 3. 带采样数的 CIDR: 精确生成 N 个, 且都在段内
func TestParseCIDRWithSampleCount(t *testing.T) {
	parser := NewIPParser()
	ips, err := parser.ParseIP("104.16.0.0/13=50")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 50 {
		t.Fatalf("期望 50 个采样, 实际 %d", len(ips))
	}
	prefix, _ := netip.ParsePrefix("104.16.0.0/13")
	for _, ip := range ips {
		addr, err := netip.ParseAddr(ip.IP)
		if err != nil {
			t.Errorf("生成非法 IP: %q", ip.IP)
			continue
		}
		if !prefix.Contains(addr) {
			t.Errorf("IP %s 不在段 104.16.0.0/13 内", ip.IP)
		}
	}
}

// 4. 纯 CIDR(无 =): 默认采样 5 个, 且都在段内
func TestParseCIDRAutoSample(t *testing.T) {
	parser := NewIPParser()
	ips, err := parser.ParseIP("1.1.1.0/24")
	if err != nil {
		t.Fatal(err)
	}
	// 无 = 时默认 5 个
	if len(ips) != 5 {
		t.Fatalf("期望 5 个, 实际 %d", len(ips))
	}
	prefix, _ := netip.ParsePrefix("1.1.1.0/24")
	for _, ip := range ips {
		addr, _ := netip.ParseAddr(ip.IP)
		if !prefix.Contains(addr) {
			t.Errorf("IP %s 不在 1.1.1.0/24 内", ip.IP)
		}
	}
}

// 5. 混合输入: 单IP + 带采样CIDR 同时解析
func TestParseMixed(t *testing.T) {
	input := "1.1.1.1\n104.16.0.0/13=10\n"
	parser := NewIPParser()
	ips, err := parser.ParseIP(input)
	if err != nil {
		t.Fatal(err)
	}
	// 1 个单 IP + 10 个采样
	if len(ips) != 11 {
		t.Fatalf("期望 11 个, 实际 %d", len(ips))
	}
	if ips[0].IP != "1.1.1.1" || ips[0].isCIDR {
		t.Errorf("第一个应是单 IP 1.1.1.1, 实际 %+v", ips[0])
	}
	for _, ip := range ips[1:] {
		if !ip.isCIDR {
			t.Errorf("采样 IP 应标记 isCIDR=true: %+v", ip)
		}
	}
}

// 6. 解析 ip.txt 文件(集成): 至少能解析出 IP
func TestParseIPFile(t *testing.T) {
	ips, err := parseIPFile("ip.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) == 0 {
		t.Fatal("ip.txt 未解析出任何 IP")
	}
	t.Logf("ip.txt 解析出 %d 个 IP", len(ips))
}

// 7. hasSampleCount: 有 = 返回 true, 无 = 返回 false
func TestHasSampleCount(t *testing.T) {
	if !hasSampleCount("104.16.0.0/13=500") {
		t.Error("带 = 应返回 true")
	}
	if hasSampleCount("104.16.0.0/13") {
		t.Error("不带 = 应返回 false")
	}
	if hasSampleCount("1.1.1.1") {
		t.Error("单 IP 应返回 false")
	}
}

// 8. get_sample_count_and_cidr: 正确拆出 (prefix, count)
func TestGetSampleCountAndCidr(t *testing.T) {
	prefix, count := get_sample_count_and_cidr("104.16.0.0/13=500")
	if prefix.String() != "104.16.0.0/13" {
		t.Errorf("期望 prefix=104.16.0.0/13, 实际 %s", prefix.String())
	}
	if count != 500 {
		t.Errorf("期望 count=500, 实际 %d", count)
	}
}

// 9. 采样数超过段大小: 应截断, 不 panic, 不超过段大小
func TestSampleCountExceedsRange(t *testing.T) {
	parser := NewIPParser()
	// /30 段只有 4 个 IP, 请求 100 个 → 应截断到 4
	ips, err := parser.ParseIP("1.1.1.0/30=100")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) > 4 {
		t.Errorf("采样数超过段大小, 实际 %d > 4", len(ips))
	}
	if len(ips) == 0 {
		t.Error("/30=100 不应返回空")
	}
}

// 10. 采样数量精确性: 各种采样数都精确生成
func TestSampleCountExact(t *testing.T) {
	for _, tc := range []struct {
		cidr string
		want int
	}{
		{"104.16.0.0/13=100", 100},
		{"104.16.0.0/13=1", 1},
		{"104.16.0.0/24=3", 3},
	} {
		parser := NewIPParser()
		ips, err := parser.ParseIP(tc.cidr)
		if err != nil {
			t.Fatal(err)
		}
		if len(ips) != tc.want {
			t.Errorf("%s: 期望 %d 个, 实际 %d", tc.cidr, tc.want, len(ips))
		}
	}
}

// 11. 采样 IP 的 isCIDR 标记: 带 = 的 CIDR 生成的 IP 都标记 isCIDR
func TestSampledIPsMarkedCIDR(t *testing.T) {
	parser := NewIPParser()
	ips, err := parser.ParseIP("1.1.1.0/24=5")
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range ips {
		if !ip.isCIDR {
			t.Errorf("采样 IP 应标记 isCIDR=true: %+v", ip)
		}
	}
}

// 辅助: 读取文件并解析(与 main.go 相同逻辑)
func parseIPFile(path string) ([]IP, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parser := NewIPParser()
	return parser.ParseIP(string(data))
}
