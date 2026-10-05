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

// VultrAdapter implements Provider for Vultr Cloud Compute instances.
type VultrAdapter struct {
	apiToken string
	client   *http.Client
}

func NewVultr(token string) *VultrAdapter {
	return &VultrAdapter{
		apiToken: token,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (v *VultrAdapter) Name() string {
	return "vultr"
}

func (v *VultrAdapter) ListInstances(ctx context.Context) ([]Instance, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.vultr.com/v2/instances", nil)
	req.Header.Set("Authorization", "Bearer "+v.apiToken)

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vultr api status %d: %s", resp.StatusCode, string(body))
	}

	var data struct {
		Instances []struct {
			ID       string `json:"id"`
			Label    string `json:"label"`
			MainIP   string `json:"main_ip"`
			Region   string `json:"region"`
			Status   string `json:"status"`
			PowerSt  string `json:"power_status"`
		} `json:"instances"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var list []Instance
	for _, inst := range data.Instances {
		list = append(list, Instance{
			ID:     inst.ID,
			Name:   inst.Label,
			IPv4:   inst.MainIP,
			Region: inst.Region,
			Status: inst.Status,
		})
	}

	return list, nil
}

func (v *VultrAdapter) CreateInstance(ctx context.Context, name, region string, sshKeyIDs []string) (*Instance, error) {
	if region == "" {
		region = "nrt" // Tokyo, Japan
	}

	payload := map[string]interface{}{
		"region":   region,
		"plan":     "vc2-1c-1gb", // 1 vCPU, 1GB RAM ($5/mo)
		"os_id":    2284,         // Ubuntu 24.04 x64
		"label":    name,
		"sshkey_id": sshKeyIDs,
		"backups":  "disabled",
	}

	buf, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.vultr.com/v2/instances", bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+v.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create vultr instance failed (%d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		Instance struct {
			ID string `json:"id"`
		} `json:"instance"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	// Poll until instance has public IP allocated
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}

		instances, err := v.ListInstances(ctx)
		if err == nil {
			for _, inst := range instances {
				if inst.ID == res.Instance.ID && inst.IPv4 != "" && inst.IPv4 != "0.0.0.0" {
					return &inst, nil
				}
			}
		}
	}

	return &Instance{
		ID:     res.Instance.ID,
		Name:   name,
		Region: region,
		Status: "pending",
	}, nil
}

func (v *VultrAdapter) DeleteInstance(ctx context.Context, instanceID string) error {
	req, _ := http.NewRequestWithContext(ctx, "DELETE", "https://api.vultr.com/v2/instances/"+instanceID, nil)
	req.Header.Set("Authorization", "Bearer "+v.apiToken)

	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func (v *VultrAdapter) RecycleInstance(ctx context.Context, instanceID, name, region string, sshKeyIDs []string) (*Instance, error) {
	if instanceID != "" {
		_ = v.DeleteInstance(ctx, instanceID)
	}
	return v.CreateInstance(ctx, name, region, sshKeyIDs)
}
