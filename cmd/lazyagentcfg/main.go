package main

import (
	"fmt"
	"os"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/tui"
)

func main() {
	path, err := config.DefaultPath()
	if err != nil {
		die(err)
	}
	if env := os.Getenv("AGENTCFG_CONFIG"); env != "" {
		path = env
	}
	cfg, err := config.Load(path)
	if err != nil {
		die(err)
	}
	if err := tui.Run(cfg); err != nil {
		die(err)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "lazyagentcfg:", err)
	os.Exit(1)
}
