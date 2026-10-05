package server

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
	ListenAddr string
	SecretKey  string
	Profile    protocol.ProfileType
}

type Server struct {
	cfg      Config
	listener net.Listener
	outbound *OutboundDialer
	closed   atomic.Bool
	wg       sync.WaitGroup
}

func New(cfg Config) *Server {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "0.0.0.0:8443"
	}
	if cfg.Profile == "" {
		cfg.Profile = protocol.ProfileWebRTC
	}
	return &Server{
		cfg:      cfg,
		outbound: NewOutboundDialer(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.cfg.ListenAddr, err)
	}
	s.listener = l
	log.Printf("[Nexus-Server] Listening for encrypted client tunnels on %s (Profile: %s)", s.cfg.ListenAddr, s.cfg.Profile)

	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			if s.closed.Load() {
				return nil
			}
			log.Printf("[Nexus-Server] Accept error: %v", err)
			continue
		}

		s.wg.Add(1)
		go func(clientConn net.Conn) {
			defer s.wg.Done()
			s.handleClient(clientConn)
		}(conn)
	}
}

func (s *Server) handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	// 1. Authenticated handshake
	cipher, err := tunnel.ServerHandshake(clientConn, s.cfg.SecretKey)
	if err != nil {
		log.Printf("[Nexus-Server] Authentication failed from %s: %v", clientConn.RemoteAddr(), err)
		return
	}

	morph := protocol.NewMorphEngine(s.cfg.Profile)

	// 2. Read target address frame
	firstFrame, err := tunnel.ReadFrame(clientConn, cipher)
	if err != nil {
		log.Printf("[Nexus-Server] Read target frame failed: %v", err)
		return
	}

	target := string(firstFrame.Data)
	streamID := firstFrame.StreamID

	// 3. Connect to target internet destination
	ctx := context.Background()
	targetConn, err := s.outbound.DialTarget(ctx, target)
	if err != nil {
		log.Printf("[Nexus-Server] Dial target %s failed: %v", target, err)
		return
	}
	defer targetConn.Close()

	// 4. Bidirectional pump
	errCh := make(chan error, 2)

	// Client tunnel -> Target Internet destination
	go func() {
		for {
			frame, err := tunnel.ReadFrame(clientConn, cipher)
			if err != nil {
				errCh <- err
				return
			}
			if len(frame.Data) > 0 {
				if _, writeErr := targetConn.Write(frame.Data); writeErr != nil {
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

	// Target Internet destination -> Client tunnel (with morphing padding)
	go func() {
		buf := make([]byte, 16384)
		for {
			n, err := targetConn.Read(buf)
			if n > 0 {
				writeErr := tunnel.WriteFrame(clientConn, cipher, morph, streamID, buf[:n], false)
				if writeErr != nil {
					errCh <- writeErr
					return
				}
			}
			if err != nil {
				_ = tunnel.WriteFrame(clientConn, cipher, morph, streamID, nil, true)
				errCh <- err
				return
			}
		}
	}()

	<-errCh
}

func (s *Server) Close() error {
	s.closed.Store(true)
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.wg.Wait()
	return nil
}
