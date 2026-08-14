package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/LennyFace24/CFip-go/src/config"
	"github.com/LennyFace24/CFip-go/src/core"
)

func main() {

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Println("加载配置文件出错:", err)
		panic(err)
	}

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
		if i >= cfg.Number { // 只输出前 N 个
			break
		}
		fmt.Printf("%-20s: %8.3f s\n", late.IP.IP, late.Latency)
	}
}
