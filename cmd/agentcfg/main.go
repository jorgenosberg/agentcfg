package main

import (
	"fmt"
	"os"

	"github.com/jorgenosberg/agentcfg/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "agentcfg:", err)
		os.Exit(1)
	}
}
