package cli

import (
	"fmt"

	"github.com/veilshard/veilshard/internal/gitops"
)

func runInitGitOps(args []string) error {
	destDir := "."
	if len(args) > 0 && args[0] != "" {
		destDir = args[0]
	}

	ui := NewUI()
	ui.Header("vpnctl init-gitops - Zero-Trust CI/CD Automation Scaffold")

	if err := gitops.InitGitOpsScaffold(destDir); err != nil {
		ui.Error(fmt.Sprintf("Failed to initialize GitOps scaffold: %v", err))
		return err
	}

	ui.Success("GitOps automation scaffold successfully generated!")
	fmt.Println()
	ui.KeyValue("Workflow", ".github/workflows/vpnctl-sync.yml")
	ui.KeyValue("Declarative Fleet", "fleet.yaml")
	ui.KeyValue("Strict .gitignore", ".gitignore (Credentials never committed)")
	ui.KeyValue("Instructions", "GITOPS_GUIDE.md")

	fmt.Printf("\n%sNext Steps:%s\n", ColorCyan, ColorReset)
	fmt.Println("1. Push this directory to your private GitHub repository.")
	fmt.Println("2. Add GitHub Repository Secrets: FLEET_SSH_KEY, CF_WORKER_URL, CF_API_SECRET.")
	fmt.Println("3. Edit fleet.yaml and push to main branch to deploy with ZERO credentials on your laptop.")
	fmt.Println()

	return nil
}
