package main

import (
	"fmt"
	"os"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/tui"
	"github.com/jorgenosberg/agentcfg/internal/version"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("lazyagentcfg", version.String())
		return
	}

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
	if err := tui.Run(path, cfg); err != nil {
		die(err)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "lazyagentcfg:", err)
	os.Exit(1)
}
