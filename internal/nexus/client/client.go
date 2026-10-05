package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/veilshard/veilshard/internal/nexus/tunnel"
	"github.com/veilshard/veilshard/internal/protocol"
)

type Config struct {
	LocalAddr  string
	ServerAddr string
	SecretKey  string
	Profile    protocol.ProfileType
}

type Client struct {
	cfg       Config
	listener  net.Listener
	streamSeq uint32
	closed    atomic.Bool
	wg        sync.WaitGroup
}

func New(cfg Config) *Client {
	if cfg.LocalAddr == "" {
		cfg.LocalAddr = "127.0.0.1:1080"
	}
	if cfg.Profile == "" {
		cfg.Profile = protocol.ProfileWebRTC
	}
	return &Client{cfg: cfg}
}

func (c *Client) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", c.cfg.LocalAddr)
	if err != nil {
		return fmt.Errorf("listen socks5 on %s: %w", c.cfg.LocalAddr, err)
	}
	c.listener = l
	log.Printf("[Nexus-Client] Local SOCKS5 listening on %s -> Remote Server: %s", c.cfg.LocalAddr, c.cfg.ServerAddr)

	go func() {
		<-ctx.Done()
		_ = c.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			if c.closed.Load() {
				return nil
			}
			log.Printf("[Nexus-Client] Accept error: %v", err)
			continue
		}

		c.wg.Add(1)
		go func(localConn net.Conn) {
			defer c.wg.Done()
			c.handleConnection(localConn)
		}(conn)
	}
}

func (c *Client) handleConnection(localConn net.Conn) {
	defer localConn.Close()

	// 1. SOCKS5 negotiation
	target, err := HandleSocks5Handshake(localConn)
	if err != nil {
		return
	}

	streamID := atomic.AddUint32(&c.streamSeq, 1)

	// 2. Connect to remote Nexus server
	remoteConn, err := net.Dial("tcp", c.cfg.ServerAddr)
	if err != nil {
		log.Printf("[Nexus-Client] Dial remote server %s failed: %v", c.cfg.ServerAddr, err)
		return
	}
	defer remoteConn.Close()

	// 3. Authenticate with X25519 & AEAD
	cipher, err := tunnel.ClientHandshake(remoteConn, c.cfg.SecretKey)
	if err != nil {
		log.Printf("[Nexus-Client] Tunnel handshake failed: %v", err)
		return
	}

	morph := protocol.NewMorphEngine(c.cfg.Profile)

	// 4. Send target address frame
	targetStr := target.String()
	if err := tunnel.WriteFrame(remoteConn, cipher, morph, streamID, []byte(targetStr), false); err != nil {
		return
	}

	// 5. Bidirectional pump
	errCh := make(chan error, 2)

	// Local SOCKS5 -> Remote Tunnel
	go func() {
		buf := make([]byte, 16384)
		for {
			n, err := localConn.Read(buf)
			if n > 0 {
				writeErr := tunnel.WriteFrame(remoteConn, cipher, morph, streamID, buf[:n], false)
				if writeErr != nil {
					errCh <- writeErr
					return
				}
			}
			if err != nil {
				// Send FIN
				_ = tunnel.WriteFrame(remoteConn, cipher, morph, streamID, nil, true)
				errCh <- err
				return
			}
		}
	}()

	// Remote Tunnel -> Local SOCKS5
	go func() {
		for {
			frame, err := tunnel.ReadFrame(remoteConn, cipher)
			if err != nil {
				errCh <- err
				return
			}
			if len(frame.Data) > 0 {
				if _, writeErr := localConn.Write(frame.Data); writeErr != nil {
					errCh <- writeErr
					return
				}
			}
			if frame.Flags&protocol.FlagStreamFIN != 0 {
				errCh <- io.EOF
				return
			}
		}
	}()

	<-errCh
}

func (c *Client) Close() error {
	c.closed.Store(true)
	if c.listener != nil {
		_ = c.listener.Close()
	}
	c.wg.Wait()
	return nil
}
