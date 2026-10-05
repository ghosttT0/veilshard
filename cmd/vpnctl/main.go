package main

import (
	"os"

	"github.com/veilshard/veilshard/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

