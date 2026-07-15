package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jorgenosberg/agentcfg/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		if !errors.Is(err, cli.ErrSilent) {
			fmt.Fprintln(os.Stderr, "agentcfg:", err)
		}
		os.Exit(1)
	}
}
