package fingerprint

import (
	"strings"
)

const (
	PlatformAuto    = "auto"
	PlatformIOS     = "ios"
	PlatformDesktop = "desktop"
	PlatformAndroid = "android"
)

// ProfileOptions holds platform-tailored client fingerprint specifications.
type ProfileOptions struct {
	Platform          string   `json:"platform"`
	ClientFingerprint string   `json:"client_fingerprint"`
	ALPN              []string `json:"alpn"`
	MinPadding        int      `json:"min_padding"`
	MaxPadding        int      `json:"max_padding"`
}

// ResolveOptions returns aligned fingerprint and padding settings for the target platform.
func ResolveOptions(platform string) ProfileOptions {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformIOS, "iphone", "ipad", "shadowrocket", "loon", "quantumult":
		return ProfileOptions{
			Platform:          PlatformIOS,
			ClientFingerprint: "safari",
			ALPN:              []string{"h2", "http/1.1"},
			MinPadding:        100,
			MaxPadding:        300,
		}
	case PlatformAndroid:
		return ProfileOptions{
			Platform:          PlatformAndroid,
			ClientFingerprint: "chrome",
			ALPN:              []string{"h2", "http/1.1"},
			MinPadding:        100,
			MaxPadding:        400,
		}
	case "firefox":
		return ProfileOptions{
			Platform:          PlatformDesktop,
			ClientFingerprint: "firefox",
			ALPN:              []string{"h2", "http/1.1"},
			MinPadding:        100,
			MaxPadding:        500,
		}
	case PlatformDesktop, "windows", "macos", "linux", "clash", "mihomo":
		fallthrough
	default:
		return ProfileOptions{
			Platform:          PlatformDesktop,
			ClientFingerprint: "chrome",
			ALPN:              []string{"h2", "http/1.1"},
			MinPadding:        100,
			MaxPadding:        500,
		}
	}
}

// DetectPlatformFromUserAgent dynamically classifies client platform based on incoming HTTP UA.
func DetectPlatformFromUserAgent(ua string) string {
	lower := strings.ToLower(ua)
	if strings.Contains(lower, "shadowrocket") ||
		strings.Contains(lower, "quantumult") ||
		strings.Contains(lower, "loon") ||
		strings.Contains(lower, "stash") ||
		strings.Contains(lower, "iphone") ||
		strings.Contains(lower, "ipad") ||
		strings.Contains(lower, "cfnetwork") {
		return PlatformIOS
	}
	if strings.Contains(lower, "android") || strings.Contains(lower, "v2rayng") {
		return PlatformAndroid
	}
	return PlatformDesktop
}

// SanitizeParameters validates and strips unconventional or risky DPI parameters.
func SanitizeParameters(fingerprint string) string {
	allowed := map[string]bool{
		"chrome":     true,
		"firefox":    true,
		"safari":     true,
		"ios":        true,
		"edge":       true,
		"qq":         true,
		"360":        true,
		"android":    true,
		"random":     false, // "random" fingerprint generates non-standard TLS signatures that DPI easily spots
		"randomized": false,
	}

	lower := strings.ToLower(strings.TrimSpace(fingerprint))
	if allowed[lower] {
		return lower
	}
	return "chrome" // Default safe fallback
}
