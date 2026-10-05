package nexus

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/veilshard/veilshard/internal/nexus/client"
	"github.com/veilshard/veilshard/internal/nexus/server"
	"github.com/veilshard/veilshard/internal/protocol"
)

func TestEndToEndProxyPipeline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secretKey := "test-nexus-secret-key-12345"

	// 1. Mock Target Server (Echo server)
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen echo server: %v", err)
	}
	defer echoListener.Close()
	echoAddr := echoListener.Addr().String()

	go func() {
		for {
			conn, err := echoListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c) // Echo back
			}(conn)
		}
	}()

	// 2. Start Nexus Remote Server
	serverListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen server: %v", err)
	}
	serverAddr := serverListener.Addr().String()
	_ = serverListener.Close()

	srv := server.New(server.Config{
		ListenAddr: serverAddr,
		SecretKey:  secretKey,
		Profile:    protocol.ProfileWebRTC,
	})

	go func() {
		_ = srv.Start(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// 3. Start Nexus Client (SOCKS5 Inbound)
	clientListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen client: %v", err)
	}
	socksAddr := clientListener.Addr().String()
	_ = clientListener.Close()

	cli := client.New(client.Config{
		LocalAddr:  socksAddr,
		ServerAddr: serverAddr,
		SecretKey:  secretKey,
		Profile:    protocol.ProfileWebRTC,
	})

	go func() {
		_ = cli.Start(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// 4. Dial through SOCKS5 to the Echo server
	socksConn, err := net.Dial("tcp", socksAddr)
	if err != nil {
		t.Fatalf("connect to socks5: %v", err)
	}
	defer socksConn.Close()

	// SOCKS5 Greeting: [0x05, 0x01, 0x00]
	if _, err := socksConn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write socks5 greeting: %v", err)
	}
	greetResp := make([]byte, 2)
	if _, err := io.ReadFull(socksConn, greetResp); err != nil {
		t.Fatalf("read socks5 greeting resp: %v", err)
	}
	if greetResp[0] != 0x05 || greetResp[1] != 0x00 {
		t.Fatalf("unexpected socks5 greeting resp: %x", greetResp)
	}

	// SOCKS5 Connect to echoAddr: [0x05, 0x01, 0x00, 0x01, IP (4B), Port (2B)]
	host, portStr, _ := net.SplitHostPort(echoAddr)
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)
	ip := net.ParseIP(host).To4()

	req := []byte{0x05, 0x01, 0x00, 0x01, ip[0], ip[1], ip[2], ip[3], byte(port >> 8), byte(port & 0xff)}
	if _, err := socksConn.Write(req); err != nil {
		t.Fatalf("write socks5 connect: %v", err)
	}

	connectResp := make([]byte, 10)
	if _, err := io.ReadFull(socksConn, connectResp); err != nil {
		t.Fatalf("read socks5 connect resp: %v", err)
	}
	if connectResp[1] != 0x00 {
		t.Fatalf("socks5 connect returned error code: %d", connectResp[1])
	}

	// 5. Send payload through SOCKS5 tunnel and read echo response
	testMessage := "Hello Nexus Proxy Protocol! High Speed, Secure, and Morphing!"
	if _, err := socksConn.Write([]byte(testMessage)); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	buf := make([]byte, len(testMessage))
	if _, err := io.ReadFull(socksConn, buf); err != nil {
		t.Fatalf("read echo response: %v", err)
	}

	if string(buf) != testMessage {
		t.Fatalf("echo mismatch: got %q, want %q", string(buf), testMessage)
	}
}
