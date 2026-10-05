package cli

import (
	"fmt"
	"os"

	"github.com/veilshard/veilshard/internal/config"
)

func runConfig(args []string) error {
	action := "show"
	if len(args) > 0 {
		action = args[0]
	}

	switch action {
	case "path":
		fmt.Println(config.DefaultConfigPath)
		return nil

	case "show", "cat", "view":
		data, err := os.ReadFile(config.DefaultConfigPath)
		if err != nil {
			return fmt.Errorf("read config %s: %w", config.DefaultConfigPath, err)
		}
		fmt.Print(string(data))
		return nil

	default:
		fmt.Printf("Usage: vpnctl config [show|path]\n")
		return nil
	}
}
