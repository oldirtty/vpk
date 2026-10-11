package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oldirtty/vpk/internal/xbps"
)

// spec is the single source of truth for a vpk command.
type spec struct {
	parent  string // groups the command under "vpk <parent> <name>" (see parents)
	name    string
	usage   string
	aliases []string
	short   string
	args    cobra.PositionalArgs
	flags   func(*cobra.Command, *opts)
	build   func(args []string, o opts) xbps.Cmd
	pkgs    func() ([]string, error) // package names for <TAB>, optional
	paths   bool                     // the argument is a file path: let the shell complete it
}

// parents are the commands that only group subcommands.
var parents = map[string]struct {
	short   string
	aliases []string
}{
	"cache":        {"Manage the package cache", nil},
	"mark":         {"Change how packages are tracked", nil},
	"repo":         {"Inspect the repositories", nil},
	"alternatives": {"List or set alternatives", []string{"alt"}},
}

// properties are the values xbps-query --property accepts (xbps-query(1)).
var properties = []string{
	"alternatives", "architecture", "automatic-install", "build-options",
	"changelog", "conf_files", "conflicts", "filename-sha256", "filename-size",
	"hold", "homepage", "install-date", "install-msg", "install-script",
	"installed_size", "license", "maintainer", "metafile-sha256", "pkgname",
	"pkgver", "preserve", "provides", "remove-msg", "remove-script", "replaces",
	"repolock", "repository", "reverts", "run_depends", "shlib-provides",
	"shlib-requires", "short_desc", "source-revisions", "state", "tags",
}

var cleanSpec = spec{
	name:    "clean",
	aliases: []string{"autoclean"},
	short:   "Remove outdated packages from the cache",
	args:    cobra.NoArgs,
	flags: func(c *cobra.Command, o *opts) {
		common(c, o)
		simulate(c, o)
		c.Flags().BoolVar(&o.all, "all", false, "also remove packages no longer installed")
	},
	build: func(_ []string, o opts) xbps.Cmd {
		base := []string{"-O"}
		if o.all {
			base = append(base, "-O")
		}
		return mutate("xbps-remove", o, base, nil)
	},
}

// apk compatibility: "vpk cache clean" is the same as "vpk clean".
var cacheCleanSpec = func() spec {
	s := cleanSpec
	s.parent = "cache"
	s.aliases = nil
	return s
}()

var specs = []spec{
	{
		name:    "add",
		usage:   "<pkg>...",
		aliases: []string{"install", "in", "i"},
		short:   "Install packages",
		args:    cobra.MinimumNArgs(1),
		pkgs:    xbps.Available,
		flags: func(c *cobra.Command, o *opts) {
			common(c, o)
			simulate(c, o)
			repository(c, o)
			f := c.Flags()
			f.BoolVarP(&o.updateCache, "update-cache", "U", false, "sync the repository index first")
			f.BoolVarP(&o.upgrade, "upgrade", "u", false, "upgrade the named packages if already installed")
			f.BoolVarP(&o.force, "force", "f", false, "force reinstall")
			f.BoolVarP(&o.automatic, "automatic", "A", false, "mark the packages as installed automatically")
			f.BoolVarP(&o.downloadOnly, "download-only", "D", false, "only download the packages and check them")
		},
		build: func(args []string, o opts) xbps.Cmd {
			var base []string
			if o.updateCache {
				base = append(base, "-S")
			}
			if o.upgrade {
				base = append(base, "-u")
			}
			if o.force {
				base = append(base, "-f")
			}
			if o.automatic {
				base = append(base, "-A")
			}
			if o.downloadOnly {
				base = append(base, "-D")
			}
			return mutate("xbps-install", o, append(base, repoArgs(o)...), args)
		},
	},
	{
		name:    "autoremove",
		aliases: []string{"orphans", "prune"},
		short:   "Remove orphaned packages",
		args:    cobra.NoArgs,
		flags: func(c *cobra.Command, o *opts) {
			common(c, o)
			simulate(c, o)
		},
		build: func(_ []string, o opts) xbps.Cmd {
			return mutate("xbps-remove", o, []string{"-o"}, nil)
		},
	},
	{
		name:    "del",
		usage:   "<pkg>...",
		aliases: []string{"remove", "rm", "erase", "uninstall"},
		short:   "Remove packages and their unused dependencies",
		args:    cobra.MinimumNArgs(1),
		pkgs:    xbps.Installed,
		flags: func(c *cobra.Command, o *opts) {
			common(c, o)
			simulate(c, o)
			c.Flags().BoolVar(&o.forceRevdeps, "force-revdeps", false, "remove even if other packages depend on it")
		},
		build: func(args []string, o opts) xbps.Cmd {
			base := []string{"-R"}
			if o.forceRevdeps {
				base = append(base, "-F")
			}
			return mutate("xbps-remove", o, base, args)
		},
	},
	{
		name:    "files",
		usage:   "<pkg>",
		aliases: []string{"contents"},
		short:   "List the files of an installed package",
		args:    cobra.ExactArgs(1),
		pkgs:    xbps.Installed,
		flags:   availableFlag,
		build: func(args []string, o opts) xbps.Cmd {
			return query(o.available, "f", args[0])
		},
	},
	{
		name:    "hold",
		usage:   "<pkg>...",
		aliases: []string{"pin"},
		short:   "Keep packages at their current version",
		args:    cobra.MinimumNArgs(1),
		pkgs:    xbps.Installed,
		build:   pkgdb("hold"),
	},
	{
		name:    "update",
		aliases: []string{"refresh", "makecache", "sync"},
		short:   "Update the repository index",
		args:    cobra.NoArgs,
		flags: func(c *cobra.Command, o *opts) {
			common(c, o)
			repository(c, o)
		},
		build: func(_ []string, o opts) xbps.Cmd {
			return mutate("xbps-install", o, append([]string{"-S"}, repoArgs(o)...), nil)
		},
	},
	{
		name:    "upgrade",
		aliases: []string{"up", "dist-upgrade", "full-upgrade", "dup"},
		short:   "Upgrade installed packages",
		args:    cobra.NoArgs,
		flags: func(c *cobra.Command, o *opts) {
			common(c, o)
			simulate(c, o)
			repository(c, o)
			c.Flags().BoolVarP(&o.downloadOnly, "download-only", "D", false, "only download the packages and check them")
		},
		build: func(_ []string, o opts) xbps.Cmd {
			base := []string{"-Su"}
			if o.downloadOnly {
				base = append(base, "-D")
			}
			c := mutate("xbps-install", o, append(base, repoArgs(o)...), nil)
			// xbps updates itself in a separate transaction: go again if more is
			// pending. Not for a dry-run, a download or another root.
			c.Again = !o.sim && !o.downloadOnly && o.root == ""
			return c
		},
	},
	{
		name:    "search",
		usage:   "<term>",
		aliases: []string{"find", "se", "s"},
		short:   "Search packages in the repositories",
		args:    cobra.ExactArgs(1),
		flags: func(c *cobra.Command, o *opts) {
			c.Flags().BoolVar(&o.regex, "regex", false, "treat the term as a regular expression")
			propertyFlag(c, o)
		},
		build: func(args []string, o opts) xbps.Cmd {
			var pre []string
			if o.regex {
				pre = append(pre, "--regex")
			}
			return xbps.Cmd{Bin: "xbps-query", Args: append(append(pre, propertyArgs(o)...), "-Rs", args[0])}
		},
	},
	{
		name:    "list",
		aliases: []string{"ls"},
		short:   "List installed, available, manual, orphaned, held or upgradable packages",
		args:    cobra.NoArgs,
		flags: func(c *cobra.Command, o *opts) {
			f := c.Flags()
			f.BoolVarP(&o.installed, "installed", "I", false, "installed packages (default)")
			f.BoolVarP(&o.available, "available", "a", false, "packages in the repositories")
			f.BoolVarP(&o.manual, "manual", "m", false, "packages installed explicitly")
			f.BoolVarP(&o.orphans, "orphans", "o", false, "dependencies nothing needs anymore")
			f.BoolVarP(&o.held, "held", "H", false, "packages on hold")
			f.BoolVarP(&o.upgradable, "upgradable", "u", false, "packages with an update (from the last synced index)")
			c.MarkFlagsMutuallyExclusive("installed", "available", "manual", "orphans", "held", "upgradable")
		},
		build: func(_ []string, o opts) xbps.Cmd {
			switch {
			case o.available:
				return xbps.Cmd{Bin: "xbps-query", Args: []string{"-Rs", ""}}
			case o.manual:
				return xbps.Cmd{Bin: "xbps-query", Args: []string{"-m"}}
			case o.orphans:
				return xbps.Cmd{Bin: "xbps-query", Args: []string{"-O"}}
			case o.held:
				return xbps.Cmd{Bin: "xbps-query", Args: []string{"-H"}}
			case o.upgradable:
				return xbps.Cmd{Bin: "xbps-install", Args: []string{"-un"}}
			}
			return xbps.Cmd{Bin: "xbps-query", Args: []string{"-l"}}
		},
	},
	{
		name:    "info",
		usage:   "<pkg>",
		aliases: []string{"show"},
		short:   "Show information about a package",
		args:    cobra.ExactArgs(1),
		pkgs:    xbps.Available,
		flags: func(c *cobra.Command, o *opts) {
			installedFlag(c, o)
			propertyFlag(c, o)
		},
		build: func(args []string, o opts) xbps.Cmd {
			return query(!o.installed, "S", args[0], propertyArgs(o)...)
		},
	},
	{
		name:    "deps",
		usage:   "<pkg>",
		aliases: []string{"depends"},
		short:   "Show the dependencies of a package",
		args:    cobra.ExactArgs(1),
		pkgs:    xbps.Available,
		flags: func(c *cobra.Command, o *opts) {
			installedFlag(c, o)
			c.Flags().BoolVar(&o.tree, "tree", false, "show the full dependency tree")
		},
		build: func(args []string, o opts) xbps.Cmd {
			var pre []string
			if o.tree {
				pre = []string{"--fulldeptree"}
			}
			return query(!o.installed, "x", args[0], pre...)
		},
	},
	{
		name:    "rdeps",
		usage:   "<pkg>",
		aliases: []string{"why", "needed-by"},
		short:   "Show the packages that depend on a package",
		args:    cobra.ExactArgs(1),
		pkgs:    xbps.Installed,
		flags:   availableFlag,
		build: func(args []string, o opts) xbps.Cmd {
			return query(o.available, "X", args[0])
		},
	},
	{
		name:    "owns",
		usage:   "<file>",
		aliases: []string{"provides", "whatprovides"},
		short:   "Find the installed package that owns a file",
		args:    cobra.ExactArgs(1),
		paths:   true,
		build: func(args []string, _ opts) xbps.Cmd {
			return xbps.Cmd{Bin: "xbps-query", Args: []string{"-o", args[0]}}
		},
	},
	{
		name:    "reinstall",
		usage:   "<pkg>...",
		aliases: []string{"fix"},
		short:   "Reinstall packages",
		args:    cobra.MinimumNArgs(1),
		pkgs:    xbps.Installed,
		flags: func(c *cobra.Command, o *opts) {
			common(c, o)
			simulate(c, o)
		},
		build: func(args []string, o opts) xbps.Cmd {
			return mutate("xbps-install", o, []string{"-f"}, args)
		},
	},
	{
		name:    "unhold",
		usage:   "<pkg>...",
		aliases: []string{"unpin"},
		short:   "Let held packages be upgraded again",
		args:    cobra.MinimumNArgs(1),
		pkgs:    xbps.Held,
		build:   pkgdb("unhold"),
	},
	{
		name:    "verify",
		usage:   "[pkg]...",
		aliases: []string{"check"},
		short:   "Check installed packages (all of them if none is given)",
		args:    cobra.ArbitraryArgs,
		pkgs:    xbps.Installed,
		build: func(args []string, _ opts) xbps.Cmd {
			if len(args) == 0 {
				return xbps.Cmd{Bin: "xbps-pkgdb", Args: []string{"-a"}, Root: true}
			}
			return xbps.Cmd{Bin: "xbps-pkgdb", Args: append([]string{"--"}, args...), Root: true}
		},
	},
	{
		name:    "reconfigure",
		usage:   "<pkg>... | --all",
		aliases: []string{"configure"},
		short:   "Run the configuration step of packages again",
		args:    reconfigureArgs,
		pkgs:    xbps.Installed,
		flags: func(c *cobra.Command, o *opts) {
			f := c.Flags()
			f.BoolVarP(&o.all, "all", "a", false, "all packages")
			f.BoolVarP(&o.force, "force", "f", false, "even if already configured")
			f.StringArrayVarP(&o.ignore, "ignore", "i", nil, "skip this package with --all (repeatable)")
			f.BoolVarP(&o.verbose, "verbose", "v", false, "verbose output")
			f.StringVarP(&o.root, "root", "p", "", "use this directory as the system root")
		},
		build: func(args []string, o opts) xbps.Cmd {
			var a []string
			if o.force {
				a = append(a, "-f")
			}
			if o.all {
				a = append(a, "-a")
			}
			for _, p := range o.ignore {
				a = append(a, "-i", p)
			}
			if o.verbose {
				a = append(a, "-v")
			}
			if o.root != "" {
				a = append(a, "-r", o.root)
			}
			if len(args) > 0 {
				a = append(a, "--")
				a = append(a, args...)
			}
			return xbps.Cmd{Bin: "xbps-reconfigure", Args: a, Root: true}
		},
	},
	cleanSpec,
	cacheCleanSpec,
	markSpec("auto", "Mark packages as installed automatically", xbps.Manual),
	markSpec("manual", "Mark packages as installed explicitly", xbps.Installed),
	markSpec("repolock", "Lock packages to their current repository", xbps.Installed),
	markSpec("repounlock", "Remove the repository lock of packages", xbps.Repolocked),
	{
		parent:  "repo",
		name:    "list",
		aliases: []string{"ls"},
		short:   "List the repositories",
		args:    cobra.NoArgs,
		flags: func(c *cobra.Command, o *opts) {
			c.Flags().BoolVarP(&o.verbose, "verbose", "v", false, "show more details")
		},
		build: func(_ []string, o opts) xbps.Cmd {
			var a []string
			if o.verbose {
				a = append(a, "-v")
			}
			return xbps.Cmd{Bin: "xbps-query", Args: append(a, "-L")}
		},
	},
	{
		parent:  "alternatives",
		name:    "list",
		usage:   "[pkg]",
		aliases: []string{"ls"},
		short:   "List the alternative groups",
		args:    cobra.MaximumNArgs(1),
		pkgs:    xbps.Installed,
		flags:   groupFlag,
		build: func(args []string, o opts) xbps.Cmd {
			return xbps.Cmd{Bin: "xbps-alternatives", Args: append(groupArgs(o), append([]string{"-l"}, args...)...)}
		},
	},
	{
		parent: "alternatives",
		name:   "set",
		usage:  "<pkg>",
		short:  "Make a package the provider of its alternative groups",
		args:   cobra.ExactArgs(1),
		pkgs:   xbps.Installed,
		flags:  groupFlag,
		build: func(args []string, o opts) xbps.Cmd {
			return xbps.Cmd{Bin: "xbps-alternatives", Args: append(groupArgs(o), "-s", args[0]), Root: true}
		},
	},
}

// markSpec is a "vpk mark <mode>" command: xbps-pkgdb -m <mode>.
func markSpec(mode, short string, pkgs func() ([]string, error)) spec {
	return spec{
		parent: "mark",
		name:   mode,
		usage:  "<pkg>...",
		short:  short,
		args:   cobra.MinimumNArgs(1),
		pkgs:   pkgs,
		build:  pkgdb(mode),
	}
}

// pkgdb builds an xbps-pkgdb command that sets a mode on packages.
func pkgdb(mode string) func([]string, opts) xbps.Cmd {
	return func(args []string, _ opts) xbps.Cmd {
		return xbps.Cmd{
			Bin:  "xbps-pkgdb",
			Args: append([]string{"-m", mode, "--"}, args...),
			Root: true,
		}
	}
}

// reconfigureArgs wants packages or --all, and --ignore only with --all.
func reconfigureArgs(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	ignore, _ := cmd.Flags().GetStringArray("ignore")
	switch {
	case all && len(args) > 0:
		return errors.New("--all takes no package names")
	case !all && len(args) == 0:
		return errors.New("give at least one package, or --all")
	case !all && len(ignore) > 0:
		return errors.New("--ignore only works together with --all")
	}
	return nil
}

// Flag groups, in the style of apk.

func common(c *cobra.Command, o *opts) {
	f := c.Flags()
	f.BoolVarP(&o.verbose, "verbose", "v", false, "verbose output")
	f.BoolVarP(&o.yes, "yes", "y", false, "assume yes to all questions")
	f.StringVarP(&o.root, "root", "p", "", "use this directory as the system root")
}

func simulate(c *cobra.Command, o *opts) {
	f := c.Flags()
	f.BoolVarP(&o.sim, "simulate", "s", false, "show what would be done, change nothing (also --dry-run, -n)")
	f.BoolVarP(&o.sim, "dry-run", "n", false, "same as --simulate")
	_ = f.MarkHidden("dry-run")
}

func repository(c *cobra.Command, o *opts) {
	c.Flags().StringArrayVarP(&o.repos, "repository", "X", nil, "extra repository URL (repeatable)")
}

func installedFlag(c *cobra.Command, o *opts) {
	c.Flags().BoolVarP(&o.installed, "installed", "I", false, "use the installed packages instead of the repositories")
}

func availableFlag(c *cobra.Command, o *opts) {
	c.Flags().BoolVarP(&o.available, "available", "a", false, "use the repositories instead of the installed packages")
}

func groupFlag(c *cobra.Command, o *opts) {
	c.Flags().StringVarP(&o.group, "group", "g", "", "alternative group name")
}

func propertyFlag(c *cobra.Command, o *opts) {
	c.Flags().StringVar(&o.property, "property", "", "show only this package property")
	_ = c.RegisterFlagCompletionFunc("property", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var out []string
		for _, p := range properties {
			if strings.HasPrefix(p, toComplete) {
				out = append(out, p)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	})
}

func propertyArgs(o opts) []string {
	if o.property == "" {
		return nil
	}
	return []string{"-p", o.property}
}

func groupArgs(o opts) []string {
	if o.group == "" {
		return nil
	}
	return []string{"-g", o.group}
}

// query builds an xbps-query command for one mode ("S" show, "x" deps,
// "X" revdeps, "f" files). remote selects repository mode (-R); otherwise
// the installed packages are queried. pre holds options that must come
// before the mode.
func query(remote bool, mode, pkg string, pre ...string) xbps.Cmd {
	flag := "-" + mode
	if remote {
		flag = "-R" + mode
	}
	return xbps.Cmd{Bin: "xbps-query", Args: append(slices.Clone(pre), flag, pkg)}
}

// repoArgs returns -R flags. Only valid for xbps-install, where -R is
// "repository"; in xbps-remove -R means "recursive".
func repoArgs(o opts) []string {
	var args []string
	for _, r := range o.repos {
		args = append(args, "-R", r)
	}
	return args
}

// mutate builds an xbps command with the shared flags. It runs as root,
// except for a harmless dry-run (see needsRoot).
// Package names go after "--" so a name starting with "-" is never a flag.
func mutate(bin string, o opts, base, pkgs []string) xbps.Cmd {
	args := append([]string{}, base...)
	if o.sim {
		args = append(args, "-n")
	}
	if o.yes {
		args = append(args, "-y")
	}
	if o.verbose {
		args = append(args, "-v")
	}
	if o.root != "" {
		args = append(args, "-r", o.root)
	}
	if len(pkgs) > 0 {
		args = append(args, "--")
		args = append(args, pkgs...)
	}
	return xbps.Cmd{Bin: bin, Args: args, Root: needsRoot(bin, o, base)}
}

func (s spec) command(o *opts, code *int) *cobra.Command {
	c := &cobra.Command{
		Use:     strings.TrimSpace(s.name + " " + s.usage),
		Aliases: s.aliases,
		Short:   s.short,
		Args:    s.args,
		Run: func(_ *cobra.Command, args []string) {
			*code = run(s.build(args, *o))
		},
	}
	if s.flags != nil {
		s.flags(c, o)
	}
	switch {
	case s.pkgs != nil:
		c.ValidArgsFunction = pkgCompletion(s.pkgs)
	case !s.paths:
		c.ValidArgsFunction = noFiles
	}
	return c
}

// pkgCompletion completes positional arguments with package names, leaving
// out the ones already on the command line.
// Entries are "name" or "name\tdescription"; cobra shows the description
// in shells that support it (zsh, fish).
func pkgCompletion(list func() ([]string, error)) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		all, err := list()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		var out []string
		for _, p := range all {
			name, _, _ := strings.Cut(p, "\t")
			if strings.HasPrefix(name, toComplete) && !slices.Contains(args, name) {
				out = append(out, p)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// needsRoot reports whether the command must run as root. A dry-run of
// xbps-install does not touch the system, unless it also syncs the
// index (-S), which writes to /var/db/xbps.
func needsRoot(bin string, o opts, base []string) bool {
	if bin != "xbps-install" || !o.sim {
		return true
	}
	for _, a := range base {
		if strings.HasPrefix(a, "-S") {
			return true
		}
	}
	return false
}

// noFiles stops the shell from falling back to file names.
func noFiles(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}
