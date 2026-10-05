package fingerprint

import (
	"testing"
)

func TestResolveOptions(t *testing.T) {
	iosOpts := ResolveOptions(PlatformIOS)
	if iosOpts.ClientFingerprint != "safari" {
		t.Errorf("expected safari for iOS, got %s", iosOpts.ClientFingerprint)
	}

	desktopOpts := ResolveOptions(PlatformDesktop)
	if desktopOpts.ClientFingerprint != "chrome" {
		t.Errorf("expected chrome for desktop, got %s", desktopOpts.ClientFingerprint)
	}

	firefoxOpts := ResolveOptions("firefox")
	if firefoxOpts.ClientFingerprint != "firefox" {
		t.Errorf("expected firefox for firefox, got %s", firefoxOpts.ClientFingerprint)
	}
}

func TestDetectPlatformFromUserAgent(t *testing.T) {
	tests := []struct {
		ua       string
		expected string
	}{
		{"Shadowrocket/2.2.34 (iOS 17.5.1)", PlatformIOS},
		{"Quantumult%20X/1.4.2 (iPhone15,2)", PlatformIOS},
		{"Loon/3.1.2 CFNetwork/1496.0.7 Darwin/23.5.0", PlatformIOS},
		{"Clash.Meta/v1.18.7 linux amd64", PlatformDesktop},
		{"Mihomo/1.18.7 darwin arm64", PlatformDesktop},
		{"v2rayNG/1.8.19 (Linux; Android 14)", PlatformAndroid},
	}

	for _, tt := range tests {
		got := DetectPlatformFromUserAgent(tt.ua)
		if got != tt.expected {
			t.Errorf("DetectPlatformFromUserAgent(%q) = %s, want %s", tt.ua, got, tt.expected)
		}
	}
}

func TestSanitizeParameters(t *testing.T) {
	if SanitizeParameters("random") != "chrome" {
		t.Errorf("expected random to be sanitized to chrome")
	}
	if SanitizeParameters("safari") != "safari" {
		t.Errorf("expected safari to be accepted")
	}
}
