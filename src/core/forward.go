package core

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	forwardDialTimeout = 3 * time.Second
	// forwardMaxAttempts 单条连接最多尝试几个节点
	forwardMaxAttempts = 3
)

// DialFunc 建立 TCP 连接，便于测试注入
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Forwarder 在本地监听 SOCKS5，把 CONNECT 请求转发到 IP 池中的优选节点。
//
// 只做 TCP 字节转发：不解密 TLS，也不改写 SNI，因此
// 真实目标域名的证书校验仍由客户端与目标服务器完成。
//
// 客户端告知的目标域名只用于判定端口，实际拨号地址是「优选节点 IP + 远端端口」——
// Cloudflare 会把流量路由到哪个站点，由客户端 ClientHello 里的 SNI 决定。
type Forwarder struct {
	pool *Pool

	mu         sync.Mutex
	dial       DialFunc
	ln         net.Listener
	active     int
	lastUsed   string
	lastUsedAt time.Time
}

func NewForwarder(pool *Pool) *Forwarder {
	return &Forwarder{
		pool: pool,
		dial: (&net.Dialer{Timeout: forwardDialTimeout}).DialContext,
	}
}

// SetDial 替换拨号实现，仅供测试使用。
func (f *Forwarder) SetDial(dial DialFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dial = dial
}

// Addr 返回实际监听地址；未开始监听时返回空串。
func (f *Forwarder) Addr() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ln == nil {
		return ""
	}
	return f.ln.Addr().String()
}

// ActiveConns 返回当前正在转发的连接数。
func (f *Forwarder) ActiveConns() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active
}

// Listen 在 addr 上监听并服务，阻塞直到 ctx 取消或监听出错。
func (f *Forwarder) Listen(ctx context.Context, addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	f.mu.Lock()
	f.ln = listener
	f.mu.Unlock()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		client, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go f.serve(ctx, client)
	}
}

func (f *Forwarder) serve(ctx context.Context, client net.Conn) {
	defer client.Close()

	if err := negotiate(client); err != nil {
		return
	}
	req, err := readRequest(client)
	if err != nil {
		return
	}

	upstream, err := f.connect(ctx, remotePort(req.Port))
	if err != nil {
		_ = writeReply(client, socks5RepHostUnreachable)
		return
	}
	defer upstream.Close()

	f.addActive(1)
	defer f.addActive(-1)

	if err := writeReply(client, socks5RepSucceeded); err != nil {
		return
	}
	pipe(client, upstream)
}

// connect 依次尝试延迟最低的若干节点，返回第一个拨通的连接。
//
// 拨不通的节点先隔离而不是直接淘汰：交给后续健康检查确认是真的失效还是临时抖动，
// 同时避免每次连接都在同一个坏节点上白等一次超时。
func (f *Forwarder) connect(ctx context.Context, port uint16) (net.Conn, error) {
	if f.pool == nil {
		return nil, errors.New("IP 池未初始化")
	}

	f.mu.Lock()
	dial := f.dial
	f.mu.Unlock()

	var lastErr error
	for _, node := range f.pool.PickOrdered(forwardMaxAttempts) {
		address := net.JoinHostPort(node.IP, strconv.Itoa(int(port)))
		conn, err := dial(ctx, "tcp", address)
		if err == nil {
			f.markUsed(node.IP)
			return conn, nil
		}
		lastErr = err
		f.pool.Isolate(node.IP)
	}

	if lastErr == nil {
		return nil, errors.New("IP 池中没有可用节点")
	}
	return nil, lastErr
}

// markUsed 记录最近一次成功转发的节点，供界面高亮
func (f *Forwarder) markUsed(ip string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastUsed = ip
	f.lastUsedAt = time.Now()
}

// LastUsed 返回最近一次成功转发的节点与时间；从未转发过时返回空串
func (f *Forwarder) LastUsed() (string, time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastUsed, f.lastUsedAt
}

func (f *Forwarder) addActive(delta int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active += delta
}

// remotePort 决定实际连接优选节点的端口。
// Cloudflare 只在 80 与 443 提供服务，其余端口一律按 HTTPS 处理。
func remotePort(targetPort uint16) uint16 {
	if targetPort == 80 {
		return 80
	}
	return 443
}

// pipe 双向拷贝，任一侧结束后关闭两端使另一个方向退出。
func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(a, b)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(b, a)
		done <- struct{}{}
	}()

	<-done
	_ = a.Close()
	_ = b.Close()
	<-done
}
