package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitGitOpsScaffold(t *testing.T) {
	tempDir := t.TempDir()

	if err := InitGitOpsScaffold(tempDir); err != nil {
		t.Fatalf("InitGitOpsScaffold failed: %v", err)
	}

	workflowFile := filepath.Join(tempDir, ".github", "workflows", "vpnctl-sync.yml")
	if _, err := os.Stat(workflowFile); err != nil {
		t.Errorf("missing workflow file: %v", err)
	}
	content, _ := os.ReadFile(workflowFile)
	if !strings.Contains(string(content), "vpnctl apply -f fleet.yaml") {
		t.Errorf("workflow missing apply command")
	}

	fleetFile := filepath.Join(tempDir, "fleet.yaml")
	if _, err := os.Stat(fleetFile); err != nil {
		t.Errorf("missing fleet.yaml file: %v", err)
	}

	gitignoreFile := filepath.Join(tempDir, ".gitignore")
	if _, err := os.Stat(gitignoreFile); err != nil {
		t.Errorf("missing .gitignore file: %v", err)
	}
	gitIgnoreContent, _ := os.ReadFile(gitignoreFile)
	if !strings.Contains(string(gitIgnoreContent), "*.key") || !strings.Contains(string(gitIgnoreContent), "id_ed25519*") {
		t.Errorf(".gitignore missing key safety patterns")
	}

	readmeFile := filepath.Join(tempDir, "GITOPS_GUIDE.md")
	if _, err := os.Stat(readmeFile); err != nil {
		t.Errorf("missing GITOPS_GUIDE.md: %v", err)
	}
}
