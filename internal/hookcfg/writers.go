package hookcfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jorgenosberg/agentcfg/internal/fsutil"
)

func key(event, matcher, cmd string) string {
	return event + "\x00" + matcher + "\x00" + cmd
}

func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// planNested handles the Claude and Codex format: a top-level "hooks" object
// mapping event to [{matcher, hooks: [{type, command, timeout}]}]. Owned
// entries are updated in place so the order of existing entries is kept.
// It returns nil when nothing needs writing. statusField, when set, names the
// field that labels the hook (see setStatus).
func planNested(path, statusField string, want []Entry, owned, foreign func(string) bool, res *Result) (func() error, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if len(want) == 0 {
			return nil, nil
		}
		raw = []byte("{}")
	} else if err != nil {
		return nil, err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	events := map[string]any{}
	if h, ok := top["hooks"]; ok {
		dec := json.NewDecoder(bytes.NewReader(h))
		dec.UseNumber()
		if err := dec.Decode(&events); err != nil {
			return nil, fmt.Errorf("parse hooks in %s: %w", path, err)
		}
	}
	before, err := marshal(events)
	if err != nil {
		return nil, err
	}

	wantBy := map[string]Entry{}
	for _, w := range want {
		wantBy[key(w.Event, w.Matcher, w.Command)] = w
	}
	seen := map[string]bool{}
	evNames := make([]string, 0, len(events))
	for ev := range events {
		evNames = append(evNames, ev)
	}
	sort.Strings(evNames)
	for _, ev := range evNames {
		groups, ok := events[ev].([]any)
		if !ok {
			return nil, fmt.Errorf("%s: hooks.%s is not a list", path, ev)
		}
		var keptGroups []any
		for _, g := range groups {
			gm, ok := g.(map[string]any)
			if !ok {
				keptGroups = append(keptGroups, g)
				continue
			}
			matcher, _ := gm["matcher"].(string)
			hooks, _ := gm["hooks"].([]any)
			var kept []any
			dropped := false
			for _, h := range hooks {
				hm, ok := h.(map[string]any)
				cmd, _ := hm["command"].(string)
				if !ok || !owned(cmd) {
					if ok && foreign(cmd) {
						res.Foreign = append(res.Foreign, Entry{Registration: Registration{Event: ev, Matcher: matcher}, Command: cmd})
					}
					kept = append(kept, h)
					continue
				}
				k := key(ev, matcher, cmd)
				if w, ok := wantBy[k]; ok && !seen[k] {
					seen[k] = true
					timeoutChanged := setTimeout(hm, "timeout", w.Timeout)
					if setStatus(hm, statusField, w) || timeoutChanged {
						res.Updated = append(res.Updated, w)
					}
					kept = append(kept, hm)
					continue
				}
				dropped = true
				res.Removed = append(res.Removed, Entry{Registration: Registration{Event: ev, Matcher: matcher}, Script: scriptName(cmd), Command: cmd})
			}
			if !dropped {
				keptGroups = append(keptGroups, gm)
				continue
			}
			if len(kept) > 0 {
				gm["hooks"] = kept
				keptGroups = append(keptGroups, gm)
			}
		}
		if len(keptGroups) == 0 {
			delete(events, ev)
		} else {
			events[ev] = keptGroups
		}
	}

	for _, w := range want {
		k := key(w.Event, w.Matcher, w.Command)
		if seen[k] {
			continue
		}
		seen[k] = true
		hook := map[string]any{"type": "command", "command": w.Command}
		setTimeout(hook, "timeout", w.Timeout)
		setStatus(hook, statusField, w)
		groups, _ := events[w.Event].([]any)
		placed := false
		for _, g := range groups {
			gm, ok := g.(map[string]any)
			if m, _ := gm["matcher"].(string); !ok || m != w.Matcher {
				continue
			}
			hooks, _ := gm["hooks"].([]any)
			gm["hooks"] = append(hooks, hook)
			placed = true
			break
		}
		if !placed {
			gm := map[string]any{"hooks": []any{hook}}
			if w.Matcher != "" {
				gm["matcher"] = w.Matcher
			}
			events[w.Event] = append(groups, gm)
		}
		res.Added = append(res.Added, w)
	}

	after, err := marshal(events)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(before, after) {
		return nil, nil
	}
	return func() error {
		return fsutil.EditJSON(path, []byte("{}"), func(doc map[string]json.RawMessage) error {
			if len(events) == 0 {
				delete(doc, "hooks")
			} else {
				doc["hooks"] = after
			}
			return nil
		})
	}, nil
}

// setTimeout sets or clears a numeric timeout field and reports whether it changed.
func setTimeout(m map[string]any, field string, n int) bool {
	cur, has := m[field]
	if n == 0 {
		if has {
			delete(m, field)
			return true
		}
		return false
	}
	if has && fmt.Sprint(cur) == strconv.Itoa(n) {
		return false
	}
	m[field] = n
	return true
}

// setStatus labels the hook with the header's status, or with the script name
// when the header has none and the entry has no label yet. A label set by
// hand is kept unless the header overrides it. It reports whether it changed.
func setStatus(m map[string]any, field string, w Entry) bool {
	if field == "" {
		return false
	}
	cur, _ := m[field].(string)
	want := w.Status
	if want == "" {
		if cur != "" {
			return false
		}
		want = strings.TrimSuffix(w.Script, filepath.Ext(w.Script))
	}
	if cur == want {
		return false
	}
	m[field] = want
	return true
}

func scriptName(cmd string) string {
	for i := len(cmd) - 1; i >= 0; i-- {
		if cmd[i] == '/' {
			return trimQuote(cmd[i+1:])
		}
	}
	return trimQuote(cmd)
}

func trimQuote(s string) string {
	if n := len(s); n > 0 && (s[n-1] == '\'' || s[n-1] == '"') {
		return s[:n-1]
	}
	return s
}

type copilotFile struct {
	Version int                      `json:"version"`
	Hooks   map[string][]copilotHook `json:"hooks"`
}

type copilotHook struct {
	Type       string `json:"type"`
	Matcher    string `json:"matcher,omitempty"`
	Bash       string `json:"bash"`
	TimeoutSec int    `json:"timeoutSec,omitempty"`
}

// planCopilot handles the agentcfg-owned Copilot file. The whole file is
// regenerated, and removed when no script is registered.
func planCopilot(path string, want []Entry, res *Result) (func() error, error) {
	type oldHook struct {
		event string
		hook  copilotHook
	}
	old := map[string]oldHook{}
	var oldOrder []string
	raw, err := os.ReadFile(path)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if exists {
		var f copilotFile
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for ev, hooks := range f.Hooks {
			for _, h := range hooks {
				k := key(ev, h.Matcher, h.Bash)
				old[k] = oldHook{ev, h}
				oldOrder = append(oldOrder, k)
			}
		}
	}

	f := copilotFile{Version: 1, Hooks: map[string][]copilotHook{}}
	seen := map[string]bool{}
	for _, w := range want {
		k := key(w.Event, w.Matcher, w.Command)
		if seen[k] {
			continue
		}
		seen[k] = true
		h := copilotHook{Type: "command", Matcher: w.Matcher, Bash: w.Command, TimeoutSec: w.Timeout}
		f.Hooks[w.Event] = append(f.Hooks[w.Event], h)
		if o, ok := old[k]; !ok {
			res.Added = append(res.Added, w)
		} else if o.hook.TimeoutSec != h.TimeoutSec {
			res.Updated = append(res.Updated, w)
		}
	}
	sort.Strings(oldOrder)
	for _, k := range oldOrder {
		if seen[k] {
			continue
		}
		o := old[k]
		h := o.hook
		res.Removed = append(res.Removed, Entry{Registration: Registration{Event: o.event, Matcher: h.Matcher, Timeout: h.TimeoutSec}, Script: scriptName(h.Bash), Command: h.Bash})
	}

	if len(want) == 0 {
		if !exists {
			return nil, nil
		}
		return func() error { return os.Remove(path) }, nil
	}
	body, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	if exists && bytes.Equal(raw, body) {
		return nil, nil
	}
	return func() error { return fsutil.AtomicWrite(path, body) }, nil
}
