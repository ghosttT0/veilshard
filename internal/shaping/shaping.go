package shaping

import (
	"crypto/rand"
	"math/big"
	"time"
)

// Profile represents a traffic distribution model.
type Profile string

const (
	ProfileWebBrowsing Profile = "web_browsing" // High variance, simulates HTML/JSON/Image chunking
	ProfileStreaming   Profile = "streaming"    // Balanced jitter for media streams
	ProfileConservative Profile = "conservative"// Minimal overhead micro-padding
)

// Shaper defines strategies for packet padding and timing obfuscation.
type Shaper struct {
	Profile    Profile
	MaxPadding int
}

func NewShaper(p Profile) *Shaper {
	maxPad := 256
	if p == ProfileWebBrowsing {
		maxPad = 512
	}
	return &Shaper{
		Profile:    p,
		MaxPadding: maxPad,
	}
}

// CalculateAdaptivePadding computes randomized, non-linear padding bytes
// to prevent fixed MTU clustering and smooth packet length histogram spikes.
func (s *Shaper) CalculateAdaptivePadding(payloadLen int) int {
	if s.MaxPadding <= 0 {
		return 0
	}

	// Quantum bucketing: align to organic multiples (e.g., 64, 128, 256)
	remainder := payloadLen % 64
	neededToBucket := 0
	if remainder > 0 {
		neededToBucket = 64 - remainder
	}

	// Add random entropy jitter [16, MaxPadding]
	n, err := rand.Int(rand.Reader, big.NewInt(int64(s.MaxPadding-16)))
	jitter := 16
	if err == nil {
		jitter += int(n.Int64())
	}

	totalPad := neededToBucket + jitter
	if totalPad > s.MaxPadding {
		totalPad = s.MaxPadding
	}
	return totalPad
}

// CalculateInterArrivalJitter generates subtle micro-delay (1-15ms)
// for the handshake sequence and initial packets to defeat timing classifiers.
func (s *Shaper) CalculateInterArrivalJitter() time.Duration {
	n, err := rand.Int(rand.Reader, big.NewInt(14))
	if err != nil {
		return 2 * time.Millisecond
	}
	return time.Duration(1+n.Int64()) * time.Millisecond
}

// ReshapePacket returns total padded slice with random noise bytes.
func ReshapePacket(original []byte, padLen int) []byte {
	if padLen <= 0 {
		return original
	}
	res := make([]byte, len(original)+padLen)
	copy(res, original)

	// Fill padding with cryptographic pseudo-random bytes
	_, _ = rand.Read(res[len(original):])
	return res
}
