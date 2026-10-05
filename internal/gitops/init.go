package gitops

import (
	"fmt"
	"os"
	"path/filepath"
)

// InitGitOpsScaffold creates a complete headless CI/CD GitOps workflow directory.
func InitGitOpsScaffold(destDir string) error {
	if destDir == "" {
		destDir = "."
	}

	workflowDir := filepath.Join(destDir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		return fmt.Errorf("mkdir workflow: %w", err)
	}

	// 1. Write GitHub Actions workflow
	workflowPath := filepath.Join(workflowDir, "vpnctl-sync.yml")
	workflowContent := `name: vpnctl GitOps Fleet Sync

on:
  push:
    branches: [ "main" ]
  workflow_dispatch:

jobs:
  orchestrate:
    name: Headless Fleet Apply & Publish
    runs-on: ubuntu-24.04
    steps:
      - name: Checkout Repository
        uses: actions/checkout@v4

      - name: Setup Go Runtime
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install veilshard
        run: |
          go install github.com/veilshard/veilshard/cmd/veilshard@latest || \
          (git clone https://github.com/veilshard/veilshard.git /tmp/veilshard && cd /tmp/veilshard && go build -o /usr/local/bin/veilshard ./cmd/veilshard)

      - name: Setup Headless SSH Key
        env:
          SSH_PRIVATE_KEY: ${{ secrets.FLEET_SSH_KEY }}
        run: |
          mkdir -p ~/.ssh
          echo "$SSH_PRIVATE_KEY" > ~/.ssh/id_ed25519
          chmod 600 ~/.ssh/id_ed25519
          ssh-keyscan -H -f <(grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' fleet.yaml) >> ~/.ssh/known_hosts 2>/dev/null || true

      - name: Orchestrate Fleet (veilshard apply)
        run: |
          veilshard apply -f fleet.yaml

      - name: Zero-Knowledge Publish to Cloudflare Worker
        env:
          CF_WORKER_URL: ${{ secrets.CF_WORKER_URL }}
          CF_API_SECRET: ${{ secrets.CF_API_SECRET }}
        run: |
          if [ -n "$CF_WORKER_URL" ]; then
            vpnctl publish -f dist/clash.yaml -worker "$CF_WORKER_URL" -secret "$CF_API_SECRET"
          else
            echo "Skipping worker publish: CF_WORKER_URL not configured."
          fi
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		return err
	}

	// 2. Write declarative fleet.yaml template
	fleetPath := filepath.Join(destDir, "fleet.yaml")
	fleetContent := `version: "1"

# Declarative Multi-Node Mesh Specification
nodes:
  # Entry Relay Node (e.g. Hong Kong BGP / Low Latency Transit)
  - name: "hk-transit"
    role: relay
    host: "103.200.1.5"
    listen_port: 8443
    target: "us-exit"
    ssh: { user: "root", key: "~/.ssh/id_ed25519", port: 22 }

  # High-Bandwidth Exit Node (e.g. US / EU REALITY Core)
  - name: "us-exit"
    role: exit
    host: "198.51.100.2"
    port: 443
    protocol: vless-reality
    sni_target: "auto"
    monthly_quota: "1000GB"
    ssh: { user: "root", key: "~/.ssh/id_ed25519", port: 22 }

output:
  clash_meta: "./dist/clash.yaml"
  sing_box: "./dist/sing-box.json"
`
	if err := os.WriteFile(fleetPath, []byte(fleetContent), 0644); err != nil {
		return err
	}

	// 3. Write strictly safe .gitignore
	gitignorePath := filepath.Join(destDir, ".gitignore")
	gitignoreContent := `# Security: Never commit SSH private keys or local state
*.pem
*.key
id_rsa*
id_ed25519*
credentials.json
state.json
dist/*.enc
dist/*.yaml
dist/*.json
`
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return err
	}

	// 4. Write GitOps README guide
	readmePath := filepath.Join(destDir, "GITOPS_GUIDE.md")
	readmeContent := `# vpnctl Zero-Trust GitOps Fleet Management

By delegating deployment to a private GitHub Actions workflow, you completely eliminate the security risk of storing production VPS root private keys on your local laptop.

## Setup Instructions

1. **Create a Private GitHub Repository**:
   Push this scaffold to your private repo. **Strictly keep it private!**

2. **Configure Repository Secrets**:
   Go to your GitHub repo -> **Settings** -> **Secrets and variables** -> **Actions** -> **New repository secret**:
   - ` + "`" + `FLEET_SSH_KEY` + "`" + `: Your SSH private key authorized on all fleet VPS instances.
   - ` + "`" + `CF_WORKER_URL` + "`" + `: Your Cloudflare Worker endpoint (e.g. ` + "`" + `https://sub.myworker.workers.dev` + "`" + `).
   - ` + "`" + `CF_API_SECRET` + "`" + `: The authorization bearer token configured in your Worker.

3. **Deploy Nodes Declaratively**:
   Whenever you add or rotate a node, simply edit ` + "`" + `fleet.yaml` + "`" + ` and commit:
   ` + "```bash" + `
   git add fleet.yaml
   git commit -m "feat: add tokyo-relay to mesh"
   git push origin main
   ` + "```" + `
   The GitHub Actions runner executes ` + "`" + `vpnctl apply` + "`" + ` in a secure headless sandbox and automatically publishes zero-knowledge encrypted subscriptions to your Worker.
`
	return os.WriteFile(readmePath, []byte(readmeContent), 0644)
}
