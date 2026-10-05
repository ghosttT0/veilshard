package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Instance represents a cloud virtual server.
type Instance struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IPv4      string    `json:"ipv4"`
	Region    string    `json:"region"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Provider defines the interface for ephemeral cloud lifecycle automation.
type Provider interface {
	Name() string
	ListInstances(ctx context.Context) ([]Instance, error)
	CreateInstance(ctx context.Context, name, region string, sshKeyIDs []string) (*Instance, error)
	DeleteInstance(ctx context.Context, instanceID string) error
	RecycleInstance(ctx context.Context, instanceID, name, region string, sshKeyIDs []string) (*Instance, error)
}

// DigitalOceanAdapter implements Provider for DigitalOcean droplets.
type DigitalOceanAdapter struct {
	apiToken string
	client   *http.Client
}

func NewDigitalOcean(token string) *DigitalOceanAdapter {
	return &DigitalOceanAdapter{
		apiToken: token,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (do *DigitalOceanAdapter) Name() string {
	return "digitalocean"
}

func (do *DigitalOceanAdapter) ListInstances(ctx context.Context) ([]Instance, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.digitalocean.com/v2/droplets", nil)
	req.Header.Set("Authorization", "Bearer "+do.apiToken)

	resp, err := do.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("do api status %d: %s", resp.StatusCode, string(body))
	}

	var data struct {
		Droplets []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Status   string `json:"status"`
			Networks struct {
				V4 []struct {
					IPAddress string `json:"ip_address"`
					Type      string `json:"type"`
				} `json:"v4"`
			} `json:"networks"`
			Region struct {
				Slug string `json:"slug"`
			} `json:"region"`
		} `json:"droplets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var list []Instance
	for _, d := range data.Droplets {
		ip := ""
		for _, v4 := range d.Networks.V4 {
			if v4.Type == "public" {
				ip = v4.IPAddress
				break
			}
		}
		list = append(list, Instance{
			ID:     fmt.Sprintf("%d", d.ID),
			Name:   d.Name,
			IPv4:   ip,
			Region: d.Region.Slug,
			Status: d.Status,
		})
	}

	return list, nil
}

func (do *DigitalOceanAdapter) CreateInstance(ctx context.Context, name, region string, sshKeyIDs []string) (*Instance, error) {
	payload := map[string]interface{}{
		"name":               name,
		"region":             region,
		"size":               "s-1vcpu-1gb",
		"image":              "ubuntu-24-04-x64",
		"ssh_keys":           sshKeyIDs,
		"backups":            false,
		"ipv6":               false,
		"monitoring":         true,
	}

	buf, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.digitalocean.com/v2/droplets", bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+do.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := do.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create droplet failed: %s", string(body))
	}

	var res struct {
		Droplet struct {
			ID int `json:"id"`
		} `json:"droplet"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	// Poll until droplet is active and has public IPv4 (typically 25-45s)
	idStr := fmt.Sprintf("%d", res.Droplet.ID)
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}

		instances, err := do.ListInstances(ctx)
		if err == nil {
			for _, inst := range instances {
				if inst.ID == idStr && inst.IPv4 != "" && inst.Status == "active" {
					return &inst, nil
				}
			}
		}
	}

	return &Instance{ID: idStr, Name: name, Region: region}, nil
}

func (do *DigitalOceanAdapter) DeleteInstance(ctx context.Context, instanceID string) error {
	req, _ := http.NewRequestWithContext(ctx, "DELETE", "https://api.digitalocean.com/v2/droplets/"+instanceID, nil)
	req.Header.Set("Authorization", "Bearer "+do.apiToken)

	resp, err := do.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func (do *DigitalOceanAdapter) RecycleInstance(ctx context.Context, instanceID, name, region string, sshKeyIDs []string) (*Instance, error) {
	// 1. Destroy blocked instance
	if instanceID != "" {
		_ = do.DeleteInstance(ctx, instanceID)
	}

	// 2. Provision fresh instance in same region
	return do.CreateInstance(ctx, name, region, sshKeyIDs)
}

// RecycleWorkflow coordinates the full destruction, replacement, and re-provisioning of a node.
func RecycleWorkflow(ctx context.Context, p Provider, nodeName, region string, oldInstanceID string, sshKeyIDs []string) (*Instance, error) {
	if region == "" {
		region = "sfo3" // Default US West region
	}
	return p.RecycleInstance(ctx, oldInstanceID, nodeName, region, sshKeyIDs)
}
