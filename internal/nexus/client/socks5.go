package client

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
)

var (
	ErrNotSocks5        = errors.New("unsupported protocol, expected SOCKS5")
	ErrUnsupportedCmd   = errors.New("unsupported SOCKS5 command, only CONNECT is supported")
	ErrUnsupportedAtyp  = errors.New("unsupported address type")
)

// TargetAddress represents the parsed remote destination requested by the SOCKS5 client.
type TargetAddress struct {
	Host string // FQDN or IP
	Port int
}

func (t TargetAddress) String() string {
	return net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
}

// HandleSocks5Handshake completes RFC 1928 negotiation and extracts target address.
func HandleSocks5Handshake(conn net.Conn) (*TargetAddress, error) {
	// 1. Negotiation
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	if header[0] != 0x05 {
		return nil, ErrNotSocks5
	}

	numMethods := int(header[1])
	methods := make([]byte, numMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return nil, err
	}

	// Reply NO_AUTH (0x00)
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return nil, err
	}

	// 2. Request
	req := make([]byte, 4)
	if _, err := io.ReadFull(conn, req); err != nil {
		return nil, err
	}

	if req[0] != 0x05 {
		return nil, ErrNotSocks5
	}
	if req[1] != 0x01 { // 0x01 = CONNECT
		// Reply Command Not Supported (0x07)
		_, _ = conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return nil, ErrUnsupportedCmd
	}

	atyp := req[3]
	var host string

	switch atyp {
	case 0x01: // IPv4
		ipBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, ipBuf); err != nil {
			return nil, err
		}
		host = net.IP(ipBuf).String()

	case 0x03: // Domain name
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return nil, err
		}
		domainLen := int(lenBuf[0])
		domainBuf := make([]byte, domainLen)
		if _, err := io.ReadFull(conn, domainBuf); err != nil {
			return nil, err
		}
		host = string(domainBuf)

	case 0x04: // IPv6
		ipBuf := make([]byte, 16)
		if _, err := io.ReadFull(conn, ipBuf); err != nil {
			return nil, err
		}
		host = net.IP(ipBuf).String()

	default:
		_, _ = conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return nil, ErrUnsupportedAtyp
	}

	// Port
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return nil, err
	}
	port := int(binary.BigEndian.Uint16(portBuf))

	// Reply Success
	// 0x05 (ver), 0x00 (success), 0x00 (rsv), 0x01 (IPv4), [0.0.0.0], [port 0]
	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return nil, err
	}

	return &TargetAddress{
		Host: host,
		Port: port,
	}, nil
}
