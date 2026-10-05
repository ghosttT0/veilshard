package server

import (
	"context"
	"fmt"
	"net"
	"time"
)

type OutboundDialer struct {
	dialer *net.Dialer
}

func NewOutboundDialer() *OutboundDialer {
	return &OutboundDialer{
		dialer: &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}
}

// DialTarget connects directly to the requested internet destination.
func (d *OutboundDialer) DialTarget(ctx context.Context, target string) (net.Conn, error) {
	conn, err := d.dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return nil, fmt.Errorf("dial target %s: %w", target, err)
	}
	return conn, nil
}
