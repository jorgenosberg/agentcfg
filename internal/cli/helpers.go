package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
)

func writeItems(w io.Writer, items []source.Item, sourcePath string) error {
	if len(items) == 0 {
		fmt.Fprintf(w, "no items in source (%s); add files there or run `agentcfg import <agent> --all`\n", sourcePath)
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KIND\tNAME\tPATH")
	for _, it := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", it.Kind, it.Name, it.Path)
	}
	return tw.Flush()
}

func writeStatus(w io.Writer, entries []sync.Entry) error {
	if len(entries) == 0 {
		fmt.Fprintln(w, "nothing to show; add targets with `agentcfg discover --add <name>` and items under your source")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TARGET\tKIND\tITEM\tSTATUS\tDEST")
	for _, e := range entries {
		label := string(e.Status)
		if e.Status == sync.StatusDisabled {
			label = "off"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			e.Target.Name, e.Item.Kind, e.Item.Name, label, e.Dest)
	}
	return tw.Flush()
}

func writeItemsJSON(w io.Writer, items []source.Item) error {
	type itemJSON struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	out := make([]itemJSON, 0, len(items))
	for _, it := range items {
		out = append(out, itemJSON{Kind: it.Kind, Name: it.Name, Path: it.Path})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func writeStatusJSON(w io.Writer, entries []sync.Entry) error {
	type entryJSON struct {
		Target string `json:"target"`
		Kind   string `json:"kind"`
		Item   string `json:"item"`
		Status string `json:"status"`
		Dest   string `json:"dest"`
		Plugin string `json:"plugin,omitempty"`
	}
	out := make([]entryJSON, 0, len(entries))
	for _, e := range entries {
		ej := entryJSON{
			Target: e.Target.Name,
			Kind:   e.Item.Kind,
			Item:   e.Item.Name,
			Status: string(e.Status),
			Dest:   e.Dest,
		}
		if e.Plugin != nil {
			ej.Plugin = e.Plugin.FullName
		}
		out = append(out, ej)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func findItem(items []source.Item, name string) (source.Item, bool) {
	for _, it := range items {
		if it.Name == name {
			return it, true
		}
	}
	return source.Item{}, false
}

func selectTargets(all []config.Target, name string) []config.Target {
	if name == "" {
		return all
	}
	var out []config.Target
	for _, t := range all {
		if t.Name == name || (t.Alias != "" && t.Alias == name) {
			out = append(out, t)
		}
	}
	return out
}

// resolveTargets selects targets by name (or all, if name is empty) and
// returns a descriptive error when nothing matches, instead of leaving
// callers to guess or silently do nothing.
func resolveTargets(cfg config.Config, name string) ([]config.Target, error) {
	targets := selectTargets(cfg.Targets, name)
	if len(targets) > 0 {
		return targets, nil
	}
	if name != "" {
		return nil, fmt.Errorf("no target named %q", name)
	}
	return nil, fmt.Errorf("no targets configured; run `agentcfg discover` to find agent dirs, then `agentcfg discover --add <name>`")
}
