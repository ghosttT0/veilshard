package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/veilshard/veilshard/internal/publish"
)

func runPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	file := fs.String("f", "dist/clash.yaml", "Path to client configuration profile to encrypt and distribute")
	workerURL := fs.String("worker", "", "Cloudflare Worker endpoint (e.g. https://sub.myworker.workers.dev)")
	secret := fs.String("secret", "", "Worker Authorization bearer token / API secret")
	scriptOnly := fs.Bool("script", false, "Generate and save worker.js for Cloudflare Worker deployment")
	outScript := fs.String("out-script", "dist/worker.js", "Destination path for generated worker script")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ui := NewUI()
	ui.Header("vpnctl publish - Zero-Knowledge Subscription Gateway")

	if *scriptOnly {
		if err := publish.SaveWorkerScript(*outScript); err != nil {
			ui.Error(fmt.Sprintf("Failed to save worker script: %v", err))
			return err
		}
		ui.Success(fmt.Sprintf("Cloudflare Worker script generated at: %s", *outScript))
		fmt.Printf("Deploy this script to Cloudflare Workers with a KV namespace named 'SUB_KV'.\n\n")
		return nil
	}

	// Read client configuration
	var content []byte
	var err error
	if _, errStat := os.Stat(*file); errStat == nil {
		content, err = os.ReadFile(*file)
		if err != nil {
			ui.Error(fmt.Sprintf("Read file error: %v", err))
			return err
		}
	} else {
		// Try fallback to local single node export if file not found
		ui.Warn(fmt.Sprintf("File %s not found. Looking for existing fleet export or local node...", *file))
		if _, errAlt := os.Stat("clash.yaml"); errAlt == nil {
			content, err = os.ReadFile("clash.yaml")
			if err != nil {
				return err
			}
		} else {
			ui.Error(fmt.Sprintf("Configuration file not found: %s", *file))
			fmt.Printf("Tip: Run %svpnctl apply%s or specify %s-f <path>%s to select a profile.\n", ColorCyan, ColorReset, ColorCyan, ColorReset)
			fmt.Printf("Or generate the Cloudflare Worker script with: %svpnctl publish --script%s\n\n", ColorBold, ColorReset)
			return fmt.Errorf("file not found: %s", *file)
		}
	}

	// Perform AES-256-GCM zero-knowledge encryption
	targetWorker := *workerURL
	if targetWorker == "" {
		targetWorker = "https://sub.your-domain.workers.dev"
	}

	pkg, err := publish.EncryptSubscription(content, targetWorker)
	if err != nil {
		ui.Error(fmt.Sprintf("Encryption failed: %v", err))
		return err
	}

	// If remote worker provided, upload ciphertext
	uploaded := false
	if *workerURL != "" && *secret != "" {
		ctx := context.Background()
		if err := publish.UploadToWorker(ctx, *workerURL, *secret, pkg); err != nil {
			ui.Warn(fmt.Sprintf("Failed to upload to worker: %v (saving local package instead)", err))
		} else {
			uploaded = true
		}
	}

	ui.Success("Zero-Knowledge Subscription Package Generated!")
	fmt.Println()
	ui.KeyValue("Encryption", "AES-256-GCM (Authenticated Encryption)")
	ui.KeyValue("Subscription ID", pkg.SubscriptionID)
	ui.KeyValue("Secret Key (256-bit)", pkg.SecretKeyHex)
	if uploaded {
		ui.KeyValue("Cloudflare Worker Upload", "SUCCESS")
	} else {
		ui.KeyValue("Cloudflare Worker Upload", "SKIPPED (Use -worker and -secret to auto-push)")
	}

	fmt.Printf("\n%sZero-Knowledge Subscription URL:%s\n", ColorGreen, ColorReset)
	fmt.Printf("%s%s%s\n\n", ColorCyan, pkg.ZeroKnowledgeURL, ColorReset)

	// Save ciphertext locally as backup
	cipherPath := filepath.Join("dist", fmt.Sprintf("sub_%s.enc", pkg.SubscriptionID))
	_ = os.MkdirAll("dist", 0755)
	_ = os.WriteFile(cipherPath, []byte(pkg.CiphertextB64), 0644)
	ui.KeyValue("Local Encrypted Backup", cipherPath)

	fmt.Printf("\n%sSecurity Rationale:%s\n", ColorBold, ColorReset)
	fmt.Println("• Server-side sees only high-entropy random ciphertext.")
	fmt.Println("• The AES key is appended in the URL Hash Fragment (#...).")
	fmt.Println("• Per RFC 3986, fragments are NEVER transmitted in HTTP request headers.")
	fmt.Println("• Web crawlers and CDN edge nodes cannot inspect or harvest node IPs.")
	fmt.Println()

	return nil
}
