package xbps

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	out := "[-] htop-3.5.3_1 Interactive process viewer\n" + // -Rs
		"[*] foo-1.0_1\n" +
		"ii python3-MarkupSafe-3.0.3_2    Safely add untrusted strings to HTML\n" + // -l
		"uu bar-2_1 Not configured yet\n" +
		"hr baz-3_1 Half removed\n" +
		"?? qux-4_1 Unknown state\n" +
		"gcc-14.2.0_1\n" + // -H, -m, -O: pkgver only
		"garbage\n\n"
	want := []string{
		"htop\tInteractive process viewer",
		"foo",
		"python3-MarkupSafe\tSafely add untrusted strings to HTML",
		"bar\tNot configured yet",
		"baz\tHalf removed",
		"qux\tUnknown state",
		"gcc",
	}
	if got := parse([]byte(out)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Descriptions come from the repositories and are printed on the terminal.
func TestParseDropsControlCharacters(t *testing.T) {
	out := "[-] evil-1_1 hello \x1b[31mred\x1b[0m world\x07\n" +
		"[-] wipe\x1b-2_1 \x1b]0;title\x07x\n"
	got := parse([]byte(out))
	want := []string{
		"evil\thello [31mred[0m world",
		"wipe\t]0;titlex",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	for _, p := range got {
		if strings.ContainsAny(p, "\x1b\x07") {
			t.Errorf("%q still has control characters", p)
		}
	}
}

func TestDedupe(t *testing.T) {
	in := []string{
		"zz\tlast",
		"firefox-esr\tESR",
		"firefox\tfrom the main repo",
		"aa\tfirst",
		"firefox\tfrom another repo",
	}
	want := []string{
		"aa\tfirst",
		"firefox\tfrom another repo",
		"firefox-esr\tESR",
		"zz\tlast",
	}
	if got := dedupe(in); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFresh(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	repo := filepath.Join(dir, "x-repodata")
	now := time.Now()

	touch := func(path string, mtime time.Time) {
		t.Helper()
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}

	if fresh(cache, []string{repo}) {
		t.Error("missing cache must not be fresh")
	}
	touch(cache, now)
	if fresh(cache, nil) {
		t.Error("no repodata to compare: must not be fresh")
	}
	touch(repo, now.Add(-time.Hour))
	if !fresh(cache, []string{repo}) {
		t.Error("cache newer than repodata must be fresh")
	}
	touch(repo, now.Add(time.Hour))
	if fresh(cache, []string{repo}) {
		t.Error("repodata newer than cache must invalidate it")
	}
}

// Available end to end: it asks xbps-query once, serves the next calls from
// the cache, and asks again after a repository update.
func TestAvailableCache(t *testing.T) {
	dir := t.TempDir()
	log := sandbox(t, dir)
	fakeBin(t, dir, "xbps-query", `echo x >> "$LOG"
printf '%s\n' '[-] zz-1_1 last' '[*] firefox-130_1 Mozilla Firefox' \
  '[-] firefox-130_1 Mozilla Firefox' '[-] aa-2_1 first'`)

	cacheHome := filepath.Join(dir, "cache-home")
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	repoDir := filepath.Join(dir, "repos", "https___example_current")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	repodata := filepath.Join(repoDir, "x86_64-repodata")
	if err := os.WriteFile(repodata, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(repodata, old, old); err != nil {
		t.Fatal(err)
	}
	saved := repodataGlob
	repodataGlob = filepath.Join(dir, "repos", "*", "*-repodata")
	t.Cleanup(func() { repodataGlob = saved })

	want := []string{"aa\tfirst", "firefox\tMozilla Firefox", "zz\tlast"}
	calls := func() int { return strings.Count(readLog(t, log), "x") }

	got, err := Available()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("first call: got %q, %v; want %q", got, err, want)
	}
	if n := calls(); n != 1 {
		t.Fatalf("first call ran xbps-query %d times, want 1", n)
	}

	cache := filepath.Join(cacheHome, "vpk", "available")
	if st, err := os.Stat(cache); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("cache file: %v, %v; want mode 0600", st, err)
	}
	if st, err := os.Stat(filepath.Dir(cache)); err != nil || st.Mode().Perm() != 0o700 {
		t.Errorf("cache dir: %v, %v; want mode 0700", st, err)
	}

	got, err = Available()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("second call: got %q, %v; want %q", got, err, want)
	}
	if n := calls(); n != 1 {
		t.Errorf("second call must come from the cache, xbps-query ran %d times", n)
	}

	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(repodata, future, future); err != nil {
		t.Fatal(err)
	}
	if _, err := Available(); err != nil {
		t.Fatal(err)
	}
	if n := calls(); n != 2 {
		t.Errorf("after a repository update xbps-query ran %d times, want 2", n)
	}
}

func TestListings(t *testing.T) {
	dir := t.TempDir()
	log := sandbox(t, dir)
	fakeBin(t, dir, "xbps-query", `echo "$*" >> "$LOG"; echo gcc-14.2.0_1`)

	for name, f := range map[string]func() ([]string, error){
		"-l":                   Installed,
		"-m":                   Manual,
		"-H":                   Held,
		"--list-repolock-pkgs": Repolocked,
	} {
		got, err := f()
		if err != nil || !reflect.DeepEqual(got, []string{"gcc"}) {
			t.Errorf("xbps-query %s: got %q, %v", name, got, err)
		}
		if !strings.Contains(readLog(t, log), name+"\n") {
			t.Errorf("xbps-query %s was never called", name)
		}
	}
}
