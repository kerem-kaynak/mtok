// mtok — a terminal dashboard for AI token usage and spend, computed entirely
// from local Claude Code and Codex session logs. No network calls.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kerem-kaynak/mtok/internal/pricing"
	"github.com/kerem-kaynak/mtok/internal/tui"
)

// Overridden at release time via -ldflags "-X main.version=...".
var version = "0.3.2"

type multiFlag []string

func (m *multiFlag) String() string     { return fmt.Sprint(*m) }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

// resolveDirs picks the data roots for one source: explicit flags win, then
// the config file, then the relocation env var the tool itself honors
// (CLAUDE_CONFIG_DIR / CODEX_HOME), then the conventional home location.
func resolveDirs(flags, cfg []string, envVar, fallback string) []string {
	if len(flags) > 0 {
		return flags
	}
	if len(cfg) > 0 {
		return cfg
	}
	if dir := os.Getenv(envVar); dir != "" {
		return []string{dir}
	}
	return []string{fallback}
}

func main() {
	var (
		claudeDirs  multiFlag
		codexDirs   multiFlag
		summary     = flag.Bool("summary", false, "print a plain-text report instead of the TUI")
		noCache     = flag.Bool("no-cache", false, "reparse all files, ignoring the parse cache")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Var(&claudeDirs, "claude-dir", "Claude Code data dir (repeatable; default $CLAUDE_CONFIG_DIR or ~/.claude)")
	flag.Var(&codexDirs, "codex-dir", "Codex data dir (repeatable; default $CODEX_HOME or ~/.codex)")
	flag.Parse()

	if *showVersion {
		fmt.Println("mtok " + version)
		return
	}

	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "mtok", "config.json")
	cfg, err := pricing.LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mtok: bad config %s: %v\n", cfgPath, err)
		os.Exit(1)
	}

	table := pricing.Defaults()
	table.Apply(cfg.Pricing)

	claudeDirs = resolveDirs(claudeDirs, cfg.ClaudeDirs, "CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	codexDirs = resolveDirs(codexDirs, cfg.CodexDirs, "CODEX_HOME", filepath.Join(home, ".codex"))

	cacheFile := filepath.Join(home, ".cache", "mtok", "scan.gob")
	if *noCache {
		cacheFile = ""
	}

	opts := tui.Options{
		ClaudeDirs: claudeDirs,
		CodexDirs:  codexDirs,
		CacheFile:  cacheFile,
		Table:      table,
		Version:    version,
	}

	if *summary {
		err = tui.Summary(opts)
	} else {
		err = tui.Run(opts)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mtok:", err)
		os.Exit(1)
	}
}
