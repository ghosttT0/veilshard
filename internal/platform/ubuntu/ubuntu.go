package ubuntu

import (
	"fmt"
	"strings"

	"github.com/veilshard/veilshard/internal/platform"
)

// SupportedVersions lists tested and supported Ubuntu versions for v0.1.
var SupportedVersions = []string{"24.04"}

// ValidateSupport checks if the current platform matches v0.1 prerequisites.
func ValidateSupport(info *platform.Info) error {
	if info.OS != "linux" {
		return fmt.Errorf("vpnctl requires Linux, detected OS: %s", info.OS)
	}

	if info.Arch != "amd64" {
		return fmt.Errorf("v0.1 requires amd64 architecture, detected: %s", info.Arch)
	}

	if !strings.EqualFold(info.Distro, "ubuntu") {
		return fmt.Errorf("v0.1 officially supports Ubuntu, detected distribution: %s", info.Distro)
	}

	matched := false
	for _, sv := range SupportedVersions {
		if strings.HasPrefix(info.VersionID, sv) {
			matched = true
			break
		}
	}
	if !matched {
		return fmt.Errorf("v0.1 officially supports Ubuntu 24.04, detected version: %s (version_id: %s)", info.Version, info.VersionID)
	}

	return nil
}
