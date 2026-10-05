package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/cloud"
	"github.com/veilshard/veilshard/internal/fleet"
)

func runRecycle(args []string) error {
	fs := flag.NewFlagSet("recycle", flag.ContinueOnError)
	providerName := fs.String("provider", "digitalocean", "Cloud provider: hetzner, vultr, digitalocean")
	token := fs.String("token", "", "Cloud provider API token (or set CLOUD_API_TOKEN / DIGITALOCEAN_TOKEN / HETZNER_TOKEN / VULTR_TOKEN)")
	region := fs.String("region", "", "Datacenter region slug (e.g. sfo3, fsn1, nrt)")
	sshKeyID := fs.String("key-id", "", "SSH Key ID registered in your cloud provider")
	fleetFile := fs.String("f", "veilshard.yaml", "Fleet configuration file to update with the new IP")

	if err := fs.Parse(args); err != nil {
		return err
	}

	nodeName := ""
	if fs.NArg() > 0 {
		nodeName = fs.Arg(0)
	}

	ui := NewUI()
	ui.Header(fmt.Sprintf("vpnctl recycle - Ephemeral Cloud IP Replacement (%s)", *providerName))

	if nodeName == "" {
		ui.Error("Node name is required. Usage: vpnctl recycle <node-name> [flags]")
		return fmt.Errorf("missing node name")
	}

	apiToken := *token
	if apiToken == "" {
		switch strings.ToLower(*providerName) {
		case "hetzner", "hcloud":
			apiToken = os.Getenv("HETZNER_TOKEN")
		case "vultr":
			apiToken = os.Getenv("VULTR_TOKEN")
		default:
			apiToken = os.Getenv("DIGITALOCEAN_TOKEN")
		}
		if apiToken == "" {
			apiToken = os.Getenv("CLOUD_API_TOKEN")
		}
	}

	if apiToken == "" {
		ui.Error(fmt.Sprintf("Cloud API token not provided for %s. Pass -token or set environment variable.", *providerName))
		return fmt.Errorf("missing cloud API token")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	adapter, err := cloud.NewProvider(*providerName, apiToken)
	if err != nil {
		ui.Error(err.Error())
		return err
	}

	targetRegion := *region
	if targetRegion == "" {
		switch strings.ToLower(*providerName) {
		case "hetzner":
			targetRegion = "fsn1"
		case "vultr":
			targetRegion = "nrt"
		default:
			targetRegion = "sfo3"
		}
	}

	ui.Section(fmt.Sprintf("Locating existing %s instance for node '%s'...", *providerName, nodeName))
	instances, err := adapter.ListInstances(ctx)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to list cloud instances: %v", err))
		return err
	}

	var targetID string
	var oldIP string
	for _, inst := range instances {
		if inst.Name == nodeName {
			targetID = inst.ID
			oldIP = inst.IPv4
			break
		}
	}

	if targetID != "" {
		fmt.Printf("• Found existing instance (ID: %s, Current IP: %s)\n", targetID, oldIP)
		fmt.Printf("• Destroying blocked instance...\n")
	} else {
		fmt.Printf("• No existing instance found named '%s', creating brand new instance...\n", nodeName)
	}

	var sshKeys []string
	if *sshKeyID != "" {
		sshKeys = append(sshKeys, *sshKeyID)
	}

	ui.Section(fmt.Sprintf("Provisioning fresh instance in region '%s'...", targetRegion))
	inst, err := adapter.RecycleInstance(ctx, targetID, nodeName, targetRegion, sshKeys)
	if err != nil {
		ui.Error(fmt.Sprintf("Recycle failed: %v", err))
		return err
	}

	ui.Success("Fresh cloud instance provisioned!")
	fmt.Println()
	ui.KeyValue("Node Name", inst.Name)
	ui.KeyValue("New Instance ID", inst.ID)
	ui.KeyValue("New IPv4 Address", inst.IPv4)
	ui.KeyValue("Region", inst.Region)
	ui.KeyValue("Status", inst.Status)

	// If fleet configuration exists, offer or update the host IP
	if _, err := os.Stat(*fleetFile); err == nil && inst.IPv4 != "" {
		cfg, err := fleet.LoadFleetConfig(*fleetFile)
		if err == nil {
			updated := false
			for i := range cfg.Nodes {
				if cfg.Nodes[i].Name == nodeName {
					cfg.Nodes[i].Host = inst.IPv4
					updated = true
					break
				}
			}
			if updated {
				_ = fleet.SaveFleetConfig(*fleetFile, cfg)
				fmt.Printf("\n%sUpdated %s with new IP: %s%s\n", ColorGreen, *fleetFile, inst.IPv4, ColorReset)
				fmt.Printf("Run %svpnctl apply -f %s%s to bootstrap the new server instantly.\n\n", ColorCyan, *fleetFile, ColorReset)
			}
		}
	}

	return nil
}
