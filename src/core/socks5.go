package core

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
)

// SOCKS5 协议常量（RFC 1928）
const (
	socks5Version = 0x05

	socks5AuthNone     = 0x00
	socks5AuthRejected = 0xFF

	socks5CmdConnect = 0x01

	socks5AddrIPv4   = 0x01
	socks5AddrDomain = 0x03
	socks5AddrIPv6   = 0x04

	socks5RepSucceeded        = 0x00
	socks5RepHostUnreachable  = 0x04
	socks5RepCmdNotSupported  = 0x07
	socks5RepAddrNotSupported = 0x08
)

var (
	errSocks5Version      = errors.New("不支持的 SOCKS 版本")
	errSocks5Auth         = errors.New("客户端不支持无认证方式")
	errSocks5Cmd          = errors.New("仅支持 CONNECT 命令")
	errSocks5AddrType     = errors.New("不支持的地址类型")
)

// Socks5Request 客户端请求连接的目标
type Socks5Request struct {
	Host string
	Port uint16
}

// negotiate 完成方法协商：只接受「无认证」，否则回绝并返回错误。
func negotiate(conn net.Conn) error {
	head := make([]byte, 2)
	if _, err := io.ReadFull(conn, head); err != nil {
		return err
	}
	if head[0] != socks5Version {
		return errSocks5Version
	}

	methods := make([]byte, int(head[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}

	for _, method := range methods {
		if method == socks5AuthNone {
			_, err := conn.Write([]byte{socks5Version, socks5AuthNone})
			return err
		}
	}

	_, _ = conn.Write([]byte{socks5Version, socks5AuthRejected})
	return errSocks5Auth
}

// readRequest 读取并解析 CONNECT 请求。请求不合法时已回送应答码。
func readRequest(conn net.Conn) (Socks5Request, error) {
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return Socks5Request{}, err
	}
	if head[0] != socks5Version {
		return Socks5Request{}, errSocks5Version
	}
	if head[1] != socks5CmdConnect {
		_ = writeReply(conn, socks5RepCmdNotSupported)
		return Socks5Request{}, errSocks5Cmd
	}

	host, err := readHost(conn, head[3])
	if err != nil {
		return Socks5Request{}, err
	}

	port := make([]byte, 2)
	if _, err := io.ReadFull(conn, port); err != nil {
		return Socks5Request{}, err
	}
	return Socks5Request{Host: host, Port: binary.BigEndian.Uint16(port)}, nil
}

func readHost(conn net.Conn, addrType byte) (string, error) {
	switch addrType {
	case socks5AddrIPv4:
		buf := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return net.IP(buf).String(), nil

	case socks5AddrIPv6:
		buf := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return net.IP(buf).String(), nil

	case socks5AddrDomain:
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return "", err
		}
		buf := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return string(buf), nil

	default:
		_ = writeReply(conn, socks5RepAddrNotSupported)
		return "", errSocks5AddrType
	}
}

// writeReply 写回应答。绑定地址填 0.0.0.0:0——客户端不会用到它。
func writeReply(w io.Writer, rep byte) error {
	_, err := w.Write([]byte{
		socks5Version, rep, 0x00,
		socks5AddrIPv4, 0, 0, 0, 0,
		0, 0,
	})
	return err
}
