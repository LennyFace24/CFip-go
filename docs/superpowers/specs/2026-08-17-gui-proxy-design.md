# CFip-go GUI + 本地代理 设计文档

日期：2026-08-17
分支：dev/gui
状态：已批准

## 目标

为 CFip-go 增加两个能力：
1. **Fyne GUI 窗口应用**：图形化完成"IP 列表 → 测速 → 看结果 → 控制代理"全流程，替代命令行操作
2. **本地 TCP 透明转发代理**（CFnat 式）：把优选 IP 变成可用的转发通道，客户端把流量指向本地端口即可经优选 IP 到达 CF 边缘

保留现有 CLI（`cmd/main.go`）与 TUI（`cmd/tui`）入口，三入口共存。

## 技术选型

- **GUI 框架**：fyne（纯 Go 跨平台，CGO 编译；环境已验证：MinGW gcc 15.2.0 + CGO_ENABLED=1）
- **代理模式**：TCP 透明转发（不解析协议、不改内容，客户端把本工具端口当服务器地址）
- **负载均衡**：延迟加权（延迟越低被选中概率越高）+ 连续失败拉黑故障切换

## 架构

```
cmd/gui/main.go        ← Fyne 窗口入口（组装布局、接线）
src/proxy/             ← 新包：TCP 透明转发代理
  ├─ pool.go           ← 优选池：延迟加权选择 + 故障拉黑
  ├─ forward.go        ← TCP 转发引擎：监听、双向转发、连接数/流量统计
  └─ proxy_test.go     ← 池选择分布、故障切换、转发连通性单测
src/gui/               ← 新包：Fyne 界面
  ├─ app.go            ← 窗口布局组装（输入/控制/表格/代理面板/日志）
  ├─ table.go          ← 测速结果表格
  └─ prefs.go          ← 设置持久化（fyne Preferences）
src/core/              ← 复用 StreamLatency / ParseIP / config，零改动
```

### 包边界

- `src/proxy`：纯业务代理逻辑，不依赖 GUI。通过接口注入测速结果与日志回调。
- `src/gui`：纯界面，通过 channel 消费测速流、调用 proxy 启停。
- `cmd/gui`：接线层。

## 数据流

```
[GUI 开始测速] → core.StreamLatency → channel
    → fyne.Do 回主线程 → 表格逐行刷新 + 进度条
[GUI 启动代理] → proxy.NewPool(优选成功结果)
    → 监听 127.0.0.1:PORT
    → 每连接: Pool.Pick() 延迟加权选 IP
    → TCP 双向转发到 IP:443
    → atomic 统计连接数/收发字节 → 定时刷新 GUI
    → 日志回调 → 日志面板
```

## 代理核心设计（src/proxy）

### pool.go

```go
type Pool struct {
    mu   sync.RWMutex
    ips  []poolItem   // 仅含测速成功的 IP
    ban  map[string]int // IP -> 连续失败计数
}
type poolItem struct {
    result core.StreamResult
    fail   int // 连续失败计数
}

// NewPool 从测速成功结果构建池
func NewPool(results []core.StreamResult) *Pool

// Pick 延迟加权随机选一个 IP；所有 IP 被拉黑时返回空
func (p *Pool) Pick() (string, bool)

// ReportResult 上报一次转发结果：成功清零 fail，失败累加；
// fail >= maxFail 时拉黑该 IP（从候选移除）
func (p *Pool) ReportResult(ip string, ok bool)

// Len 当前可用 IP 数
func (p *Pool) Len() int
```

**延迟加权**：权重 `w = 1/latency`（延迟越小权重越大），`Pick` 按权重比例随机。实现：累计权重前缀和 + 随机数二分。

**拉黑阈值**：`maxFail = 3`（连续 3 次失败拉黑）。拉黑后 `Pick` 不再选中；保留在 `ban` 中供 GUI 展示状态。

### forward.go

```go
type Forwarder struct {
    listener net.Listener
    pool     *Pool
    conns    atomic.Int64  // 活跃连接数
    rx       atomic.Int64  // 收字节
    tx       atomic.Int64  // 发字节
    logf     func(format string, args ...any)  // 日志回调，可 nil
}

// NewForwarder 监听 addr，连接到达后转发到 Pool.Pick():443
func NewForwarder(addr string, pool *Pool, logf func(string, ...any)) *Forwarder

// Start 开始监听（阻塞在 Accept 循环，goroutine 中运行）
func (f *Forwarder) Start() error

// Close 关闭监听并终止转发
func (f *Forwarder) Close() error

// Stats 返回 (活跃连接数, 收字节, 发字节)
func (f *Forwarder) Stats() (conns, rx, tx int64)
```

**转发逻辑**：accept 后 goroutine 处理；`Pick` 选 IP → `net.DialTimeout(ip:443, 3s)` → 双向 `io.Copy`；任一端关闭即断开；结束后 `ReportResult(ip, 成功/失败)`；连接数/流量 atomic 累加。目标端口 443 暂为常量（后续可配置）。

## GUI 设计（src/gui）

### 布局

```
┌──────────────────────────────────────────────┐
│ IP 列表 [多行输入框]        [载入 ip.txt]      │
│ 并发[16] 延迟上限[500] 优选数[20]              │
├──────────────────────────────────────────────┤
│ [开始测速] [停止]   进度条 123/500 成功45      │
├──────────────────────────────────────────────┤
│ IP      延迟(ms)  状态  来源   [点击表头排序]  │
│ …                                            │
├──────────────────────────────────────────────┤
│ 代理: 端口[1234] [启动代理] [停止代理]         │
│ 池: 8/20 IP 可用   连接: 3   收: 1.2MB 发: 3KB│
├──────────────────────────────────────────────┤
│ 日志面板（滚动）                              │
└──────────────────────────────────────────────┘
```

### 交互与线程

- 测速 goroutine 产出结果 → `fyne.Do` 回主线程刷新表格（Fyne 必须主线程更新 UI）
- 代理统计每 500ms ticker 刷新
- 日志回调线程安全（内部锁 + 队列，由主线程 drain）

### 设置持久化（prefs.go）

fyne `Preferences` 保存：上次 IP 列表、并发数、延迟上限、优选数、代理端口。启动自动恢复。

## 测试策略

- `src/proxy`：
  - 延迟加权：快 IP 在大量 Pick 中命中率显著高于慢 IP（统计测试）
  - 故障拉黑：连续失败 3 次后不再被 Pick；成功后 fail 清零
  - 转发连通性：本地起 echo TCP server 当"目标"，起 Forwarder 转发，客户端经 Forwarder 访问 echo，验证数据往返 + 统计计数
- `src/gui`：导出事件逻辑抽状态机可测（如"测速中禁止再次开始"）；界面人工验证
- 现有 CLI/TUI 测试全部保持通过

## 打包发布

`fyne package -os windows -icon icon.png` 生成独立 exe（后续 Task 细化）。首版先保证 `go run ./cmd/gui` 可运行。

## 非目标（YAGNI）

- 不做 clash/sing-box 配置导出（用户已砍）
- 不做 SOCKS5 / HTTP 代理协议（TCP 透明转发先行）
- 不做目标端口可配置（443 常量）
- 不做跨平台打包细节（先 Windows）
