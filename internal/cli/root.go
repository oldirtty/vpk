package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/oldirtty/vpk/internal/xbps"
)

// version is set at build time: go build -ldflags "-X github.com/oldirtty/vpk/internal/cli.version=1.2.3"
var version = "dev"

// run is a variable so tests can replace it.
var run = xbps.Run

// opts holds the values of all flags. Each command registers only the
// flags it supports (see commands.go).
type opts struct {
	yes          bool
	sim          bool
	verbose      bool
	force        bool
	forceRevdeps bool
	updateCache  bool
	upgrade      bool
	all          bool
	installed    bool
	available    bool
	manual       bool
	orphans      bool
	held         bool
	upgradable   bool
	tree         bool
	regex        bool
	automatic    bool
	downloadOnly bool
	root         string
	property     string
	group        string
	repos        []string
	ignore       []string
}

// Execute runs vpk and returns the process exit code.
func Execute() int {
	var o opts
	code := 0

	if err := newRoot(&o, &code).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "vpk:", err)
		return 2
	}
	return code
}

func newRoot(o *opts, code *int) *cobra.Command {
	root := &cobra.Command{
		Use:           "vpk",
		Short:         "Void Package Keeper: apk-style frontend for xbps",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// cobra would bind -v to --version, which clashes with --verbose elsewhere.
	root.Flags().BoolP("version", "V", false, "print the vpk version")
	root.SetVersionTemplate("vpk {{.Version}}\n")

	groups := map[string]*cobra.Command{}
	for _, s := range specs {
		cmd := s.command(o, code)
		if s.parent == "" {
			root.AddCommand(cmd)
			continue
		}
		g, ok := groups[s.parent]
		if !ok {
			info := parents[s.parent]
			g = &cobra.Command{
				Use:     s.parent,
				Aliases: info.aliases,
				Short:   info.short,
				// Runnable on purpose: otherwise cobra prints the help and
				// exits 0 for "vpk mark bogus".
				RunE: func(cmd *cobra.Command, args []string) error {
					if len(args) > 0 {
						return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
					}
					return cmd.Help()
				},
			}
			groups[s.parent] = g
			root.AddCommand(g)
		}
		g.AddCommand(cmd)
	}

	return root
}
