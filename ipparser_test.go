package main

import (
	"net/netip"
	"os"
	"testing"
)

// 集成测试: 读 ip.txt, 验证解析结果合法 + 分类正确
func TestParseIP(t *testing.T) {
	file, err := os.ReadFile("ip.txt")
	if err != nil {
		t.Fatalf("读文件失败: %v", err)
	}
	ips, err := parseIP(string(file))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	var singleCount, cidrCount int
	for _, ip := range ips {
		addr, err := netip.ParseAddr(ip.IP)
		if err != nil {
			t.Errorf("解析结果不是合法 IP: %q", ip.IP)
			continue
		}
		if !addr.Is4() {
			t.Errorf("解析结果不是 IPv4: %q", ip.IP)
			continue
		}
		if ip.isCIDR {
			cidrCount++
		} else {
			singleCount++
		}
	}

	t.Logf("单 IP 数量=%d, CIDR 生成数量=%d, 总计=%d", singleCount, cidrCount, len(ips))

	if singleCount != 2 {
		t.Errorf("期望单 IP 数量=2, 实际=%d", singleCount)
	}
	if len(ips) == 0 {
		t.Fatal("未解析出任何 IP")
	}
}

// 表驱动测试: 每个 CIDR 生成的 IP 必须都落在该网段内, 且数量符合采样规则
func TestParseCIDRSampling(t *testing.T) {
	cases := []struct {
		cidr      string
		wantCount int
	}{
		{"1.1.1.1/24", 5},       // /24 → 采样 5 个
		{"104.16.0.0/16", 1280}, // /16 → 采样 1280 个
	}

	for _, tc := range cases {
		t.Run(tc.cidr, func(t *testing.T) {
			prefix, err := netip.ParsePrefix(tc.cidr)
			if err != nil {
				t.Fatal(err)
			}

			ips := parseCIDR(tc.cidr)

			// 1. 每个 IP 必须落在网段内
			for _, ip := range ips {
				addr, err := netip.ParseAddr(ip.IP)
				if err != nil {
					t.Errorf("生成了非法 IP: %q", ip.IP)
					continue
				}
				if !prefix.Contains(addr) {
					t.Errorf("生成的 IP 不在网段 %s 内: %s", tc.cidr, ip.IP)
				}
			}

			// 2. 采样数量必须符合规则
			if len(ips) != tc.wantCount {
				t.Errorf("采样数量不符: 期望 %d, 实际 %d", tc.wantCount, len(ips))
			}
		})
	}
}

// 单 IP 解析测试
func TestParseSingleIP(t *testing.T) {
	ips, err := parseIP("1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 1 {
		t.Fatalf("期望 1 个 IP, 实际 %d", len(ips))
	}
	if ips[0].isCIDR {
		t.Errorf("单 IP 不应被标记为 CIDR")
	}
	if ips[0].IP != "1.1.1.1" {
		t.Errorf("期望 1.1.1.1, 实际 %s", ips[0].IP)
	}
}

// 注释行和空行应被跳过
func TestParseSkipComments(t *testing.T) {
	ips, err := parseIP("1.1.1.1\n\n# 注释\n// 注释\n  8.8.8.8  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 2 {
		t.Errorf("期望 2 个 IP, 实际 %d", len(ips))
	}
}
