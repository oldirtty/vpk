package xbps

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// repodataGlob finds the repository indexes xbps keeps on disk. It is a
// variable so tests can point it elsewhere.
var repodataGlob = "/var/db/xbps/*/*-repodata"

// Installed returns completion entries ("name\tdescription") for the
// installed packages. It is fast, so it is not cached.
func Installed() ([]string, error) {
	return query("-l")
}

// Manual returns the packages that were installed explicitly.
func Manual() ([]string, error) {
	return query("-m")
}

// Held returns the packages on hold.
func Held() ([]string, error) {
	return query("-H")
}

// Repolocked returns the packages locked to their repository.
func Repolocked() ([]string, error) {
	return query("--list-repolock-pkgs")
}

// Available returns completion entries for every package in the
// repositories. The result is cached under the user's cache dir and is
// discarded when any repodata file is newer than the cache.
func Available() ([]string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return fetchAvailable()
	}
	cache := filepath.Join(dir, "vpk", "available")
	repodata, _ := filepath.Glob(repodataGlob)

	if fresh(cache, repodata) {
		if pkgs := load(cache); pkgs != nil {
			return pkgs, nil
		}
	}

	pkgs, err := fetchAvailable()
	if err != nil {
		return nil, err
	}
	store(cache, pkgs) // best effort
	return pkgs, nil
}

func fetchAvailable() ([]string, error) {
	pkgs, err := query("-Rs", "")
	if err != nil {
		return nil, err
	}
	return dedupe(pkgs), nil
}

func query(args ...string) ([]string, error) {
	out, err := exec.Command("xbps-query", args...).Output()
	if len(out) == 0 && err != nil {
		return nil, err
	}
	return parse(out), nil
}

// parse reads xbps-query output, one package per line:
//
//	[-] htop-3.5.3_1 Interactive process viewer     (-Rs)
//	ii htop-3.5.3_1  Interactive process viewer     (-l)
//	htop-3.5.3_1                                    (-H, -m, -O)
//
// and returns "htop\tInteractive process viewer" (or just "htop").
// Control characters are dropped: descriptions come from the repositories
// and end up on the user's terminal.
func parse(out []byte) []string {
	var pkgs []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) && !unicode.IsSpace(r) {
				return -1
			}
			return r
		}, line)
		f := strings.Fields(line)
		if len(f) > 1 && isState(f[0]) {
			f = f[1:]
		}
		if len(f) == 0 {
			continue
		}
		i := strings.LastIndex(f[0], "-") // name-version
		if i <= 0 {
			continue
		}
		entry := f[0][:i]
		if desc := strings.Join(f[1:], " "); desc != "" {
			entry += "\t" + desc
		}
		pkgs = append(pkgs, entry)
	}
	return pkgs
}

// dedupe sorts the entries and keeps the first one of each package name
// (the same package can be listed by more than one repository).
func dedupe(pkgs []string) []string {
	slices.Sort(pkgs)
	var out []string
	last := ""
	for _, p := range pkgs {
		name, _, _ := strings.Cut(p, "\t")
		if len(out) > 0 && name == last {
			continue
		}
		out = append(out, p)
		last = name
	}
	return out
}

// fresh reports whether cache exists and is newer than every repodata file.
// Without repodata files to compare against, the cache is never trusted.
func fresh(cache string, repodata []string) bool {
	info, err := os.Stat(cache)
	if err != nil || len(repodata) == 0 {
		return false
	}
	for _, r := range repodata {
		st, err := os.Stat(r)
		if err != nil || st.ModTime().After(info.ModTime()) {
			return false
		}
	}
	return true
}

// load reads the cache, or returns nil if it is missing or empty.
func load(cache string) []string {
	b, err := os.ReadFile(cache)
	if err != nil || len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// store writes the cache atomically and only readable by the user.
func store(cache string, pkgs []string) {
	dir := filepath.Dir(cache)
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "available-*")
	if err != nil {
		return
	}
	_, err = tmp.WriteString(strings.Join(pkgs, "\n") + "\n")
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp.Name())
		return
	}
	if os.Rename(tmp.Name(), cache) != nil {
		os.Remove(tmp.Name())
	}
}

// isState reports whether tok is a state column of xbps-query: ii, uu, hr, ??
// (-l) or [-], [*] (-Rs).
func isState(tok string) bool {
	switch tok {
	case "ii", "uu", "hr", "??":
		return true
	}
	return strings.HasPrefix(tok, "[")
}
