package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/linduan/bdb/internal/app"
	"github.com/linduan/bdb/internal/config"
	"github.com/linduan/bdb/internal/history"
	"github.com/linduan/bdb/internal/i18n"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file (default: $XDG_CONFIG_HOME/bdb/config.yaml)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: bdb [--config FILE] [script.sql]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "bdb: only one SQL script may be given")
		os.Exit(2)
	}

	cfg, warnings, err := config.Load(configPath)
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
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "bdb: %v\n", err)
		os.Exit(1)
	}
}
