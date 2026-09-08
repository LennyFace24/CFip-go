package core

import "strings"

// ColoInfo 一个 Cloudflare 边缘机房。Code 即 cf-ray 里的 IATA 三字码。
type ColoInfo struct {
	Code string
	Name string
}

// RecommendedColos 是对中国大陆访问较友好的东亚 / 东南亚机房，
// 作为界面上的快捷选择，省去用户手动查代码。
var RecommendedColos = []ColoInfo{
	{Code: "HKG", Name: "香港"},
	{Code: "TPE", Name: "台北"},
	{Code: "KHH", Name: "高雄"},
	{Code: "MFM", Name: "澳门"},
	{Code: "NRT", Name: "东京"},
	{Code: "KIX", Name: "大阪"},
	{Code: "FUK", Name: "福冈"},
	{Code: "OKA", Name: "那霸"},
	{Code: "ICN", Name: "首尔"},
	{Code: "SIN", Name: "新加坡"},
	{Code: "KUL", Name: "吉隆坡"},
	{Code: "BKK", Name: "曼谷"},
	{Code: "MNL", Name: "马尼拉"},
	{Code: "HAN", Name: "河内"},
	{Code: "SGN", Name: "胡志明"},
	{Code: "PNH", Name: "金边"},
	{Code: "VTE", Name: "万象"},
	{Code: "RGN", Name: "仰光"},
	{Code: "CMB", Name: "科伦坡"},
	{Code: "KTM", Name: "加德满都"},
}

// ParseColo 从 cf-ray 响应头取值中解析机房代码。
// cf-ray 形如 "8f2a1b3c4d5e6f70-HKG"，取最后一个 '-' 之后的部分。
func ParseColo(cfRay string) string {
	cfRay = strings.TrimSpace(cfRay)
	if cfRay == "" {
		return ""
	}
	if i := strings.LastIndex(cfRay, "-"); i >= 0 && i < len(cfRay)-1 {
		return strings.ToUpper(cfRay[i+1:])
	}
	return ""
}

// SplitColos 把用户输入的白名单拆成机房代码，支持空格 / 逗号 / 顿号 / 分号分隔。
func SplitColos(input string) []string {
	replacer := strings.NewReplacer(
		",", " ", "，", " ", "、", " ", ";", " ", "；", " ",
		"\t", " ", "\n", " ", "\r", " ",
	)
	var out []string
	seen := make(map[string]struct{})
	for _, part := range strings.Fields(replacer.Replace(input)) {
		code := strings.ToUpper(part)
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

// ColoAllowed 判断机房是否通过白名单。白名单为空表示不过滤。
// 注意：开启白名单后，取不到机房代码的结果一律视为不符——
// 否则一旦 cf-ray 缺失，所有 IP 都会被放行，过滤形同虚设。
func ColoAllowed(colo string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return true
	}
	if colo == "" {
		return false
	}
	for _, w := range whitelist {
		if strings.EqualFold(colo, w) {
			return true
		}
	}
	return false
}
