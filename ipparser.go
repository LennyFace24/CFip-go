package main

import (
	"math/rand"
	"net"
	"net/netip"
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
		if line == "" {
			continue
		}
		if net.ParseIP(line) != nil {
			IPs = append(IPs, IP{IP: line, isCIDR: false})
			continue
		}
		if _, _, err := net.ParseCIDR(line); err == nil {
			ipcidrs := parseCIDR(line)
			IPs = append(IPs, ipcidrs...)
			continue
		}
	}
	return IPs, nil
}

func parseCIDR(cidr string) []IP {
	var IPs []IP
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return IPs
	}
	// 获得ip的4字节表示
	bytes := prefix.Addr().As4()

	// first := rand.Intn(52)
	// second := rand.Intn(52) + 51
	// third := rand.Intn(52) + 51 + 51
	// fourth := rand.Intn(52) + 51 + 51 + 51
	// fifth := rand.Intn(52) + 51 + 51 + 51 + 51

	for i := 0; i < 5; i++ {
		fourth := rand.Intn(52) + 51*i
		// Do something with the generated IP addresses
		bytes[3] = byte(fourth)

		if 24-prefix.Bits() <= 0 {
			ip := net.IPv4(bytes[0], bytes[1], bytes[2], bytes[3]).String()
			IPs = append(IPs, IP{IP: ip, isCIDR: true})
			continue
		}
		for j := 0; j < 1<<(24-prefix.Bits()); j++ {
			if j > 255 {
				break
			}
			bytes[2] = byte(j)

			if 16-prefix.Bits() <= 0 {
				ip := net.IPv4(bytes[0], bytes[1], bytes[2], bytes[3]).String()
				IPs = append(IPs, IP{IP: ip, isCIDR: true})
				continue
			}
			for k := 0; k < 1<<(16-prefix.Bits()); k++ {
				if k > 255 {
					break
				}
				bytes[1] = byte(k)
				if 8-prefix.Bits() <= 0 {
					ip := net.IPv4(bytes[0], bytes[1], bytes[2], bytes[3]).String()
					IPs = append(IPs, IP{IP: ip, isCIDR: true})
					continue
				}
				for l := 0; l < 1<<(8-prefix.Bits()); l++ {
					if l > 255 {
						break
					}
					bytes[0] = byte(l)
					ip := net.IPv4(bytes[0], bytes[1], bytes[2], bytes[3]).String()
					IPs = append(IPs, IP{IP: ip, isCIDR: true})
				}
			}
		}

	}

	return IPs
}
