// Package panel serves the admin web UI and its JSON API on the same port as
// the subscription server. Every route requires the admin key (X-Admin-Key
// header, ?key= query parameter, or panel cookie).
package panel

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/veilshard/veilshard/internal/coresync"
	"github.com/veilshard/veilshard/internal/exporter"
	"github.com/veilshard/veilshard/internal/exporter/qr"
	"github.com/veilshard/veilshard/internal/exporter/uri"
	"github.com/veilshard/veilshard/internal/traffic"
	"github.com/veilshard/veilshard/internal/sublog"
	"github.com/veilshard/veilshard/internal/users"
)

const (
	adminHeader   = "X-Admin-Key"
	sessionCookie = "vpnctl_session"
	sessionTTL    = 24 * time.Hour
)

type panelHandler struct {
	key      string
	sessions sync.Map // session token -> unix expiry
}

// Mount registers panel routes. Unauthenticated /panel requests receive
// nothing but a bare login form — no page markup, no data, no endpoints.
func Mount(mux *http.ServeMux, adminKey string) {
	if adminKey == "" {
		return
	}
	p := &panelHandler{key: adminKey}
	mux.HandleFunc("/panel", p.page)
	mux.HandleFunc("/panel/login", p.login)
	mux.HandleFunc("/panel/logout", p.logout)
	mux.HandleFunc("/api/users", p.auth(p.apiUsers))
	mux.HandleFunc("/api/logs", p.auth(p.apiLogs))
	mux.HandleFunc("/api/user/add", p.auth(p.apiAdd))
	mux.HandleFunc("/api/user/remove", p.auth(p.apiRemove))
	mux.HandleFunc("/api/user/toggle", p.auth(p.apiToggle))
	mux.HandleFunc("/api/user/quota", p.auth(p.apiQuota))
	mux.HandleFunc("/api/user/reset-token", p.auth(p.apiResetToken))
	mux.HandleFunc("/api/user/qr", p.auth(p.apiQR))
}

func secureHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

// newSession mints a 256-bit session token with a sliding server-side expiry.
func (p *panelHandler) newSession() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	p.sessions.Store(tok, time.Now().Add(sessionTTL).Unix())
	return tok, nil
}

func (p *panelHandler) validSession(tok string) bool {
	if tok == "" {
		return false
	}
	v, ok := p.sessions.Load(tok)
	if !ok {
		return false
	}
	if time.Now().Unix() > v.(int64) {
		p.sessions.Delete(tok)
		return false
	}
	return true
}

// authorized accepts either the admin key (X-Admin-Key, for CLI use) or a
// valid server-side session cookie (browser flow).
func (p *panelHandler) authorized(r *http.Request) bool {
	if k := r.Header.Get(adminHeader); k != "" && subtle.ConstantTimeCompare([]byte(k), []byte(p.key)) == 1 {
		return true
	}
	if c, err := r.Cookie(sessionCookie); err == nil && p.validSession(c.Value) {
		return true
	}
	return false
}

// auth guards the JSON API: authorized requests pass, everything else gets a
// jittered-delay 401 that reveals nothing.
func (p *panelHandler) auth(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secureHeaders(w)
		// Mutations must speak JSON: cross-site form posts cannot set this
		// content type, which layers CSRF protection on top of SameSite.
		if r.Method == http.MethodPost && !strings.HasPrefix(r.URL.Path, "/api/user/qr") {
			if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				writeErr(w, http.StatusUnsupportedMediaType, "unsupported media type")
				return
			}
		}
		if !p.authorized(r) {
			time.Sleep(time.Duration(600+time.Now().UnixNano()%500) * time.Millisecond)
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		fn(w, r)
	}
}

// page serves the admin console only to authenticated sessions; everyone else
// gets a bare login form and nothing else.
func (p *panelHandler) page(w http.ResponseWriter, r *http.Request) {
	secureHeaders(w)
	if !p.authorized(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(loginPageHTML))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(panelHTML))
}

// login validates the form-posted admin key and starts a server-side session.
// The key travels only in the POST body — never in any URL.
func (p *panelHandler) login(w http.ResponseWriter, r *http.Request) {
	secureHeaders(w)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/panel", http.StatusSeeOther)
		return
	}
	key := r.FormValue("key")
	if subtle.ConstantTimeCompare([]byte(key), []byte(p.key)) != 1 {
		time.Sleep(time.Duration(600+time.Now().UnixNano()%500) * time.Millisecond)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(loginPageHTML))
		return
	}
	tok, err := p.newSession()
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/panel", http.StatusSeeOther)
}

// logout invalidates the session immediately.
func (p *panelHandler) logout(w http.ResponseWriter, r *http.Request) {
	secureHeaders(w)
	if c, err := r.Cookie(sessionCookie); err == nil {
		p.sessions.Delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/panel", http.StatusSeeOther)
}

type userView struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	UUID      string `json:"uuid"`
	Created   int64  `json:"created"`
	Expires   int64  `json:"expires"` // 0 = never
	Quota     int64  `json:"quota"`   // 0 = unlimited
	Used      int64  `json:"used"`
	Up        int64  `json:"up"`
	Down      int64  `json:"down"`
	UpHuman   string `json:"up_human"`
	DownHuman string `json:"down_human"`
	SubToken  string `json:"sub_token"`
	SubURL    string `json:"sub_url"`
	VlessURI  string `json:"vless_uri"`
	UsedHuman string `json:"used_human"`
	QuotaHuma string `json:"quota_human"`
}

func (p *panelHandler) userViews(r *http.Request) ([]userView, error) {
	store, err := users.NewStore("")
	if err != nil {
		return nil, err
	}
	snap, _ := traffic.Query()

	// Behind Cloudflare Tunnel the request arrives as plain HTTP with
	// X-Forwarded-Proto=https; subscription links should reflect that.
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}

	list := store.List()
	views := make([]userView, 0, len(list))
	for _, u := range list {
		status := "active"
		if u.IsRevoked {
			status = "revoked"
		} else if !u.Enabled {
			status = "disabled"
		} else if u.ExpiresAt != nil && time.Now().After(*u.ExpiresAt) {
			status = "expired"
		}

		var used, up, down int64
		if snap != nil {
			if usage, ok := traffic.ForUser(snap, u.Name); ok {
				up, down = usage.Up, usage.Down
				used = usage.Total()
			}
		}

		var expires int64
		if u.ExpiresAt != nil {
			expires = u.ExpiresAt.Unix()
		}

		subURL := ""
		vless := ""
		if u.SubToken != "" {
			subURL = fmt.Sprintf("%s://%s/sub/%s", scheme, r.Host, u.SubToken)
			if ctx, err := exporter.LoadExportContext(u.Name); err == nil {
				vless = uri.GenerateURI(ctx)
			}
		}

		views = append(views, userView{
			Name:      u.Name,
			Status:    status,
			UUID:      u.UUID,
			Created:   u.CreatedAt.Unix(),
			Expires:   expires,
			Quota:     u.QuotaBytes,
			Used:      used,
			Up:        up,
			Down:      down,
			UpHuman:   traffic.Human(up),
			DownHuman: traffic.Human(down),
			SubToken:  u.SubToken,
			SubURL:    subURL,
			VlessURI:  vless,
			UsedHuman: traffic.Human(used),
			QuotaHuma: humanQuota(u.QuotaBytes),
		})
	}
	return views, nil
}

func humanQuota(b int64) string {
	if b <= 0 {
		return "∞"
	}
	return traffic.Human(b)
}

func (p *panelHandler) apiUsers(w http.ResponseWriter, r *http.Request) {
	views, err := p.userViews(r)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]any{"users": views, "now": time.Now().Unix(), "node_online": nodeOnline()})
}

// nodeOnline reports whether the proxy core service is running.
func nodeOnline() bool {
	return exec.Command("systemctl", "is-active", "--quiet", "vpnctl-proxy").Run() == nil
}

// apiLogs serves the most recent subscription access events.
func (p *panelHandler) apiLogs(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{"logs": sublog.Recent(100)})
}

type mutateReq struct {
	Name    string  `json:"name"`
	QuotaGB float64 `json:"quota_gb"` // 0 = unlimited
	Days    int     `json:"days"`     // 0 = never
	Enabled *bool   `json:"enabled"`
}

func decodeBody(r *http.Request) (*mutateReq, error) {
	var m mutateReq
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}
	if strings.TrimSpace(m.Name) == "" {
		return nil, fmt.Errorf("name required")
	}
	return &m, nil
}

func (p *panelHandler) apiAdd(w http.ResponseWriter, r *http.Request) {
	m, err := decodeBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	store, err := users.NewStore("")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var dur time.Duration
	if m.Days > 0 {
		dur = time.Duration(m.Days) * 24 * time.Hour
	}
	u, err := store.AddWithExpiry(m.Name, "", dur)
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if m.QuotaGB > 0 {
		if _, err := store.SetQuotaBytes(m.Name, int64(m.QuotaGB*1e9)); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := coresync.Sync(); err != nil {
		writeErr(w, http.StatusInternalServerError, "user stored, but core sync failed: "+err.Error())
		return
	}
	writeOK(w, map[string]any{"ok": true, "name": u.Name, "sub_token": u.SubToken})
}

func (p *panelHandler) apiRemove(w http.ResponseWriter, r *http.Request) {
	m, err := decodeBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	store, err := users.NewStore("")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := store.Remove(m.Name); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if err := coresync.Sync(); err != nil {
		writeErr(w, http.StatusInternalServerError, "removed, but core sync failed: "+err.Error())
		return
	}
	writeOK(w, map[string]any{"ok": true})
}

func (p *panelHandler) apiToggle(w http.ResponseWriter, r *http.Request) {
	m, err := decodeBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.Enabled == nil {
		writeErr(w, http.StatusBadRequest, "enabled (bool) required")
		return
	}
	store, err := users.NewStore("")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := store.SetEnabled(m.Name, *m.Enabled); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if err := coresync.Sync(); err != nil {
		writeErr(w, http.StatusInternalServerError, "core sync failed: "+err.Error())
		return
	}
	writeOK(w, map[string]any{"ok": true})
}

func (p *panelHandler) apiQuota(w http.ResponseWriter, r *http.Request) {
	m, err := decodeBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	store, err := users.NewStore("")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var quota int64
	if m.QuotaGB > 0 {
		quota = int64(m.QuotaGB * 1e9)
	}
	u, err := store.SetQuotaBytes(m.Name, quota)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeOK(w, map[string]any{"ok": true, "quota": u.QuotaBytes})
}

func (p *panelHandler) apiResetToken(w http.ResponseWriter, r *http.Request) {
	m, err := decodeBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	store, err := users.NewStore("")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	u, err := store.ResetSubToken(m.Name)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeOK(w, map[string]any{"ok": true, "sub_token": u.SubToken})
}

func (p *panelHandler) apiQR(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	ctx, err := exporter.LoadExportContext(name)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	link := uri.GenerateURI(ctx)
	art, err := qr.GenerateTerminal(link)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]any{"ok": true, "uri": link, "qr": art})
}

func writeOK(w http.ResponseWriter, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}
