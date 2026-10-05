package exporter

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/credentials"
	"github.com/veilshard/veilshard/internal/users"
)

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
	if serverIP == "" {
		serverIP = "YOUR_SERVER_IP"
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
