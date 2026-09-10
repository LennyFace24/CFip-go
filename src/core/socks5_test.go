package core

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestNegotiateAcceptsNoAuth(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() { _, _ = client.Write([]byte{socks5Version, 1, socks5AuthNone}) }()

	done := make(chan error, 1)
	go func() { done <- negotiate(server) }()

	reply := make([]byte, 2)
	if _, err := io.ReadFull(client, reply); err != nil {
		t.Fatalf("读取协商应答失败: %v", err)
	}
	if reply[0] != socks5Version || reply[1] != socks5AuthNone {
		t.Fatalf("want 05 00, got % x", reply)
	}
	if err := <-done; err != nil {
		t.Fatalf("negotiate: %v", err)
	}
}

func TestNegotiateRejectsUnsupportedAuth(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// 客户端只提供「用户名密码」（0x02）
	go func() { _, _ = client.Write([]byte{socks5Version, 1, 0x02}) }()

	done := make(chan error, 1)
	go func() { done <- negotiate(server) }()

	reply := make([]byte, 2)
	if _, err := io.ReadFull(client, reply); err != nil {
		t.Fatalf("读取协商应答失败: %v", err)
	}
	if reply[1] != socks5AuthRejected {
		t.Fatalf("want %#x, got %#x", socks5AuthRejected, reply[1])
	}
	if err := <-done; !errors.Is(err, errSocks5Auth) {
		t.Fatalf("want errSocks5Auth, got %v", err)
	}
}

func TestReadRequestParsesDomain(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	request := []byte{socks5Version, socks5CmdConnect, 0x00, socks5AddrDomain, 0x0b}
	request = append(request, "example.com"...)
	request = append(request, 0x01, 0xbb) // 端口 443

	go func() { _, _ = client.Write(request) }()

	req, err := readRequest(server)
	if err != nil {
		t.Fatalf("readRequest: %v", err)
	}
	if req.Host != "example.com" || req.Port != 443 {
		t.Fatalf("want example.com:443, got %s:%d", req.Host, req.Port)
	}
}

func TestReadRequestParsesIPv4(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	request := []byte{socks5Version, socks5CmdConnect, 0x00, socks5AddrIPv4, 1, 2, 3, 4, 0x00, 0x50}

	go func() { _, _ = client.Write(request) }()

	req, err := readRequest(server)
	if err != nil {
		t.Fatalf("readRequest: %v", err)
	}
	if req.Host != "1.2.3.4" || req.Port != 80 {
		t.Fatalf("want 1.2.3.4:80, got %s:%d", req.Host, req.Port)
	}
}

func TestReadRequestRejectsOtherCommands(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// CMD = 0x02（BIND），本实现不支持
	go func() { _, _ = client.Write([]byte{socks5Version, 0x02, 0x00, socks5AddrIPv4}) }()

	done := make(chan error, 1)
	go func() { _, err := readRequest(server); done <- err }()

	reply := make([]byte, 10)
	if _, err := io.ReadFull(client, reply); err != nil {
		t.Fatalf("读取应答失败: %v", err)
	}
	if reply[1] != socks5RepCmdNotSupported {
		t.Fatalf("want %#x, got %#x", socks5RepCmdNotSupported, reply[1])
	}
	if err := <-done; !errors.Is(err, errSocks5Cmd) {
		t.Fatalf("want errSocks5Cmd, got %v", err)
	}
}

func TestRemotePort(t *testing.T) {
	cases := map[uint16]uint16{80: 80, 443: 443, 8080: 443, 1234: 443}
	for target, want := range cases {
		if got := remotePort(target); got != want {
			t.Errorf("remotePort(%d) = %d, want %d", target, got, want)
		}
	}
}

// startEcho 启动一个本地回显服务，返回其地址
func startEcho(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动回显服务失败: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()
	return listener.Addr().String()
}

func waitForAddr(t *testing.T, forwarder *Forwarder) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if addr := forwarder.Addr(); addr != "" {
			return addr
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("转发器未在预期时间内开始监听")
	return ""
}

// socks5Connect 完成一次 SOCKS5 握手并发出 CONNECT 请求，返回应答码
func socks5Connect(t *testing.T, conn net.Conn, host string, port uint16) byte {
	t.Helper()

	if _, err := conn.Write([]byte{socks5Version, 1, socks5AuthNone}); err != nil {
		t.Fatalf("发送方法协商失败: %v", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("读取方法协商应答失败: %v", err)
	}
	if reply[1] != socks5AuthNone {
		t.Fatalf("服务端未选择无认证: % x", reply)
	}

	request := []byte{socks5Version, socks5CmdConnect, 0x00, socks5AddrDomain, byte(len(host))}
	request = append(request, host...)
	request = append(request, byte(port>>8), byte(port))
	if _, err := conn.Write(request); err != nil {
		t.Fatalf("发送 CONNECT 失败: %v", err)
	}

	head := make([]byte, 10)
	if _, err := io.ReadFull(conn, head); err != nil {
		t.Fatalf("读取 CONNECT 应答失败: %v", err)
	}
	return head[1]
}

func TestForwarderRelaysTraffic(t *testing.T) {
	echoAddr := startEcho(t)

	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})
	pool.TryAdd("1.2.3.4", 0.05, "HKG")

	forwarder := NewForwarder(pool)
	var mu sync.Mutex
	var dialed []string
	forwarder.SetDial(func(_ context.Context, _, address string) (net.Conn, error) {
		mu.Lock()
		dialed = append(dialed, address)
		mu.Unlock()
		return net.Dial("tcp", echoAddr)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = forwarder.Listen(ctx, "127.0.0.1:0") }()

	conn, err := net.Dial("tcp", waitForAddr(t, forwarder))
	if err != nil {
		t.Fatalf("连接转发器失败: %v", err)
	}
	defer conn.Close()

	if rep := socks5Connect(t, conn, "example.com", 443); rep != socks5RepSucceeded {
		t.Fatalf("CONNECT 应答码 want %#x, got %#x", socks5RepSucceeded, rep)
	}

	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("发送数据失败: %v", err)
	}
	echoed := make([]byte, 5)
	if _, err := io.ReadFull(conn, echoed); err != nil {
		t.Fatalf("读取回显失败: %v", err)
	}
	if string(echoed) != "hello" {
		t.Fatalf("want hello, got %q", echoed)
	}

	mu.Lock()
	got := append([]string(nil), dialed...)
	mu.Unlock()
	if len(got) != 1 || got[0] != "1.2.3.4:443" {
		t.Fatalf("want [1.2.3.4:443], got %v", got)
	}
}

func TestForwarderReportsFailureWhenPoolEmpty(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Second})

	forwarder := NewForwarder(pool)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = forwarder.Listen(ctx, "127.0.0.1:0") }()

	conn, err := net.Dial("tcp", waitForAddr(t, forwarder))
	if err != nil {
		t.Fatalf("连接转发器失败: %v", err)
	}
	defer conn.Close()

	if rep := socks5Connect(t, conn, "example.com", 443); rep != socks5RepHostUnreachable {
		t.Fatalf("want %#x, got %#x", socks5RepHostUnreachable, rep)
	}
}

func TestForwarderDropsUnreachableNode(t *testing.T) {
	pool := NewPool(PoolConfig{PrimarySize: 1, BackupSize: 0, Cooldown: time.Minute})
	pool.TryAdd("1.2.3.4", 0.05, "HKG")

	forwarder := NewForwarder(pool)
	forwarder.SetDial(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("拨号失败")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = forwarder.Listen(ctx, "127.0.0.1:0") }()

	conn, err := net.Dial("tcp", waitForAddr(t, forwarder))
	if err != nil {
		t.Fatalf("连接转发器失败: %v", err)
	}
	defer conn.Close()

	if rep := socks5Connect(t, conn, "example.com", 443); rep != socks5RepHostUnreachable {
		t.Fatalf("want %#x, got %#x", socks5RepHostUnreachable, rep)
	}
	if pool.Contains("1.2.3.4") {
		t.Error("拨号失败的节点应被移出池")
	}
}
