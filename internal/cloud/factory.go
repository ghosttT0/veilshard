package cloud

import (
	"fmt"
	"strings"
)

// NewProvider instantiates the appropriate cloud provider adapter.
func NewProvider(providerName, token string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "hetzner", "hcloud":
		return NewHetzner(token), nil
	case "vultr":
		return NewVultr(token), nil
	case "digitalocean", "do", "":
		return NewDigitalOcean(token), nil
	default:
		return nil, fmt.Errorf("unsupported cloud provider '%s' (supported: hetzner, vultr, digitalocean)", providerName)
	}
}
