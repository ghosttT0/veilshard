package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/veilshard/veilshard/internal/nexus/client"
	"github.com/veilshard/veilshard/internal/nexus/server"
	"github.com/veilshard/veilshard/internal/protocol"
)

const NexusVersion = "v0.1.0-alpha"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Capture interrupt signals for graceful exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[Nexus] Received shutdown signal, closing...")
		cancel()
	}()

	switch cmd {
	case "server":
		runServer(ctx, args)
	case "client":
		runClient(ctx, args)
	case "version", "-v", "--version":
		fmt.Printf("Nexus Core %s - Next-Generation Adaptive Morphing Proxy\n", NexusVersion)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func runServer(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	listen := fs.String("listen", "0.0.0.0:8443", "Tunnel listen address")
	key := fs.String("key", "", "Authentication pre-shared secret key (required)")
	profile := fs.String("profile", "webrtc", "Traffic morphing profile: webrtc, https, uniform")
	_ = fs.Parse(args)

	if *key == "" {
		log.Fatal("Error: -key is required for server authentication")
	}

	srv := server.New(server.Config{
		ListenAddr: *listen,
		SecretKey:  *key,
		Profile:    protocol.ProfileType(*profile),
	})

	if err := srv.Start(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}

func runClient(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("client", flag.ExitOnError)
	socks := fs.String("socks", "127.0.0.1:1080", "Local SOCKS5 listen address")
	serverAddr := fs.String("server", "", "Remote Nexus server address (e.g. 1.2.3.4:8443) (required)")
	key := fs.String("key", "", "Authentication pre-shared secret key (required)")
	profile := fs.String("profile", "webrtc", "Traffic morphing profile: webrtc, https, uniform")
	_ = fs.Parse(args)

	if *serverAddr == "" || *key == "" {
		log.Fatal("Error: both -server and -key are required")
	}

	cli := client.New(client.Config{
		LocalAddr:  *socks,
		ServerAddr: *serverAddr,
		SecretKey:  *key,
		Profile:    protocol.ProfileType(*profile),
	})

	if err := cli.Start(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("Client exited with error: %v", err)
	}
}

func printUsage() {
	fmt.Printf("Nexus Core %s - Next-Generation Adaptive Morphing Proxy\n\n", NexusVersion)
	fmt.Printf("Usage:\n")
	fmt.Printf("  nexus server -listen <addr:port> -key <secret> [-profile webrtc|https|uniform]\n")
	fmt.Printf("  nexus client -socks <addr:port> -server <server_ip:port> -key <secret> [-profile webrtc|https|uniform]\n")
	fmt.Printf("  nexus version\n\n")
	fmt.Printf("Examples:\n")
	fmt.Printf("  # On VPS Server:\n")
	fmt.Printf("  nexus server -listen 0.0.0.0:8443 -key my-strong-secret-token\n\n")
	fmt.Printf("  # On Local Computer (Browser connects to 127.0.0.1:1080 SOCKS5):\n")
	fmt.Printf("  nexus client -socks 127.0.0.1:1080 -server 198.51.100.1:8443 -key my-strong-secret-token\n\n")
}
