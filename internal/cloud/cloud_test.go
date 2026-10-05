package cloud

import (
	"testing"
)

func TestNewProviderFactory(t *testing.T) {
	do, err := NewProvider("digitalocean", "mock-token")
	if err != nil || do.Name() != "digitalocean" {
		t.Errorf("expected digitalocean adapter, got %v (err: %v)", do, err)
	}

	hz, err := NewProvider("hetzner", "mock-token")
	if err != nil || hz.Name() != "hetzner" {
		t.Errorf("expected hetzner adapter, got %v (err: %v)", hz, err)
	}

	vl, err := NewProvider("vultr", "mock-token")
	if err != nil || vl.Name() != "vultr" {
		t.Errorf("expected vultr adapter, got %v (err: %v)", vl, err)
	}

	_, err = NewProvider("unknown-cloud", "mock-token")
	if err == nil {
		t.Errorf("expected error for unknown cloud provider")
	}
}
