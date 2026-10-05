package protocol

import (
	"crypto/rand"
	"math/big"
)

type ProfileType string

const (
	ProfileWebRTC  ProfileType = "webrtc"  // Mimic Zoom/Teams RTP audio/video frames
	ProfileHTTPS   ProfileType = "https"   // Mimic standard web browsing / HLS chunk streaming
	ProfileUniform ProfileType = "uniform" // Quantized bucket padding to flatten length histogram
)

// MorphEngine computes dynamic packet padding to disguise traffic statistical distributions.
type MorphEngine struct {
	profile ProfileType
}

func NewMorphEngine(profile ProfileType) *MorphEngine {
	if profile == "" {
		profile = ProfileWebRTC
	}
	return &MorphEngine{profile: profile}
}

// CalculatePadding computes the exact number of dummy bytes to append to the frame
// so that the resulting packet fits the target statistical distribution.
func (m *MorphEngine) CalculatePadding(actualDataLen int) int {
	switch m.profile {
	case ProfileWebRTC:
		return m.webrtcPadding(actualDataLen)
	case ProfileHTTPS:
		return m.httpsPadding(actualDataLen)
	case ProfileUniform:
		fallthrough
	default:
		return m.bucketPadding(actualDataLen, 128)
	}
}

// webrtcPadding simulates WebRTC voice (100-240B) and video frames (600-1350B).
func (m *MorphEngine) webrtcPadding(actual int) int {
	if actual < 200 {
		// Pad to voice frame size (200 - 240B)
		target := 200 + randInt(40)
		if target > actual {
			return target - actual
		}
	} else if actual < 800 {
		// Pad to video P-frame size (800 - 900B)
		target := 800 + randInt(100)
		if target > actual {
			return target - actual
		}
	} else if actual < 1350 {
		// Pad to standard video packet MTU size (1300 - 1380B)
		target := 1300 + randInt(80)
		if target > actual {
			return target - actual
		}
	}
	// Small jitter padding for large packets
	return randInt(32)
}

// httpsPadding simulates typical HTTPS client requests (400-800B) and server responses (1380-1420B).
func (m *MorphEngine) httpsPadding(actual int) int {
	if actual < 500 {
		target := 500 + randInt(150)
		if target > actual {
			return target - actual
		}
	} else if actual < 1380 {
		target := 1380 + randInt(40)
		if target > actual {
			return target - actual
		}
	}
	return randInt(16)
}

// bucketPadding quantizes length into fixed buckets (e.g. multiples of 64 or 128)
// with small randomized noise to erase statistical peaks.
func (m *MorphEngine) bucketPadding(actual int, bucketSize int) int {
	if bucketSize <= 0 {
		bucketSize = 64
	}

	remainder := actual % bucketSize
	basePadding := 0
	if remainder != 0 {
		basePadding = bucketSize - remainder
	}

	// Add random noise of 0 to (bucketSize - 1)
	noise := randInt(bucketSize)
	return basePadding + noise
}

func randInt(max int) int {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}
