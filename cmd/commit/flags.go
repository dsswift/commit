package main

import (
	"flag"
	"fmt"
	"strconv"
)

// reverseFlag is a custom flag type that accepts bare --reverse (=1) or --reverse=N.
type reverseFlag int

func (r *reverseFlag) Set(s string) error {
	if s == "true" {
		*r = 1
		return nil
	}
	if s == "false" {
		*r = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("invalid reverse count %q: must be a positive integer", s)
	}
	if n < 1 {
		return fmt.Errorf("invalid reverse count %d: must be a positive integer", n)
	}
	*r = reverseFlag(n)
	return nil
}

func (r *reverseFlag) String() string   { return strconv.Itoa(int(*r)) }
func (r *reverseFlag) IsBoolFlag() bool { return true }

type flags struct {
	staged      bool
	dryRun      bool
	verbose     bool
	reverse     int
	force       bool
	interactive bool
	version     bool
	upgrade     bool
	single      bool
	smart       bool
	diffFile    string
	diffFrom    string
	diffTo      string
	provider    string
	setConfig   string
}

func parseFlags() flags {
	f := flags{}

	flag.BoolVar(&f.staged, "staged", false, "Only commit staged files")
	flag.BoolVar(&f.dryRun, "dry-run", false, "Preview commits without creating them")
	flag.BoolVar(&f.verbose, "v", false, "Verbose output")
	flag.BoolVar(&f.verbose, "verbose", false, "Verbose output")
	flag.Var((*reverseFlag)(&f.reverse), "reverse", "Reverse last N commits into uncommitted changes (default 1)")
	flag.BoolVar(&f.force, "force", false, "Force operation (for --reverse/--interactive on pushed commits)")
	flag.BoolVar(&f.interactive, "i", false, "Interactive rebase wizard")
	flag.BoolVar(&f.interactive, "interactive", false, "Interactive rebase wizard")
	flag.BoolVar(&f.version, "version", false, "Print version")
	flag.BoolVar(&f.upgrade, "upgrade", false, "Upgrade to latest version")
	flag.StringVar(&f.diffFile, "diff", "", "Analyze changes to a specific file")
	flag.StringVar(&f.diffFrom, "from", "", "Start ref for diff analysis")
	flag.StringVar(&f.diffTo, "to", "", "End ref for diff analysis")
	flag.StringVar(&f.provider, "provider", "", "Override LLM provider")
	flag.BoolVar(&f.single, "single", false, "Create a single commit for all files")
	flag.BoolVar(&f.single, "1", false, "Create a single commit for all files (shorthand)")
	flag.BoolVar(&f.smart, "smart", false, "Create semantic commits (default)")
	flag.StringVar(&f.setConfig, "set", "", "Set config value (e.g., defaultMode=single)")

	flag.Parse()

	return f
}
