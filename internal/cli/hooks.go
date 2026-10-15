package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/jorgenosberg/agentcfg/internal/agent"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/hookcfg"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

// registerHooks brings hook registrations on targets in line with the
// installed scripts' headers and prints what changed.
func registerHooks(w io.Writer, targets []config.Target, dryRun bool) error {
	var failed int
	codexWritten := false
	for _, r := range hookcfg.ReconcileAll(targets, dryRun) {
		if r.Err != nil {
			fmt.Fprintf(w, "error\t%s: %v\n", r.Target.Name, r.Err)
			failed++
			continue
		}
		for _, e := range r.Added {
			fmt.Fprintf(w, "register\t%s (%s) in %s\n", e.Script, e.Registration, r.File)
		}
		for _, e := range r.Updated {
			fmt.Fprintf(w, "update\t%s (%s) in %s\n", e.Script, e.Registration, r.File)
		}
		for _, e := range r.Removed {
			fmt.Fprintf(w, "unregister\t%s (%s) in %s\n", e.Script, e.Registration, r.File)
		}
		for _, e := range r.Foreign {
			fmt.Fprintf(w, "warn\t%s also runs %q (%s); remove it if agentcfg should own this hook\n", r.File, e.Command, e.Registration)
		}
		if r.Written && r.Target.Agent == agent.Codex {
			codexWritten = true
		}
	}
	if codexWritten {
		fmt.Fprintln(w, "note\tCodex skips changed hooks until you review them with /hooks")
	}
	if failed > 0 {
		return fmt.Errorf("hook registration failed on %d target(s)", failed)
	}
	return nil
}

// registerItemHooks runs registerHooks when it is a hook item.
func registerItemHooks(w io.Writer, it source.Item, targets []config.Target) error {
	if it.Kind != source.KindHook {
		return nil
	}
	return registerHooks(w, targets, false)
}

// writeHookStatus lists hook registrations that differ from the script
// headers. It prints nothing when everything matches.
func writeHookStatus(w io.Writer, targets []config.Target) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	header := false
	row := func(target, script, state, detail string) {
		if !header {
			fmt.Fprintln(tw, "\nHOOK REGISTRATION\tSCRIPT\tSTATE\tDETAIL")
			header = true
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", target, script, state, detail)
	}
	for _, r := range hookcfg.ReconcileAll(targets, true) {
		name := r.Target.Name
		if r.Err != nil {
			row(name, "", "error", r.Err.Error())
			continue
		}
		for _, e := range r.Added {
			row(name, e.Script, "unregistered", e.Registration.String())
		}
		for _, e := range r.Updated {
			row(name, e.Script, "outdated", e.Registration.String())
		}
		for _, e := range r.Removed {
			row(name, e.Script, "stale", e.Command)
		}
		for _, e := range r.Foreign {
			row(name, "", "duplicate", e.Command)
		}
	}
	if header {
		fmt.Fprintln(tw, "\n`agentcfg sync` fixes unregistered, outdated, and stale rows; remove duplicates by hand")
	}
	return tw.Flush()
}
