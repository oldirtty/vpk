package cli

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/oldirtty/vpk/internal/xbps"
)

func TestPkgCompletion(t *testing.T) {
	list := func() ([]string, error) {
		return []string{"firefox\tbrowser", "firefox-esr", "htop\tviewer"}, nil
	}
	got, dir := pkgCompletion(list)(nil, nil, "fir")
	want := []string{"firefox\tbrowser", "firefox-esr"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", dir)
	}
}

// vpk del a b <TAB> must not offer a and b again.
func TestPkgCompletionSkipsWhatIsAlreadyTyped(t *testing.T) {
	list := func() ([]string, error) {
		return []string{"firefox\tbrowser", "firefox-esr", "htop\tviewer"}, nil
	}
	got, _ := pkgCompletion(list)(nil, []string{"firefox"}, "fir")
	want := []string{"firefox-esr"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPkgCompletionError(t *testing.T) {
	list := func() ([]string, error) { return nil, errors.New("boom") }
	got, dir := pkgCompletion(list)(nil, nil, "")
	if got != nil || dir != cobra.ShellCompDirectiveError {
		t.Errorf("got %q, %v; want nil, Error", got, dir)
	}
}

// Where each command gets its <TAB> candidates from.
func TestCompletionSources(t *testing.T) {
	ptr := func(f func() ([]string, error)) uintptr {
		if f == nil {
			return 0
		}
		return reflect.ValueOf(f).Pointer()
	}
	want := map[string]func() ([]string, error){
		"add":               xbps.Available,
		"info":              xbps.Available,
		"deps":              xbps.Available,
		"del":               xbps.Installed,
		"files":             xbps.Installed,
		"rdeps":             xbps.Installed,
		"reinstall":         xbps.Installed,
		"hold":              xbps.Installed,
		"verify":            xbps.Installed,
		"reconfigure":       xbps.Installed,
		"unhold":            xbps.Held,
		"mark auto":         xbps.Manual,
		"mark manual":       xbps.Installed,
		"mark repolock":     xbps.Installed,
		"mark repounlock":   xbps.Repolocked,
		"alternatives list": xbps.Installed,
		"alternatives set":  xbps.Installed,
		"autoremove":        nil,
		"list":              nil,
		"owns":              nil, // a file path, completed by the shell
		"search":            nil,
		"update":            nil,
		"upgrade":           nil,
		"clean":             nil,
		"cache clean":       nil,
		"repo list":         nil,
	}
	seen := map[string]bool{}
	for _, s := range specs {
		name := strings.TrimSpace(s.parent + " " + s.name)
		w, ok := want[name]
		if !ok {
			t.Errorf("command %q has no entry in this test", name)
			continue
		}
		seen[name] = true
		if ptr(s.pkgs) != ptr(w) {
			t.Errorf("command %q completes from the wrong source", name)
		}
		if s.paths != (name == "owns") {
			t.Errorf("command %q: paths = %v", name, s.paths)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("command %q is expected but missing from specs", name)
		}
	}
}

// complete asks the real command tree what <TAB> would offer.
func complete(t *testing.T, args ...string) string {
	t.Helper()
	var o opts
	code := 0
	root := newRoot(&o, &code)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"__complete"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestPropertyCompletion(t *testing.T) {
	for _, cmd := range []string{"info", "search"} {
		out := complete(t, cmd, "--property", "")
		for _, want := range []string{"pkgver", "short_desc", "run_depends", "state"} {
			if !strings.Contains(out, want+"\n") {
				t.Errorf("vpk %s --property <TAB> does not offer %q:\n%s", cmd, want, out)
			}
		}
	}
}

func TestSubcommandCompletion(t *testing.T) {
	out := complete(t, "mark", "")
	for _, want := range []string{"auto", "manual", "repolock", "repounlock"} {
		if !strings.Contains(out, want+"\t") {
			t.Errorf("vpk mark <TAB> does not offer %q:\n%s", want, out)
		}
	}
	out = complete(t, "alt", "")
	if !strings.Contains(out, "list\t") || !strings.Contains(out, "set\t") {
		t.Errorf("vpk alt <TAB> does not offer list and set:\n%s", out)
	}
}

func TestFlagCompletion(t *testing.T) {
	out := complete(t, "add", "--")
	for _, want := range []string{"--automatic", "--download-only", "--simulate", "--update-cache"} {
		if !strings.Contains(out, want+"\t") {
			t.Errorf("vpk add -- <TAB> does not offer %q:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "--dry-run") {
			t.Errorf("hidden --dry-run is offered:\n%s", out)
		}
	}
}

func TestPropertyCompletionFiltersByPrefix(t *testing.T) {
	out := complete(t, "info", "--property", "sh")
	for _, want := range []string{"short_desc\n", "shlib-provides\n", "shlib-requires\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "pkgver") || strings.Contains(out, "state") {
		t.Errorf("offers properties that do not start with sh:\n%s", out)
	}
}
