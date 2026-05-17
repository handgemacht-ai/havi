// Package unified orchestrates `havi install` and `havi uninstall` with no
// per-IDE target argument. It detects which IDEs are present, prompts the
// user (or accepts an explicit selection via flags), runs each per-IDE writer,
// verifies its output, and prints a summary table. A failure on one IDE
// leaves the rest cleanly written; the process exit code is non-zero whenever
// any selected target's row is ✗.
package unified

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
)

// Action enumerates what to do with the selected targets.
type Action int

const (
	ActionInstall Action = iota
	ActionUninstall
)

func (a Action) String() string {
	if a == ActionUninstall {
		return "uninstall"
	}
	return "install"
}

// Options controls a unified install/uninstall run.
type Options struct {
	Action  Action
	IDEs    []string // explicit selection; empty means "use detection + prompt"
	Yes     bool     // skip prompt, use detected defaults
	All     bool     // select every known target
	Port    string   // for the optional server-health hint (defaults to 8090)
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

// Result is one row of the summary table.
type Result struct {
	Key    string // target key (codex, cursor, copilot, agents-md)
	Status string // configured | already-configured | failed | skipped
	Path   string
	Err    error
}

// Failed reports whether this result counts as ✗ in the summary.
func (r Result) Failed() bool {
	return r.Status == "failed"
}

// Run executes the install or uninstall flow described by opts. It returns
// one Result per processed target plus an aggregate error that is non-nil
// when at least one target failed.
func Run(ctx context.Context, opts Options) ([]Result, error) {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	targets := allTargets()
	keyed := indexTargets(targets)

	selected, err := chooseTargets(opts, targets, keyed, stderr)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		fmt.Fprintln(stderr, "havi: no targets selected; nothing to do")
		return nil, errors.New("no targets selected")
	}

	results := make([]Result, 0, len(selected))
	for _, t := range selected {
		results = append(results, runOne(t, opts.Action))
	}

	printSummary(stdout, opts.Action, results)

	if opts.Action == ActionInstall {
		printServerHint(ctx, stdout, opts.Port)
	}

	for _, r := range results {
		if r.Failed() {
			return results, fmt.Errorf("%d target(s) failed", countFailures(results))
		}
	}
	return results, nil
}

func runOne(t target, action Action) Result {
	r := Result{Key: t.key, Path: t.configPath}
	if t.resolveErr != nil {
		r.Status = "failed"
		r.Err = t.resolveErr
		return r
	}
	var (
		status string
		err    error
	)
	if action == ActionUninstall {
		status, err = t.uninstall(t.configPath)
	} else {
		status, err = t.install(t.configPath)
	}
	if err != nil {
		r.Status = "failed"
		r.Err = err
		return r
	}
	r.Status = status
	if action == ActionInstall && status != "failed" && t.verify != nil {
		if verr := t.verify(t.configPath); verr != nil {
			r.Status = "failed"
			r.Err = fmt.Errorf("smoke probe: %w", verr)
		}
	}
	return r
}

// chooseTargets resolves which targets to act on, honoring (in order):
//
//  1. opts.IDEs (explicit; unknown keys are an error).
//  2. opts.All (every known target).
//  3. opts.Yes (every detected target).
//  4. interactive huh checkbox (default; every detected target pre-checked).
func chooseTargets(opts Options, all []target, keyed map[string]target, errOut io.Writer) ([]target, error) {
	if len(opts.IDEs) > 0 {
		out := make([]target, 0, len(opts.IDEs))
		for _, key := range opts.IDEs {
			t, ok := keyed[key]
			if !ok {
				return nil, fmt.Errorf("unknown target %q (known: %s)", key, knownKeys(all))
			}
			out = append(out, t)
		}
		return out, nil
	}
	if opts.All {
		return all, nil
	}
	if opts.Yes {
		out := make([]target, 0, len(all))
		for _, t := range all {
			if t.detected {
				out = append(out, t)
			}
		}
		return out, nil
	}

	options := make([]huh.Option[string], 0, len(all))
	defaults := make([]string, 0, len(all))
	for _, t := range all {
		label := t.label
		if t.detected {
			label = "(detected) " + label
		} else {
			label = "(not detected) " + label
		}
		options = append(options, huh.NewOption(label, t.key))
		if t.detected {
			defaults = append(defaults, t.key)
		}
	}

	var picked []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which targets should "+opts.Action.String()+"?").
				Options(options...).
				Value(&picked),
		),
	)
	form.Init()
	for _, key := range defaults {
		picked = append(picked, key)
	}
	if err := form.Run(); err != nil {
		fmt.Fprintf(errOut, "havi: interactive prompt unavailable (%v); pass --ides or --yes\n", err)
		return nil, err
	}

	out := make([]target, 0, len(picked))
	for _, key := range picked {
		out = append(out, keyed[key])
	}
	return out, nil
}

func indexTargets(ts []target) map[string]target {
	m := make(map[string]target, len(ts))
	for _, t := range ts {
		m[t.key] = t
	}
	return m
}

func knownKeys(ts []target) string {
	keys := make([]string, 0, len(ts))
	for _, t := range ts {
		keys = append(keys, t.key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func countFailures(rs []Result) int {
	n := 0
	for _, r := range rs {
		if r.Failed() {
			n++
		}
	}
	return n
}

func printSummary(w io.Writer, action Action, results []Result) {
	if len(results) == 0 {
		return
	}
	fmt.Fprintf(w, "\nhavi %s — summary:\n", action)
	for _, r := range results {
		mark := "✓"
		if r.Failed() {
			mark = "✗"
		}
		line := fmt.Sprintf("  %s  %-10s  %s  (%s)", mark, r.Key, r.Status, r.Path)
		if r.Err != nil {
			line += "\n     error: " + r.Err.Error()
		}
		fmt.Fprintln(w, line)
	}
}

func printServerHint(ctx context.Context, w io.Writer, port string) {
	base, ok := serverHealth(ctx, port)
	if ok {
		fmt.Fprintf(w, "\n  havi server is running at %s\n", base)
		return
	}
	fmt.Fprintf(w, "\n  havi server is not running yet — start it with: havi serve --daemon\n  (your IDE will reach it at %s once it's up)\n", base)
}
