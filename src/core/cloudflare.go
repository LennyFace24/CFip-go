package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
)

// DefaultSampleCount 网段默认采样数，与 ParseIP 内部默认值保持一致。
const DefaultSampleCount = 5

// CloudflareIPv4URL 是 Cloudflare 官方公布的 IPv4 网段列表地址。
const CloudflareIPv4URL = "https://www.cloudflare.com/ips-v4"

// 在线列表体积很小，限制读取上限避免异常响应吃满内存
const maxCIDRBodySize = 64 << 10

// CloudflareCIDRs 是 Cloudflare 官方公布的 IPv4 网段（https://www.cloudflare.com/ips-v4）。
// 顺序与官方列表一致，便于 GUI 原样展示。
var CloudflareCIDRs = []string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
}

// CIDRsText 把任意网段列表拼成可直接交给 ParseIP 的文本，每行一条。
// sample > 0 时写成 "cidr=sample" 形式；否则不带采样数，由 ParseIP 取默认值。
func CIDRsText(cidrs []string, sample int) string {
	var b strings.Builder
	for _, cidr := range cidrs {
		if sample > 0 {
			b.WriteString(fmt.Sprintf("%s=%d\n", cidr, sample))
			continue
		}
		b.WriteString(cidr + "\n")
	}
	return b.String()
}

// CloudflareIPText 返回内置网段的文本，每行一条。
func CloudflareIPText(sample int) string {
	return CIDRsText(CloudflareCIDRs, sample)
}

// FetchCIDRs 在线拉取网段列表，返回去重后的合法 IPv4 CIDR。
// 空行、注释、IPv6 与非法行一律忽略，因此部分内容损坏时仍可得到可用结果。
func FetchCIDRs(ctx context.Context, url string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCIDRBodySize))
	if err != nil {
		return nil, err
	}
	cidrs := ParseCIDRLines(string(body))
	if len(cidrs) == 0 {
		return nil, fmt.Errorf("响应中没有解析到网段")
	}
	return cidrs, nil
}

// ParseCIDRLines 从任意文本中筛出合法的 IPv4 CIDR，按出现顺序去重。
func ParseCIDRLines(text string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "\r"))
		if line == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(line)
		if err != nil || !prefix.Addr().Is4() {
			continue
		}
		if _, dup := seen[line]; dup {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out
}
