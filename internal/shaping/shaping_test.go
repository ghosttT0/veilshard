package shaping

import (
	"testing"
	"time"
)

func TestCalculateAdaptivePadding(t *testing.T) {
	shaper := NewShaper(ProfileWebBrowsing)

	pad1 := shaper.CalculateAdaptivePadding(100)
	if pad1 < 16 || pad1 > shaper.MaxPadding {
		t.Errorf("padding out of expected range: %d", pad1)
	}

	pad2 := shaper.CalculateAdaptivePadding(1440)
	if pad2 < 16 || pad2 > shaper.MaxPadding {
		t.Errorf("padding out of expected range: %d", pad2)
	}
}

func TestCalculateInterArrivalJitter(t *testing.T) {
	shaper := NewShaper(ProfileWebBrowsing)
	for i := 0; i < 10; i++ {
		jitter := shaper.CalculateInterArrivalJitter()
		if jitter < 1*time.Millisecond || jitter > 15*time.Millisecond {
			t.Errorf("jitter out of range: %v", jitter)
		}
	}
}

func TestReshapePacket(t *testing.T) {
	orig := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")
	padded := ReshapePacket(orig, 64)
	if len(padded) != len(orig)+64 {
		t.Errorf("expected len %d, got %d", len(orig)+64, len(padded))
	}
	if string(padded[:len(orig)]) != string(orig) {
		t.Errorf("original prefix modified")
	}
}
