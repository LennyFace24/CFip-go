package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/LennyFace24/CFip-go/src/core"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// cidrFetchTimeout 在线拉取网段的最长等待时间，超时即回退内置列表
const cidrFetchTimeout = 8 * time.Second

// CIDRSource 内置网段列表及其来源，便于界面告知用户用的是在线还是内置。
type CIDRSource struct {
	CIDRs  []string
	Online bool   // true = 在线拉取成功
	Error  string // 在线拉取失败的原因，仅 Online 为 false 时有值
}

// ImportResult 导入 ip.txt 的结果。
type ImportResult struct {
	Path    string // 文件完整路径
	Content string // 文件内容，前端据此回填到输入框
	Count   int    // 解析出的 IP 数量
}

// IPService 提供 IP 来源相关能力：内置网段、导入文件、解析、剪贴板。
type IPService struct {
	app *application.App
}

func (s *IPService) ServiceName() string { return "IPService" }

// BuiltinCIDRs 返回内置网段：优先在线拉取官方列表，拉取失败时回退到编译期内置列表。
func (s *IPService) BuiltinCIDRs() CIDRSource {
	cidrs, err := fetchOnlineCIDRs()
	if err == nil && len(cidrs) > 0 {
		return CIDRSource{CIDRs: cidrs, Online: true}
	}
	source := CIDRSource{CIDRs: core.CloudflareCIDRs, Online: false}
	if err != nil {
		source.Error = err.Error()
	}
	return source
}

// fetchOnlineCIDRs 带超时地拉取官方网段列表
func fetchOnlineCIDRs() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cidrFetchTimeout)
	defer cancel()
	return core.FetchCIDRs(ctx, core.CloudflareIPv4URL)
}

// BuiltinText 返回指定网段的文本，每行一条；sample <= 0 时不指定采样数。
// cidrs 为空时返回全部内置网段。
func (s *IPService) BuiltinText(cidrs []string, sample int) string {
	if len(cidrs) == 0 {
		return core.CloudflareIPText(sample)
	}
	return core.CIDRsText(cidrs, sample)
}

// ImportFile 打开系统文件对话框选择 ip.txt 并读取内容。
func (s *IPService) ImportFile() (ImportResult, error) {
	if s.app == nil {
		return ImportResult{}, errors.New("应用尚未初始化")
	}
	path, err := s.app.Dialog.OpenFile().
		SetTitle("选择 IP 列表文件").
		AddFilter("文本文件 (*.txt)", "*.txt").
		AddFilter("所有文件 (*.*)", "*.*").
		PromptForSingleSelection()
	if err != nil {
		return ImportResult{}, err
	}
	if path == "" {
		return ImportResult{}, errors.New("已取消选择文件")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ImportResult{}, err
	}
	content := string(data)
	count, err := s.Parse(content)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Path: path, Content: content, Count: count}, nil
}

// Parse 解析 IP 文本，返回可测速的 IP 数量，供前端实时校验输入。
func (s *IPService) Parse(text string) (int, error) {
	ips, err := core.NewIPParser().ParseIP(text)
	if err != nil {
		return 0, err
	}
	return len(ips), nil
}

// CopyToClipboard 把文本写入系统剪贴板。
func (s *IPService) CopyToClipboard(text string) bool {
	if s.app == nil {
		return false
	}
	return s.app.Clipboard.SetText(text)
}
