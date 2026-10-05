package exporter

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/credentials"
	"github.com/veilshard/veilshard/internal/state"
	"github.com/veilshard/veilshard/internal/users"
)

func autoDetectPublicIPv4() string {
	client := &http.Client{Timeout: 3 * time.Second}
	endpoints := []string{"https://api.ipify.org", "https://icanhazip.com", "https://ifconfig.me/ip", "https://ip.sb"}
	for _, ep := range endpoints {
		resp, err := client.Get(ep)
		if err == nil {
			data, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			ipStr := strings.TrimSpace(string(data))
			if ip := net.ParseIP(ipStr); ip != nil && ip.To4() != nil {
				return ip.String()
			}
		}
	}
	return ""
}

// ExportContext holds all metadata needed to produce client configs.
type ExportContext struct {
	ServerIP   string
	Port       int
	UUID       string
	UserName   string
	PublicKey  string
	ShortID    string
	ServerName string
	NodeName   string
}

// LoadExportContext loads credentials, users, and server details.
func LoadExportContext(username string) (*ExportContext, error) {
	cfg, err := config.Load("")
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	credsData, err := os.ReadFile("/etc/vpnctl/credentials.json")
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	var creds credentials.Credentials
	if err := json.Unmarshal(credsData, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	userStore, _ := users.NewStore("")
	var targetUser *users.User
	if userStore != nil {
		if username != "" {
			targetUser, err = userStore.Get(username)
			if err != nil {
				return nil, fmt.Errorf("user %s not found: %w", username, err)
			}
		} else {
			enabled := userStore.ListEnabled()
			if len(enabled) > 0 {
				targetUser = enabled[0]
			}
		}
	}

	userUUID := creds.UUID
	userName := "default"
	if targetUser != nil {
		userUUID = targetUser.UUID
		userName = targetUser.Name
	}

	serverIP := cfg.Server.PublicIP
	if serverIP == "" || serverIP == "YOUR_SERVER_IP" {
		if st, err := state.Load(""); err == nil && st.PublicIPv4 != "" {
			serverIP = st.PublicIPv4
		}
	}
	if serverIP == "" || serverIP == "YOUR_SERVER_IP" {
		serverIP = autoDetectPublicIPv4()
	}
	if serverIP == "" {
		serverIP = "127.0.0.1"
	}

	nodeName := fmt.Sprintf("vpnctl-%s", userName)

	sni := creds.ServerName
	if len(cfg.Protocol.ServerNames) > 0 {
		sni = cfg.Protocol.ServerNames[0]
	}

	return &ExportContext{
		ServerIP:   serverIP,
		Port:       cfg.Server.Port,
		UUID:       userUUID,
		UserName:   userName,
		PublicKey:  creds.PublicKey,
		ShortID:    creds.ShortID,
		ServerName: sni,
		NodeName:   nodeName,
	}, nil
}
