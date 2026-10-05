package users

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type RevokeRequest struct {
	Username string `json:"username"`
}

type RevokeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	User    string `json:"user,omitempty"`
}

// StartRevocationWebhookServer runs an authenticated HTTP listener for remote token revocation.
func StartRevocationWebhookServer(addr string, secret string, onRevoked func(username string) error) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/user/revoke", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(RevokeResponse{Success: false, Message: "Method not allowed"})
			return
		}

		// Authenticate bearer token
		auth := r.Header.Get("Authorization")
		if secret != "" && auth != "Bearer "+secret {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(RevokeResponse{Success: false, Message: "Unauthorized"})
			return
		}

		var req RevokeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(RevokeResponse{Success: false, Message: "Invalid JSON or missing username"})
			return
		}

		store, err := NewStore("")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(RevokeResponse{Success: false, Message: err.Error()})
			return
		}

		u, err := store.Revoke(req.Username)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(RevokeResponse{Success: false, Message: fmt.Sprintf("User '%s' not found: %v", req.Username, err)})
			return
		}

		if onRevoked != nil {
			_ = onRevoked(u.Name)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(RevokeResponse{
			Success: true,
			Message: fmt.Sprintf("User '%s' credentials revoked immediately", u.Name),
			User:    u.Name,
		})
	})

	// Telegram webhook endpoint
	mux.HandleFunc("/api/telegram", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var tgPayload struct {
			Message struct {
				Text string `json:"text"`
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&tgPayload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		text := strings.TrimSpace(tgPayload.Message.Text)
		if strings.HasPrefix(text, "/revoke ") {
			targetUser := strings.TrimSpace(strings.TrimPrefix(text, "/revoke "))
			store, _ := NewStore("")
			if store != nil {
				_, _ = store.Revoke(targetUser)
				if onRevoked != nil {
					_ = onRevoked(targetUser)
				}
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	})

	return http.ListenAndServe(addr, mux)
}
