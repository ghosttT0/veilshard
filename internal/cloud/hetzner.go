package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HetznerAdapter implements Provider for Hetzner Cloud (hcloud).
type HetznerAdapter struct {
	apiToken string
	client   *http.Client
}

func NewHetzner(token string) *HetznerAdapter {
	return &HetznerAdapter{
		apiToken: token,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (hz *HetznerAdapter) Name() string {
	return "hetzner"
}

func (hz *HetznerAdapter) ListInstances(ctx context.Context) ([]Instance, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.hetzner.cloud/v1/servers", nil)
	req.Header.Set("Authorization", "Bearer "+hz.apiToken)

	resp, err := hz.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hetzner api status %d: %s", resp.StatusCode, string(body))
	}

	var data struct {
		Servers []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			PublicNet struct {
				IPv4 struct {
					IP string `json:"ip"`
				} `json:"ipv4"`
			} `json:"public_net"`
			Datacenter struct {
				Location struct {
					Name string `json:"name"`
				} `json:"location"`
			} `json:"datacenter"`
		} `json:"servers"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var list []Instance
	for _, s := range data.Servers {
		list = append(list, Instance{
			ID:     fmt.Sprintf("%d", s.ID),
			Name:   s.Name,
			IPv4:   s.PublicNet.IPv4.IP,
			Region: s.Datacenter.Location.Name,
			Status: s.Status,
		})
	}

	return list, nil
}

func (hz *HetznerAdapter) CreateInstance(ctx context.Context, name, region string, sshKeyIDs []string) (*Instance, error) {
	if region == "" {
		region = "fsn1" // Falkenstein (Germany)
	}

	payload := map[string]interface{}{
		"name":        name,
		"server_type": "cpx11", // 2 vCPU, 2GB RAM (~€3.85/mo)
		"image":       "ubuntu-24.04",
		"location":    region,
		"ssh_keys":    sshKeyIDs,
		"start_after_create": true,
	}

	buf, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.hetzner.cloud/v1/servers", bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+hz.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := hz.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create hetzner server failed (%d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		Server struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			PublicNet struct {
				IPv4 struct {
					IP string `json:"ip"`
				} `json:"ipv4"`
			} `json:"public_net"`
		} `json:"server"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	idStr := fmt.Sprintf("%d", res.Server.ID)
	// Poll until server is running and IPv4 assigned (typically 10-25s on Hetzner)
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}

		instances, err := hz.ListInstances(ctx)
		if err == nil {
			for _, inst := range instances {
				if inst.ID == idStr && inst.IPv4 != "" && inst.Status == "running" {
					return &inst, nil
				}
			}
		}
	}

	return &Instance{
		ID:     idStr,
		Name:   name,
		IPv4:   res.Server.PublicNet.IPv4.IP,
		Region: region,
		Status: "running",
	}, nil
}

func (hz *HetznerAdapter) DeleteInstance(ctx context.Context, instanceID string) error {
	req, _ := http.NewRequestWithContext(ctx, "DELETE", "https://api.hetzner.cloud/v1/servers/"+instanceID, nil)
	req.Header.Set("Authorization", "Bearer "+hz.apiToken)

	resp, err := hz.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func (hz *HetznerAdapter) RecycleInstance(ctx context.Context, instanceID, name, region string, sshKeyIDs []string) (*Instance, error) {
	if instanceID != "" {
		_ = hz.DeleteInstance(ctx, instanceID)
	}
	return hz.CreateInstance(ctx, name, region, sshKeyIDs)
}
