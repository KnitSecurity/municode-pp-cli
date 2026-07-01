// Copyright 2026 Ryan Jamieson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: `init` scaffolds a self-contained directory so all of the
// CLI's files (config, database, cloned <city>/ folders, cache, state) live in
// one place instead of scattered across the home directory.

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/KnitSecurity/municode-pp-cli/internal/cliutil"

	"github.com/spf13/cobra"
)

func newInitCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "init [dir]",
		Short: "Set up a self-contained directory for config, data, database, and cloned codes",
		Long: "Create a self-contained directory that holds everything this CLI writes — config, the SQLite " +
			"database, cloned <city>/ code folders, cache, and state — instead of scattering files across your " +
			"home directory.\n\n" +
			"With no argument, init targets the binary's own directory: install the binaries there " +
			"(e.g. `GOBIN=~/municode go install .../cmd/municode-pp-cli@latest`) and run `init`, and the CLI " +
			"uses that directory automatically (portable mode — no environment variable). Pass a directory to " +
			"scaffold it instead; to use a directory that is not the binary's own, set MUNICODE_HOME to it (or " +
			"pass --home). The default when nothing is configured is ~/.municode.",
		Example: "  municode-pp-cli init\n  municode-pp-cli init ~/municode",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := ""
			if len(args) == 1 {
				dir = args[0]
			} else if exe, err := os.Executable(); err == nil {
				dir = filepath.Dir(exe)
			}
			if dir == "" {
				return usageErr(fmt.Errorf("could not determine a directory; pass one: municode-pp-cli init <dir>"))
			}
			abs, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			for _, sub := range []string{"config", "data", "state", "cache"} {
				if err := os.MkdirAll(filepath.Join(abs, sub), 0o755); err != nil {
					return fmt.Errorf("creating %s: %w", sub, err)
				}
			}
			marker := filepath.Join(abs, cliutil.HomeMarkerName)
			if err := os.WriteFile(marker, []byte("municode self-contained home\n"), 0o644); err != nil {
				return fmt.Errorf("writing home marker: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Initialized municode home at %s\n", abs)
			fmt.Fprintf(out, "  config: %s\n", filepath.Join(abs, "config"))
			fmt.Fprintf(out, "  data:   %s   (database + cloned <city>/ folders)\n", filepath.Join(abs, "data"))
			fmt.Fprintf(out, "  state:  %s\n", filepath.Join(abs, "state"))
			fmt.Fprintf(out, "  cache:  %s\n", filepath.Join(abs, "cache"))

			exeDir := ""
			if exe, e := os.Executable(); e == nil {
				exeDir = filepath.Dir(exe)
			}
			home, _ := os.UserHomeDir()
			switch {
			case abs == exeDir:
				fmt.Fprintln(out, "\nThis is the binary's own directory, so it is used automatically (portable mode) — no environment variable needed.")
			case home != "" && abs == filepath.Join(home, cliutil.DefaultHomeDirName):
				fmt.Fprintln(out, "\nThis is the default location (~/.municode), so it is used automatically when no override is set.")
			default:
				fmt.Fprintf(out, "\nTo use this directory, add to your shell profile:\n  export MUNICODE_HOME=%q\nor pass --home %q on each run.\n", abs, abs)
			}
			return nil
		},
	}
}
