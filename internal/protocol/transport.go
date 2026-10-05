package protocol

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrConnectionClosed = errors.New("transport connection closed")
)

type TransportType string

const (
	TransportUDP TransportType = "udp"
	TransportTCP TransportType = "tcp"
)

// DualTransport manages an active session that can hot-switch between UDP and TCP streams.
type DualTransport struct {
	mu             sync.RWMutex
	currentType    TransportType
	cipher         *SessionCipher
	morph          *MorphEngine
	probe          *LinkProbe
	outboundFrames chan *Frame
	closed         bool
	onModeSwitch   func(oldMode, newMode TransportType)
}

func NewDualTransport(cipher *SessionCipher, profile ProfileType, onSwitch func(oldMode, newMode TransportType)) *DualTransport {
	return &DualTransport{
		currentType:    TransportUDP,
		cipher:         cipher,
		morph:          NewMorphEngine(profile),
		probe:          NewLinkProbe("udp"),
		outboundFrames: make(chan *Frame, 256),
		onModeSwitch:   onSwitch,
	}
}

// CurrentMode returns current active transport ("udp" or "tcp").
func (dt *DualTransport) CurrentMode() TransportType {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	return dt.currentType
}

// Send prepares a frame, applies statistical traffic morphing padding,
// encrypts it via AEAD, and registers metrics for the link probe.
func (dt *DualTransport) Send(streamID uint32, data []byte, fin bool) ([]byte, error) {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	if dt.closed {
		return nil, ErrConnectionClosed
	}

	// 1. Calculate dynamic padding based on chosen profile (WebRTC / HTTPS / Uniform)
	paddingLen := dt.morph.CalculatePadding(len(data))

	// 2. Construct protocol frame
	frame := NewDataFrame(streamID, data, paddingLen, fin)
	rawBytes := frame.Encode()

	// 3. Encrypt payload with AEAD
	encryptedPayload := dt.cipher.Encrypt(rawBytes)

	// 4. Record metric
	dt.probe.RecordSent()

	return encryptedPayload, nil
}

// Receive decrypts incoming packet and reconstructs the protocol Frame.
func (dt *DualTransport) Receive(ciphertext []byte) (*Frame, error) {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	if dt.closed {
		return nil, ErrConnectionClosed
	}

	// 1. Decrypt & authenticate payload
	plaintext, err := dt.cipher.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt packet: %w", err)
	}

	// 2. Decode wire frame
	frame, err := Decode(plaintext)
	if err != nil {
		return nil, fmt.Errorf("decode frame: %w", err)
	}

	return frame, nil
}

// ReportLoss informs the probe of a dropped packet or timeout.
// Triggers automatic fallback to TCP if conditions degrade.
func (dt *DualTransport) ReportLoss() {
	action := dt.probe.RecordLoss()
	if action == ActionDowngradeToTCP {
		dt.switchMode(TransportTCP)
	}
}

// ReportAck informs the probe of a successful delivery with measured RTT.
// May trigger automatic upgrade back to UDP when network heals.
func (dt *DualTransport) ReportAck(rtt time.Duration) {
	action := dt.probe.RecordAck(rtt)
	if action == ActionUpgradeToUDP {
		dt.switchMode(TransportUDP)
	}
}

func (dt *DualTransport) switchMode(newType TransportType) {
	dt.mu.Lock()
	oldType := dt.currentType
	if oldType == newType {
		dt.mu.Unlock()
		return
	}
	dt.currentType = newType
	dt.mu.Unlock()

	if dt.onModeSwitch != nil {
		dt.onModeSwitch(oldType, newType)
	}
}

// Stats returns link quality and traffic metrics.
func (dt *DualTransport) Stats() LinkStats {
	return dt.probe.GetStats()
}

// Close gracefully closes the dual transport.
func (dt *DualTransport) Close(ctx context.Context) error {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.closed = true
	return nil
}
