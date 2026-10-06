// Package sublog keeps an in-memory ring buffer of subscription link access
// events for the admin panel. Buffer is capped and cleared on process restart.
package sublog

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const maxEvents = 500

// Event is one subscription fetch attempt.
type Event struct {
	At     time.Time `json:"at"`
	IP     string    `json:"ip"`
	User   string    `json:"user"`  // resolved owner, or "invalid" for unknown tokens
	Token  string    `json:"token"` // first 8 hex chars of the presented token
	UA     string    `json:"ua"`
	Status int       `json:"status"`
}

var (
	mu     sync.Mutex
	events []Event
)

// Record appends one event, evicting the oldest beyond the cap.
func Record(ip, user, token, ua string, status int) {
	if len(token) > 8 {
		token = token[:8]
	}
	ua = strings.TrimSpace(ua)
	if len(ua) > 60 {
		ua = ua[:60]
	}
	mu.Lock()
	defer mu.Unlock()
	events = append(events, Event{At: time.Now(), IP: ip, User: user, Token: token, UA: ua, Status: status})
	if len(events) > maxEvents {
		events = events[len(events)-maxEvents:]
	}
}

// Recent returns up to n events, newest first.
func Recent(n int) []Event {
	mu.Lock()
	defer mu.Unlock()
	if n > len(events) {
		n = len(events)
	}
	out := make([]Event, 0, n)
	for i := len(events) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, events[i])
	}
	return out
}

// ClientIP resolves the real client address, preferring tunnel headers so
// Cloudflare-fronted requests log the visitor instead of the tunnel.
func ClientIP(r *http.Request) string {
	if v := r.Header.Get("CF-Connecting-IP"); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
