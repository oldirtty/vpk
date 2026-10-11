package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/oldirtty/vpk/internal/xbps"
)

func TestNoDuplicateNames(t *testing.T) {
	// Names and aliases must be unique among siblings: the top-level
	// commands (parents included) or the subcommands of one parent.
	seen := map[string]string{}
	for name, p := range parents {
		for _, n := range append([]string{name}, p.aliases...) {
			if prev, ok := seen[n]; ok {
				t.Errorf("%q used by both %q and %q", n, prev, name)
			}
			seen[n] = name
		}
	}
	for _, s := range specs {
		for _, n := range append([]string{s.name}, s.aliases...) {
			key := s.parent + "/" + n
			if s.parent == "" {
				key = n
			}
			if prev, ok := seen[key]; ok {
				t.Errorf("%q used by both %q and %q", key, prev, s.name)
			}
			seen[key] = s.name
		}
	}
}

func TestParentsExist(t *testing.T) {
	for _, s := range specs {
		if _, ok := parents[s.parent]; s.parent != "" && !ok {
			t.Errorf("command %q has an unknown parent %q", s.name, s.parent)
		}
	}
}

// capture runs the real command tree with xbps stubbed out and returns the
// command that would have been executed.
func capture(t *testing.T, args ...string) (xbps.Cmd, bool, error) {
	t.Helper()
	var got xbps.Cmd
	called := false
	run = func(c xbps.Cmd) int { got, called = c, true; return 0 }
	defer func() { run = xbps.Run }()

	var o opts
	code := 0
	root := newRoot(&o, &code)
	root.SetArgs(args)
	err := root.Execute()
	return got, called, err
}

// vpk returns the command line that would have been executed.
func vpk(t *testing.T, args ...string) (string, bool) {
	t.Helper()
	got, called, err := capture(t, args...)
	if err != nil {
		return "error: " + err.Error(), called
	}
	return strings.Join(append([]string{got.Bin}, got.Args...), " "), called
}

func TestCommands(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		// add and its aliases
		{[]string{"add", "firefox"}, "xbps-install -- firefox"},
		{[]string{"install", "firefox"}, "xbps-install -- firefox"},
		{[]string{"in", "firefox"}, "xbps-install -- firefox"},
		{[]string{"i", "firefox"}, "xbps-install -- firefox"},
		{[]string{"add", "-y", "git", "curl"}, "xbps-install -y -- git curl"},
		{[]string{"add", "-U", "vim"}, "xbps-install -S -- vim"},
		{[]string{"add", "-s", "vim"}, "xbps-install -n -- vim"},
		{[]string{"add", "--dry-run", "vim"}, "xbps-install -n -- vim"},
		{[]string{"add", "-n", "vim"}, "xbps-install -n -- vim"},
		{[]string{"add", "-f", "vim"}, "xbps-install -f -- vim"},
		{[]string{"add", "-u", "vim"}, "xbps-install -u -- vim"},
		{[]string{"add", "-A", "vim"}, "xbps-install -A -- vim"},
		{[]string{"add", "--automatic", "vim"}, "xbps-install -A -- vim"},
		{[]string{"add", "-D", "vim"}, "xbps-install -D -- vim"},
		{[]string{"add", "--download-only", "vim"}, "xbps-install -D -- vim"},
		{[]string{"add", "-X", "https://r.example/a", "-X", "https://r.example/b", "vim"},
			"xbps-install -R https://r.example/a -R https://r.example/b -- vim"},
		{[]string{"add", "-p", "/mnt", "-v", "vim"}, "xbps-install -v -r /mnt -- vim"},
		{[]string{"add", "--", "-weird"}, "xbps-install -- -weird"},

		// del and its aliases
		{[]string{"del", "foo"}, "xbps-remove -R -- foo"},
		{[]string{"remove", "foo"}, "xbps-remove -R -- foo"},
		{[]string{"rm", "foo"}, "xbps-remove -R -- foo"},
		{[]string{"del", "-s", "foo"}, "xbps-remove -R -n -- foo"},
		{[]string{"del", "--force-revdeps", "foo"}, "xbps-remove -R -F -- foo"},

		// update / upgrade
		{[]string{"update"}, "xbps-install -S"},
		{[]string{"refresh"}, "xbps-install -S"},
		{[]string{"update", "-y"}, "xbps-install -S -y"},
		{[]string{"upgrade"}, "xbps-install -Su"},
		{[]string{"up", "-s"}, "xbps-install -Su -n"},
		{[]string{"upgrade", "-D"}, "xbps-install -Su -D"},

		// search
		{[]string{"search", "vim"}, "xbps-query -Rs vim"},
		{[]string{"s", "vim"}, "xbps-query -Rs vim"},
		{[]string{"search", "--regex", "^vim"}, "xbps-query --regex -Rs ^vim"},
		{[]string{"search", "--property", "pkgver", "vim"}, "xbps-query -p pkgver -Rs vim"},
		{[]string{"search", "--regex", "--property", "pkgver", "^vim"}, "xbps-query --regex -p pkgver -Rs ^vim"},

		// clean, cache clean, autoremove
		{[]string{"clean"}, "xbps-remove -O"},
		{[]string{"autoclean"}, "xbps-remove -O"},
		{[]string{"clean", "--all"}, "xbps-remove -O -O"},
		{[]string{"cache", "clean"}, "xbps-remove -O"},
		{[]string{"cache", "clean", "--all"}, "xbps-remove -O -O"},
		{[]string{"autoremove"}, "xbps-remove -o"},
		{[]string{"orphans", "-s"}, "xbps-remove -o -n"},
		{[]string{"prune", "-y"}, "xbps-remove -o -y"},

		// list
		{[]string{"list"}, "xbps-query -l"},
		{[]string{"ls"}, "xbps-query -l"},
		{[]string{"list", "-I"}, "xbps-query -l"},
		{[]string{"list", "-a"}, "xbps-query -Rs "},
		{[]string{"list", "--manual"}, "xbps-query -m"},
		{[]string{"list", "-o"}, "xbps-query -O"},
		{[]string{"list", "-H"}, "xbps-query -H"},
		{[]string{"list", "-u"}, "xbps-install -un"},
		{[]string{"list", "--upgradable"}, "xbps-install -un"},

		// info, deps, rdeps, files, owns: repositories by default for info
		// and deps, installed packages for the others
		{[]string{"info", "htop"}, "xbps-query -RS htop"},
		{[]string{"show", "htop"}, "xbps-query -RS htop"},
		{[]string{"info", "-I", "htop"}, "xbps-query -S htop"},
		{[]string{"info", "--property", "pkgver", "htop"}, "xbps-query -p pkgver -RS htop"},
		{[]string{"info", "-I", "--property", "state", "htop"}, "xbps-query -p state -S htop"},
		{[]string{"deps", "htop"}, "xbps-query -Rx htop"},
		{[]string{"depends", "htop"}, "xbps-query -Rx htop"},
		{[]string{"deps", "-I", "htop"}, "xbps-query -x htop"},
		{[]string{"deps", "--tree", "htop"}, "xbps-query --fulldeptree -Rx htop"},
		{[]string{"rdeps", "htop"}, "xbps-query -X htop"},
		{[]string{"why", "htop"}, "xbps-query -X htop"},
		{[]string{"needed-by", "htop"}, "xbps-query -X htop"},
		{[]string{"rdeps", "-a", "htop"}, "xbps-query -RX htop"},
		{[]string{"files", "htop"}, "xbps-query -f htop"},
		{[]string{"contents", "htop"}, "xbps-query -f htop"},
		{[]string{"files", "-a", "htop"}, "xbps-query -Rf htop"},
		{[]string{"owns", "/usr/bin/htop"}, "xbps-query -o /usr/bin/htop"},
		{[]string{"provides", "/usr/bin/htop"}, "xbps-query -o /usr/bin/htop"},
		{[]string{"whatprovides", "/usr/bin/htop"}, "xbps-query -o /usr/bin/htop"},

		// reinstall
		{[]string{"reinstall", "htop"}, "xbps-install -f -- htop"},
		{[]string{"fix", "htop"}, "xbps-install -f -- htop"},
		{[]string{"reinstall", "-s", "htop"}, "xbps-install -f -n -- htop"},

		// hold / unhold and mark
		{[]string{"hold", "htop", "curl"}, "xbps-pkgdb -m hold -- htop curl"},
		{[]string{"pin", "htop"}, "xbps-pkgdb -m hold -- htop"},
		{[]string{"unhold", "htop"}, "xbps-pkgdb -m unhold -- htop"},
		{[]string{"unpin", "htop"}, "xbps-pkgdb -m unhold -- htop"},
		{[]string{"mark", "auto", "htop"}, "xbps-pkgdb -m auto -- htop"},
		{[]string{"mark", "manual", "htop", "curl"}, "xbps-pkgdb -m manual -- htop curl"},
		{[]string{"mark", "repolock", "htop"}, "xbps-pkgdb -m repolock -- htop"},
		{[]string{"mark", "repounlock", "htop"}, "xbps-pkgdb -m repounlock -- htop"},

		// verify
		{[]string{"verify"}, "xbps-pkgdb -a"},
		{[]string{"check"}, "xbps-pkgdb -a"},
		{[]string{"verify", "htop"}, "xbps-pkgdb -- htop"},

		// reconfigure
		{[]string{"reconfigure", "htop"}, "xbps-reconfigure -- htop"},
		{[]string{"configure", "-f", "htop"}, "xbps-reconfigure -f -- htop"},
		{[]string{"reconfigure", "-a"}, "xbps-reconfigure -a"},
		{[]string{"reconfigure", "--force", "--all"}, "xbps-reconfigure -f -a"},
		{[]string{"reconfigure", "-a", "-i", "linux", "-i", "dkms"}, "xbps-reconfigure -a -i linux -i dkms"},
		{[]string{"reconfigure", "-v", "htop"}, "xbps-reconfigure -v -- htop"},
		{[]string{"reconfigure", "-p", "/mnt", "htop"}, "xbps-reconfigure -r /mnt -- htop"},

		// repo and alternatives
		{[]string{"repo", "list"}, "xbps-query -L"},
		{[]string{"repo", "ls", "-v"}, "xbps-query -v -L"},
		{[]string{"alternatives", "list"}, "xbps-alternatives -l"},
		{[]string{"alt", "ls", "vim"}, "xbps-alternatives -l vim"},
		{[]string{"alternatives", "list", "-g", "editor"}, "xbps-alternatives -g editor -l"},
		{[]string{"alternatives", "set", "neovim"}, "xbps-alternatives -s neovim"},
		{[]string{"alternatives", "set", "-g", "editor", "neovim"}, "xbps-alternatives -g editor -s neovim"},
	}
	for _, tt := range tests {
		got, called := vpk(t, tt.args...)
		if !called || got != tt.want {
			t.Errorf("vpk %s:\n got  %q\n want %q", strings.Join(tt.args, " "), got, tt.want)
		}
	}
}

func TestRejects(t *testing.T) {
	for _, args := range [][]string{
		{"add"},                           // needs at least one package
		{"del"},                           // same
		{"update", "vim"},                 // takes no arguments
		{"search"},                        // needs a term
		{"update", "-U"},                  // flag not supported here
		{"del", "-X", "x", "a"},           // -X is install-only
		{"list", "-a", "-m"},              // list modes are mutually exclusive
		{"list", "-u", "-H"},              // same
		{"list", "htop"},                  // takes no arguments
		{"info"},                          // needs a package
		{"info", "-a", "htop"},            // info has -I, not -a
		{"rdeps", "-I", "htop"},           // rdeps has -a, not -I
		{"owns"},                          // needs a file
		{"hold"},                          // needs a package
		{"unhold"},                        // same
		{"reinstall"},                     // same
		{"verify", "-y"},                  // no flags
		{"mark", "auto"},                  // needs a package
		{"mark", "bogus", "htop"},         // unknown mode
		{"reconfigure"},                   // needs packages or --all
		{"reconfigure", "-a", "vim"},      // --all takes no packages
		{"reconfigure", "-i", "x", "vim"}, // --ignore needs --all
		{"reconfigure", "-y", "vim"},      // no -y here
		{"alternatives", "set"},           // needs a package
		{"alternatives", "list", "a", "b"},
		{"repo", "list", "x"},
		{"search", "-r", "x"}, // --regex has no short form
	} {
		if got, called := vpk(t, args...); called {
			t.Errorf("vpk %s should be rejected, ran %q", strings.Join(args, " "), got)
		}
	}
}

func TestRoot(t *testing.T) {
	tests := []struct {
		args []string
		want bool // runs through doas/sudo
	}{
		{[]string{"add", "vim"}, true},
		{[]string{"add", "-s", "vim"}, false},      // dry-run does not touch the system
		{[]string{"add", "-U", "-s", "vim"}, true}, // ...unless it syncs the index
		{[]string{"add", "-D", "vim"}, true},
		{[]string{"del", "-s", "vim"}, true},
		{[]string{"update"}, true},
		{[]string{"upgrade"}, true},
		{[]string{"upgrade", "-s"}, true}, // -Su syncs
		{[]string{"autoremove"}, true},
		{[]string{"reinstall", "-s", "vim"}, false},
		{[]string{"hold", "vim"}, true},
		{[]string{"unhold", "vim"}, true},
		{[]string{"mark", "auto", "vim"}, true},
		{[]string{"verify"}, true},
		{[]string{"reconfigure", "-a"}, true},
		{[]string{"alternatives", "set", "vi"}, true},
		{[]string{"search", "vim"}, false},
		{[]string{"list"}, false},
		{[]string{"list", "-u"}, false},
		{[]string{"info", "vim"}, false},
		{[]string{"deps", "vim"}, false},
		{[]string{"rdeps", "vim"}, false},
		{[]string{"files", "vim"}, false},
		{[]string{"owns", "/usr/bin/vim"}, false},
		{[]string{"repo", "list"}, false},
		{[]string{"alternatives", "list"}, false},
	}
	for _, tt := range tests {
		got, called, err := capture(t, tt.args...)
		if err != nil || !called {
			t.Errorf("vpk %s: not run (err=%v)", strings.Join(tt.args, " "), err)
			continue
		}
		if got.Root != tt.want {
			t.Errorf("vpk %s: Root = %v, want %v", strings.Join(tt.args, " "), got.Root, tt.want)
		}
	}
}

// xbps updates itself in a separate transaction, so a real upgrade runs
// again when more is pending. Nothing else does.
func TestAgain(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"upgrade"}, true},
		{[]string{"upgrade", "-y"}, true},
		{[]string{"upgrade", "-s"}, false},
		{[]string{"upgrade", "-D"}, false},
		{[]string{"upgrade", "-p", "/mnt"}, false},
		{[]string{"update"}, false},
		{[]string{"add", "vim"}, false},
	}
	for _, tt := range tests {
		got, called, err := capture(t, tt.args...)
		if err != nil || !called {
			t.Errorf("vpk %s: not run (err=%v)", strings.Join(tt.args, " "), err)
			continue
		}
		if got.Again != tt.want {
			t.Errorf("vpk %s: Again = %v, want %v", strings.Join(tt.args, " "), got.Again, tt.want)
		}
	}
}

func TestVersion(t *testing.T) {
	for _, flag := range []string{"--version", "-V"} {
		var o opts
		code := 0
		root := newRoot(&o, &code)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{flag})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if got, want := out.String(), "vpk "+version+"\n"; got != want {
			t.Errorf("vpk %s printed %q, want %q", flag, got, want)
		}
	}
}

// help runs vpk with args and returns what it printed.
func help(t *testing.T, args ...string) string {
	t.Helper()
	var o opts
	code := 0
	root := newRoot(&o, &code)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// -s/--simulate is the documented flag; --dry-run and -n still work but
// stay out of the way.
func TestDryRunIsHidden(t *testing.T) {
	for _, cmd := range []string{"add", "del", "upgrade", "reinstall", "autoremove", "clean"} {
		out := help(t, cmd, "-h")
		if !strings.Contains(out, "--simulate") {
			t.Errorf("vpk %s -h does not show --simulate", cmd)
		}
		if strings.Contains(out, "  --dry-run") || strings.Contains(out, "-n, ") {
			t.Errorf("vpk %s -h shows --dry-run:\n%s", cmd, out)
		}
	}
}

func TestParentsAreListed(t *testing.T) {
	out := help(t, "-h")
	for name, p := range parents {
		if !strings.Contains(out, name) || !strings.Contains(out, p.short) {
			t.Errorf("vpk -h does not list %q", name)
		}
	}
}

// "vpk mark bogus" is a typo: an error, not a help page with exit code 0.
func TestParentRejectsUnknownSubcommand(t *testing.T) {
	_, called, err := capture(t, "mark", "bogus", "htop")
	if err == nil || called || !strings.Contains(err.Error(), `unknown command "bogus"`) {
		t.Errorf("err = %v, called = %v; want an unknown command error", err, called)
	}
}

func TestParentAloneShowsHelp(t *testing.T) {
	for _, parent := range []string{"mark", "repo", "alternatives", "alt", "cache"} {
		_, called, err := capture(t, parent)
		if err != nil || called {
			t.Errorf("vpk %s: err = %v, called = %v; want help and no xbps call", parent, err, called)
		}
	}
	if out := help(t, "mark"); !strings.Contains(out, "repolock") {
		t.Errorf("vpk mark does not list its subcommands:\n%s", out)
	}
}
