package main

import (
	"fmt"
	"github.com/LennyFace24/CFip-go/src/core"
	"os"
	"sort"
)

func main() {
	file, err := os.ReadFile("ip.txt")
	if err != nil {
		panic(err)
	}
	parser := core.NewIPParser() // 初始化IP解析器
	ips, err := parser.ParseIP(string(file))
	if err != nil {
		panic(err)
	}

	// 请求每个ip获取延迟
	lates := core.RequestAndChooseGoodAndGetLatency(ips)
	sort.Slice(lates, func(i, j int) bool {
		return lates[i].Latency < lates[j].Latency
	})

	fmt.Println("IP 延迟排序结果如下:")
	// 输出结果
	for i, late := range lates {
		if i >= 20 {
			fmt.Println("超过20个了")
			break
		}
		fmt.Printf("%v: %.3f\n", late.IP, late.Latency)
	}
}
