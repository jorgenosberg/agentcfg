package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/source"
	"github.com/jorgenosberg/agentcfg/internal/sync"
)

func writeItems(w io.Writer, items []source.Item) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KIND\tNAME\tPATH")
	for _, it := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", it.Kind, it.Name, it.Path)
	}
	return tw.Flush()
}

func writeStatus(w io.Writer, entries []sync.Entry) error {
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
