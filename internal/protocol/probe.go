package protocol

import (
	"sync"
	"time"
)

type TransportAction int

const (
	ActionStay TransportAction = iota
	ActionDowngradeToTCP
	ActionUpgradeToUDP
)

// LinkStats holds statistical link quality metrics.
type LinkStats struct {
	Mode               string
	LossRate           float64
	SmoothedRTT        time.Duration
	ConsecutiveTimeouts int
	TotalPacketsSent   uint64
	TotalPacketsLost   uint64
}

// LinkProbe monitors network quality and detects carrier UDP QoS throttling.
type LinkProbe struct {
	mu                  sync.RWMutex
	currentMode         string
	windowSize          int
	sentWindow          []int64
	lossWindow          []int64
	head                int
	consecutiveTimeouts int
	smoothedRTT         time.Duration
	minRTT              time.Duration
	totalSent           uint64
	totalLost           uint64
}

func NewLinkProbe(initialMode string) *LinkProbe {
	if initialMode == "" {
		initialMode = "udp"
	}
	return &LinkProbe{
		currentMode: initialMode,
		windowSize:  30, // 30-sample sliding window
		sentWindow:  make([]int64, 30),
		lossWindow:  make([]int64, 30),
		minRTT:      1000 * time.Second,
	}
}

// RecordSent registers a dispatched probe or data packet.
func (p *LinkProbe) RecordSent() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalSent++
	p.sentWindow[p.head]++
}

// RecordAck registers a successful round-trip response with measured RTT.
func (p *LinkProbe) RecordAck(rtt time.Duration) TransportAction {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.consecutiveTimeouts = 0

	// EWMA (Exponential Weighted Moving Average) for RTT: RTT = 0.875 * RTT + 0.125 * sample
	if p.smoothedRTT == 0 {
		p.smoothedRTT = rtt
	} else {
		p.smoothedRTT = time.Duration(0.875*float64(p.smoothedRTT) + 0.125*float64(rtt))
	}

	if rtt < p.minRTT {
		p.minRTT = rtt
	}

	// Advance window periodically
	p.advanceWindow()

	lossRate := p.calcLossRate()

	// If currently on TCP, but UDP canary probes show stable link (< 5% loss)
	if p.currentMode == "tcp" && lossRate < 0.05 {
		p.currentMode = "udp"
		return ActionUpgradeToUDP
	}

	return ActionStay
}

// RecordLoss registers a timed-out packet or dropped sequence.
func (p *LinkProbe) RecordLoss() TransportAction {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalLost++
	p.lossWindow[p.head]++
	p.consecutiveTimeouts++
	p.advanceWindow()

	lossRate := p.calcLossRate()

	// If currently on UDP, and experiencing high loss (> 20%) or multiple consecutive timeouts
	if p.currentMode == "udp" {
		if p.consecutiveTimeouts >= 3 || lossRate > 0.20 {
			p.currentMode = "tcp"
			return ActionDowngradeToTCP
		}
	}

	return ActionStay
}

func (p *LinkProbe) advanceWindow() {
	p.head = (p.head + 1) % p.windowSize
	p.sentWindow[p.head] = 0
	p.lossWindow[p.head] = 0
}

func (p *LinkProbe) calcLossRate() float64 {
	var totalS, totalL int64
	for i := 0; i < p.windowSize; i++ {
		totalS += p.sentWindow[i]
		totalL += p.lossWindow[i]
	}
	if totalS == 0 {
		return 0.0
	}
	return float64(totalL) / float64(totalS)
}

// GetStats snapshots current network health.
func (p *LinkProbe) GetStats() LinkStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return LinkStats{
		Mode:               p.currentMode,
		LossRate:           p.calcLossRate(),
		SmoothedRTT:        p.smoothedRTT,
		ConsecutiveTimeouts: p.consecutiveTimeouts,
		TotalPacketsSent:   p.totalSent,
		TotalPacketsLost:   p.totalLost,
	}
}
