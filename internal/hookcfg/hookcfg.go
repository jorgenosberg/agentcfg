// Package hookcfg registers installed hook scripts in each agent's hook
// configuration. A script opts in with one header line per registration:
//
//	# agentcfg-hook: PreToolUse Bash timeout=10
//
// Scripts without a header are installed but never registered.
package hookcfg

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jorgenosberg/agentcfg/internal/agent"
	"github.com/jorgenosberg/agentcfg/internal/config"
	"github.com/jorgenosberg/agentcfg/internal/paths"
	"github.com/jorgenosberg/agentcfg/internal/source"
)

const headerPrefix = "agentcfg-hook:"

// gitRoot expands to the repository root inside a hook command. Codex and
// Copilot have no project-dir variable like Claude's $CLAUDE_PROJECT_DIR.
const gitRoot = `"$(git rev-parse --show-toplevel)"`

// rootPrefixes are the forms of a repository-root prefix recognised when
// deciding whether a project hook command points at a managed script.
var rootPrefixes = []string{
	`"$CLAUDE_PROJECT_DIR"`, `$CLAUDE_PROJECT_DIR`, `"${CLAUDE_PROJECT_DIR}"`, `${CLAUDE_PROJECT_DIR}`,
	gitRoot, `$(git rev-parse --show-toplevel)`,
}

// Registration is one header line: when the agent should run the script.
type Registration struct {
	Event   string
	Matcher string // empty means every tool, or no matcher for the event
	Timeout int    // seconds; 0 leaves the agent default
	Status  string // label shown by agents that support one (Codex statusMessage)
}

func (r Registration) String() string {
	s := r.Event
	if r.Matcher != "" {
		s += " " + r.Matcher
	}
	if r.Timeout > 0 {
		s += " timeout=" + strconv.Itoa(r.Timeout)
	}
	if r.Status != "" {
		s += ` status="` + r.Status + `"`
	}
	return s
}

// Parse reads the registration headers from the script's leading comment block.
func Parse(path string) ([]Registration, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var regs []Registration
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			break
		}
		rest, ok := strings.CutPrefix(strings.TrimSpace(strings.TrimPrefix(line, "#")), headerPrefix)
		if !ok {
			continue
		}
		r, err := parseLine(rest)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		regs = append(regs, r)
	}
	return regs, sc.Err()
}

func parseLine(s string) (Registration, error) {
	var status string
	if before, rest, ok := strings.Cut(s, `status="`); ok {
		value, after, closed := strings.Cut(rest, `"`)
		if !closed || value == "" {
			return Registration{}, fmt.Errorf(`status in %s header needs a closing quote and a value`, headerPrefix)
		}
		status, s = value, before+" "+after
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return Registration{}, fmt.Errorf("%s header has no event", headerPrefix)
	}
	r := Registration{Event: fields[0], Status: status}
	for _, f := range fields[1:] {
		if v, ok := strings.CutPrefix(f, "timeout="); ok {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				return r, fmt.Errorf("invalid timeout %q", v)
			}
			r.Timeout = n
			continue
		}
		if r.Matcher != "" {
			return r, fmt.Errorf("unexpected %q in %s header (matchers cannot contain spaces)", f, headerPrefix)
		}
		r.Matcher = f
	}
	return r, nil
}

// layout describes where an agent keeps hook registrations.
type layout struct {
	global, project string // config file relative to the target path
	prefix          string // repository root in project hook commands
	statusField     string // entry field holding the hook's label, if the agent has one
	copilot         bool   // agentcfg-owned Copilot file instead of a shared settings file
}

var layouts = map[string]layout{
	agent.Claude:  {global: "settings.json", project: ".claude/settings.json", prefix: `"$CLAUDE_PROJECT_DIR"`},
	agent.Codex:   {global: "hooks.json", project: ".codex/hooks.json", prefix: gitRoot, statusField: "statusMessage"},
	agent.Copilot: {global: "hooks/agentcfg.json", project: ".github/hooks/agentcfg.json", prefix: gitRoot, copilot: true},
}

func layoutFor(t config.Target) (layout, bool) {
	l, ok := layouts[t.Agent]
	return l, ok && t.SupportsKind(source.KindHook) && t.SubdirFor(source.KindHook) != ""
}

// Supported reports whether agentcfg can register hooks for the target.
func Supported(t config.Target) bool {
	_, ok := layoutFor(t)
	return ok
}

// ConfigFile returns the file holding the target's hook registrations, or ""
// when the target is not supported.
func ConfigFile(t config.Target) string {
	l, ok := layoutFor(t)
	if !ok {
		return ""
	}
	if t.Project {
		return filepath.Join(t.Path, l.project)
	}
	return filepath.Join(t.Path, l.global)
}

func hookDir(t config.Target) string {
	return filepath.Join(t.Path, t.SubdirFor(source.KindHook))
}

// Command returns the command line that runs the named script on the target.
func Command(t config.Target, name string) string {
	l, _ := layoutFor(t)
	if t.Project {
		return l.prefix + "/" + filepath.ToSlash(filepath.Join(t.SubdirFor(source.KindHook), name))
	}
	return shellQuote(filepath.Join(hookDir(t), name))
}

func shellQuote(p string) string {
	if !strings.ContainsAny(p, " \t'\"$`\\") {
		return p
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}

// resolve returns the script path a hook command runs, or "" when the
// command is not a bare script path.
func resolve(t config.Target, cmd string) string {
	s := strings.TrimSpace(cmd)
	for _, p := range rootPrefixes {
		if rest, ok := strings.CutPrefix(s, p+"/"); ok {
			if !t.Project || strings.ContainsAny(rest, " \t") {
				return ""
			}
			return filepath.Join(t.Path, rest)
		}
	}
	quoted := len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\''
	if quoted {
		s = strings.ReplaceAll(s[1:len(s)-1], `'\''`, "'")
	} else if strings.ContainsAny(s, " \t") {
		return ""
	}
	for _, p := range []string{"~/", "$HOME/", "${HOME}/"} {
		if rest, ok := strings.CutPrefix(s, p); ok {
			home, err := paths.Home()
			if err != nil {
				return ""
			}
			s = filepath.Join(home, rest)
			break
		}
	}
	if !filepath.IsAbs(s) {
		return ""
	}
	return filepath.Clean(s)
}

// Entry is one registration of one script.
type Entry struct {
	Registration
	Script  string // script file name
	Command string
}

// Result is the outcome of reconciling one target.
type Result struct {
	Target  config.Target
	File    string
	Added   []Entry
	Updated []Entry // timeout changed
	Removed []Entry // script gone, header removed, or registration dropped
	// Foreign lists registrations agentcfg does not own that run a script
	// with the same name as a managed one, so it may run twice.
	Foreign []Entry
	Written bool
	Err     error
}

// Changed reports whether the registrations differ from the headers.
func (r Result) Changed() bool {
	return len(r.Added)+len(r.Updated)+len(r.Removed) > 0
}

// scripts returns the registrations declared by each script in the target's
// hook directory, keyed by file name. Headerless scripts map to nil.
func scripts(dir string) (map[string][]Registration, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return map[string][]Registration{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string][]Registration{}
	for _, e := range entries {
		if source.SkipHookEntry(e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			continue
		}
		regs, err := Parse(p)
		if err != nil {
			return nil, err
		}
		out[e.Name()] = regs
	}
	return out, nil
}

// Reconcile makes the target's hook registrations match the headers of the
// scripts installed in its hook directory. agentcfg owns only entries whose
// command runs a script in that directory that either carries a header or no
// longer exists; everything else is left alone. The file is written only when
// its registrations change. With dryRun nothing is written.
func Reconcile(t config.Target, dryRun bool) Result {
	res := Result{Target: t, File: ConfigFile(t)}
	l, ok := layoutFor(t)
	if !ok {
		res.Err = fmt.Errorf("%s: hook registration is not supported for agent %q", t.Name, t.Agent)
		return res
	}
	dir := hookDir(t)
	sc, err := scripts(dir)
	if err != nil {
		res.Err = err
		return res
	}
	names := make([]string, 0, len(sc))
	for n := range sc {
		names = append(names, n)
	}
	sort.Strings(names)
	var want []Entry
	for _, n := range names {
		for _, r := range sc[n] {
			want = append(want, Entry{Registration: r, Script: n, Command: Command(t, n)})
		}
	}
	owned := func(cmd string) bool {
		p := resolve(t, cmd)
		if p == "" || filepath.Dir(p) != dir {
			return false
		}
		regs, exists := sc[filepath.Base(p)]
		return !exists || len(regs) > 0
	}
	foreign := func(cmd string) bool {
		p := resolve(t, cmd)
		return p != "" && filepath.Dir(p) != dir && len(sc[filepath.Base(p)]) > 0
	}

	var write func() error
	if l.copilot {
		write, err = planCopilot(res.File, want, &res)
	} else {
		write, err = planNested(res.File, l.statusField, want, owned, foreign, &res)
	}
	if err != nil {
		res.Err = err
		return res
	}
	if write != nil && !dryRun {
		if err := write(); err != nil {
			res.Err = err
			return res
		}
		res.Written = true
	}
	return res
}

// ReconcileAll reconciles every supported target, skipping duplicates that
// share a config file.
func ReconcileAll(targets []config.Target, dryRun bool) []Result {
	var out []Result
	seen := map[string]bool{}
	for _, t := range targets {
		f := ConfigFile(t)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, Reconcile(t, dryRun))
	}
	return out
}
