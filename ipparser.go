package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

type IP struct {
	IP     string
	isCIDR bool
}

func parseIP(ips string) ([]IP, error) {
	var IPs []IP

	//  1.1.1.1
	for _, line := range strings.Split(ips, "\n") {
		line = strings.TrimSpace(line)
		// 空字符串跳过
		if line == "" {
			continue
		}
		// 解析单个IP
		if net.ParseIP(line) != nil {
			IPs = append(IPs, IP{IP: line, isCIDR: false})
			continue
		}
		// 标准cidr格式
		if _, _, err := net.ParseCIDR(line); err == nil {
			ipcidrs := parseCIDR(line)
			IPs = append(IPs, ipcidrs...)
			continue
		}
		// cidr格式带采样数
		if hasSampleCount(line) {
			if _, _, err := net.ParseCIDR(strings.Split(line, "=")[0]); err == nil {
				ipcidrs := parseCIDR(line)
				IPs = append(IPs, ipcidrs...)
				continue
			}
		}
	}
	return IPs, nil
}

func parseCIDR(cidr string) []IP {
	sample_count := 5 // 默认五个
	var prefix netip.Prefix
	if hasSampleCount(cidr) {
		prefix, sample_count = get_sample_count_and_cidr(cidr)
	} else {
		prefix, _ = netip.ParsePrefix(cidr)
	}

	if hasSampleCount(cidr) {
		prefix, sample_count = get_sample_count_and_cidr(cidr)
	}
	// 返回采样ip数组
	return return_sampled_ips(sample_count, prefix, prefix.Bits())
}

// 限制ip采样数量后的算法
func return_sampled_ips(sample_count int, prefix netip.Prefix, bits int) []IP {
	var IPs []IP

	// 获取网段开始ip
	// [104, 10, 0, 0]
	a := prefix.Masked().Addr().As4()

	// 采样数大于等于5个时，先把最后五个排除
	random_range := 1 << (32 - bits)
	for range sample_count {
		ip := rand.Intn(random_range + 1) // 生成随机数
		// 将随机数转换为IP地址
		// 右移8位，获得被修改的数字
		b3 := int(a[3]) | (ip & 0xFF)
		b2 := int(a[2]) | (ip >> 8 & 0xFF)
		b1 := int(a[1]) | (ip >> 16 & 0xFF)
		b0 := int(a[0]) | (ip >> 24 & 0xFF)
		// 将被修改的数字与原始IP地址的前三个字节组合成新的IP地址
		new_ip := fmt.Sprintf("%d.%d.%d.%d", b0, b1, b2, b3)
		IPs = append(IPs, IP{IP: new_ip, isCIDR: true})
	}

	return IPs
}

func hasSampleCount(cidr string) bool {
	return strings.Contains(cidr, "=")
}

func get_sample_count_and_cidr(cidr string) (netip.Prefix, int) {
	// 解析出采样数量和cidr
	parts := strings.Split(cidr, "=")
	sample_count, err := strconv.Atoi(parts[1])
	if err != nil {
		fmt.Println("携带采样数的cidr ip格式不正确！，具体字符串为： ", cidr)
		return netip.Prefix{}, -1
	}
	// 这个只是用户指定的，如果用户指定的采样数大于理论最大采样数，则使用理论最大采样数
	// 通过网段位数算出剩下的ip有多少个
	prefix, err := netip.ParsePrefix(parts[0])
	if err != nil {
		fmt.Println("cidr格式不正确！，具体字符串为： ", cidr)
		return netip.Prefix{}, -1
	}
	if sample_count > 1<<(32-prefix.Bits()) {
		fmt.Println("用户指定的采样数大于理论最大采样数，使用理论最大采样数，具体字符串为： ", cidr)
		sample_count = 1 << (32 - prefix.Bits())
	}
	return prefix, sample_count
}
