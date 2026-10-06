// Package panel serves the admin web UI and its JSON API on the same port as
// the subscription server. Every route requires the admin key (X-Admin-Key
// header, ?key= query parameter, or panel cookie).
package panel

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/coresync"
	"github.com/veilshard/veilshard/internal/exporter"
	"github.com/veilshard/veilshard/internal/exporter/qr"
	"github.com/veilshard/veilshard/internal/exporter/uri"
	"github.com/veilshard/veilshard/internal/traffic"
	"github.com/veilshard/veilshard/internal/users"
)

const adminCookie = "vpnctl_panel_key"

type panelHandler struct {
	key string
}

// Mount registers panel routes on the mux.
func Mount(mux *http.ServeMux, adminKey string) {
	if adminKey == "" {
		return
	}
	p := &panelHandler{key: adminKey}
	mux.HandleFunc("/panel", p.auth(p.page))
	mux.HandleFunc("/api/users", p.auth(p.apiUsers))
	mux.HandleFunc("/api/user/add", p.auth(p.apiAdd))
	mux.HandleFunc("/api/user/remove", p.auth(p.apiRemove))
	mux.HandleFunc("/api/user/toggle", p.auth(p.apiToggle))
	mux.HandleFunc("/api/user/quota", p.auth(p.apiQuota))
	mux.HandleFunc("/api/user/reset-token", p.auth(p.apiResetToken))
	mux.HandleFunc("/api/user/qr", p.auth(p.apiQR))
}

func (p *panelHandler) auth(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		given := r.Header.Get("X-Admin-Key")
		if given == "" {
			given = r.URL.Query().Get("key")
		}
		if given == "" {
			if c, err := r.Cookie(adminCookie); err == nil {
				given = c.Value
			}
		}
		if subtle.ConstantTimeCompare([]byte(given), []byte(p.key)) != 1 {
			time.Sleep(800 * time.Millisecond)
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeErr(w, http.StatusUnauthorized, "invalid admin key")
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(loginPageHTML))
			return
		}
		fn(w, r)
	}
}

func (p *panelHandler) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(panelHTML))
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
			subURL = fmt.Sprintf("http://%s/sub/%s", r.Host, u.SubToken)
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
	writeOK(w, map[string]any{"users": views, "now": time.Now().Unix()})
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
