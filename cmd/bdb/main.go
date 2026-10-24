package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LinduanTof/b-db/internal/app"
	"github.com/LinduanTof/b-db/internal/config"
	"github.com/LinduanTof/b-db/internal/history"
	"github.com/LinduanTof/b-db/internal/i18n"
)

var version = "dev"

func main() {
	var configPath string
	var showVersion bool
	flag.StringVar(&configPath, "config", "", "path to config file (default: $XDG_CONFIG_HOME/bdb/config.yaml)")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: bdb [--config FILE] [script.sql]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if showVersion {
		fmt.Printf("bdb %s\n", version)
		return
	}

	if flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "bdb: only one SQL script may be given")
		os.Exit(2)
	}

	cfgPath := config.ResolvePath(configPath)
	if created, err := config.EnsureFile(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "bdb: warning: could not create default config: %v\n", err)
	} else if created {
		fmt.Fprintf(os.Stderr, "bdb: created default config %s\n", cfgPath)
	}

	cfg, warnings, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bdb: %v\n", err)
		os.Exit(1)
	}
	for _, warning := range warnings {
		if strings.Contains(warning, "not found") {
			continue
		}
		fmt.Fprintf(os.Stderr, "bdb: warning: %s\n", warning)
	}

	translator := i18n.New(cfg.UI.Language)
	model := app.New(cfg, translator)
	model.SetHistory(history.Load(history.DefaultPath(), history.DefaultLimit))
	if flag.NArg() == 1 {
		model.SetInitialScript(flag.Arg(0))
	}
	program := tea.NewProgram(model)
	model.SetSender(func(msg tea.Msg) { program.Send(msg) })
	_, err = program.Run()
	// Bubble Tea handles SIGINT/SIGTERM by quitting without routing through
	// Update, so clean up regardless of how Run returned.
	model.Shutdown()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bdb: %v\n", err)
		os.Exit(1)
	}
}
