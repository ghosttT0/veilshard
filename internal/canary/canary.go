package canary

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/veilshard/veilshard/internal/fleet"
)

type NodeHealthState string

const (
	StateAlive         NodeHealthState = "ALIVE"          // TCP + TLS Handshake OK (Green)
	StateBlockedRST    NodeHealthState = "TCP_RST/BLOCKED" // Handshake failed / TCP RST received (Red)
	StateQoSThrottled  NodeHealthState = "HIGH_LATENCY/QOS" // High packet delay > 600ms (Yellow)
	StateTimeout       NodeHealthState = "TIMEOUT/DROP"    // Total drop (Red)
)

type NodeHealthResult struct {
	Name        string          `json:"name"`
	Host        string          `json:"host"`
	Port        int             `json:"port"`
	SNI         string          `json:"sni"`
	State       NodeHealthState `json:"state"`
	TCPRTT      time.Duration   `json:"tcp_rtt"`
	TLSRTT      time.Duration   `json:"tls_rtt"`
	TotalRTT    time.Duration   `json:"total_rtt"`
	ErrorMessage string         `json:"error_message,omitempty"`
}

// ProbeNodeFromClientSide performs realistic client-perspective inspection on a node.
func ProbeNodeFromClientSide(node fleet.NodeConfig, timeout time.Duration) *NodeHealthResult {
	res := &NodeHealthResult{
		Name: node.Name,
		Host: node.Host,
		Port: node.Port,
		SNI:  node.SNITarget,
	}

	if res.SNI == "" || res.SNI == "auto" {
		res.SNI = "gateway.icloud.com"
	}
	if strings.Contains(res.SNI, ":") {
		res.SNI = strings.Split(res.SNI, ":")[0]
	}

	target := fmt.Sprintf("%s:%d", node.Host, node.Port)

	// Phase 1: Raw TCP Connection
	tcpStart := time.Now()
	tcpDialer := &net.Dialer{Timeout: timeout}
	rawConn, err := tcpDialer.Dial("tcp", target)
	if err != nil {
		res.TCPRTT = time.Since(tcpStart)
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "reset") {
			res.State = StateBlockedRST
		} else {
			res.State = StateTimeout
		}
		res.ErrorMessage = err.Error()
		return res
	}
	res.TCPRTT = time.Since(tcpStart)

	// Phase 2: TLS 1.3 ClientHello Probe (simulating REALITY client)
	tlsConfig := &tls.Config{
		ServerName:         res.SNI,
		InsecureSkipVerify: true,
	}

	tlsStart := time.Now()
	tlsConn := tls.Client(rawConn, tlsConfig)
	_ = tlsConn.SetDeadline(time.Now().Add(timeout))

	err = tlsConn.Handshake()
	res.TLSRTT = time.Since(tlsStart)
	res.TotalRTT = res.TCPRTT + res.TLSRTT
	_ = tlsConn.Close()

	if err != nil {
		res.State = StateBlockedRST
		res.ErrorMessage = fmt.Sprintf("TLS handshake blocked: %v", err)
		return res
	}

	// Phase 3: Evaluate latency and quality
	if res.TotalRTT > 700*time.Millisecond {
		res.State = StateQoSThrottled
	} else {
		res.State = StateAlive
	}

	return res
}

// ProbeFleetHealth benchmarks all fleet nodes in parallel.
func ProbeFleetHealth(nodes []fleet.NodeConfig, timeout time.Duration) []*NodeHealthResult {
	results := make([]*NodeHealthResult, len(nodes))
	var wg sync.WaitGroup

	for i, n := range nodes {
		wg.Add(1)
		go func(idx int, node fleet.NodeConfig) {
			defer wg.Done()
			results[idx] = ProbeNodeFromClientSide(node, timeout)
		}(i, n)
	}

	wg.Wait()
	return results
}
