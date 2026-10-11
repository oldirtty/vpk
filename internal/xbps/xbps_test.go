package xbps

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeBin writes an executable shell script called name into dir.
func fakeBin(t *testing.T, dir, name, body string) {
	t.Helper()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// sandbox makes dir the whole PATH, so only the fake binaries exist, and
// pretends to be a normal user. The fakes append to the file it returns.
func sandbox(t *testing.T, dir string) (log string) {
	t.Helper()
	log = filepath.Join(dir, "log")
	t.Setenv("PATH", dir)
	t.Setenv("LOG", log)
	old := geteuid
	geteuid = func() int { return 1000 }
	t.Cleanup(func() { geteuid = old })
	return log
}

func readLog(t *testing.T, log string) string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunExitCode(t *testing.T) {
	dir := t.TempDir()
	sandbox(t, dir)
	fakeBin(t, dir, "xbps-install", "exit 7")
	if got := Run(Cmd{Bin: "xbps-install"}); got != 7 {
		t.Errorf("exit code = %d, want 7", got)
	}
}

func TestRunDeathBySignal(t *testing.T) {
	dir := t.TempDir()
	sandbox(t, dir)
	fakeBin(t, dir, "xbps-install", "kill -9 $$")
	if got := Run(Cmd{Bin: "xbps-install"}); got != 128+9 {
		t.Errorf("exit code = %d, want 137", got)
	}
}

func TestRunMissingBinary(t *testing.T) {
	sandbox(t, t.TempDir())
	if got := Run(Cmd{Bin: "xbps-install"}); got != 1 {
		t.Errorf("exit code = %d, want 1", got)
	}
}

func TestRunEscalation(t *testing.T) {
	tests := []struct {
		name    string
		escalat []string // fake escalators on PATH
		euid    int
		root    bool
		want    string // what the log must contain
		code    int
	}{
		{"doas first", []string{"doas", "sudo", "run0"}, 1000, true, "doas\nxbps-install -S\n", 0},
		{"sudo when no doas", []string{"sudo", "run0"}, 1000, true, "sudo\nxbps-install -S\n", 0},
		{"run0 as last resort", []string{"run0"}, 1000, true, "run0\nxbps-install -S\n", 0},
		{"none available", nil, 1000, true, "", 1},
		{"already root", []string{"doas"}, 0, true, "xbps-install -S\n", 0},
		{"root not needed", []string{"doas"}, 1000, false, "xbps-install -S\n", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			log := sandbox(t, dir)
			geteuid = func() int { return tt.euid }
			fakeBin(t, dir, "xbps-install", `echo "xbps-install $*" >> "$LOG"`)
			for _, e := range tt.escalat {
				fakeBin(t, dir, e, `echo `+e+` >> "$LOG"; exec "$@"`)
			}
			code := Run(Cmd{Bin: "xbps-install", Args: []string{"-S"}, Root: tt.root})
			if code != tt.code {
				t.Errorf("exit code = %d, want %d", code, tt.code)
			}
			if got := readLog(t, log); got != tt.want {
				t.Errorf("log = %q, want %q", got, tt.want)
			}
		})
	}
}

// xbps updates itself in a separate transaction: if the dry-run still has
// work after the first run, Run goes again, once.
func TestRunAgain(t *testing.T) {
	fake := `
echo "$*" >> "$LOG"
if [ "$1" = "-un" ]; then
  n=0
  [ -f "$COUNT" ] && read n < "$COUNT"
  [ "$n" -eq 0 ] && echo "xbps-0.60.8_1 update x86_64 https://repo 1 2"
  echo $((n + 1)) > "$COUNT"
fi`

	t.Run("updates pending", func(t *testing.T) {
		dir := t.TempDir()
		log := sandbox(t, dir)
		t.Setenv("COUNT", filepath.Join(dir, "count"))
		fakeBin(t, dir, "xbps-install", fake)
		if code := Run(Cmd{Bin: "xbps-install", Args: []string{"-Su"}, Again: true}); code != 0 {
			t.Fatalf("exit code = %d", code)
		}
		if got, want := readLog(t, log), "-Su\n-un\n-Su\n"; got != want {
			t.Errorf("log = %q, want %q", got, want)
		}
	})

	t.Run("nothing left", func(t *testing.T) {
		dir := t.TempDir()
		log := sandbox(t, dir)
		count := filepath.Join(dir, "count")
		t.Setenv("COUNT", count)
		if err := os.WriteFile(count, []byte("1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		fakeBin(t, dir, "xbps-install", fake)
		Run(Cmd{Bin: "xbps-install", Args: []string{"-Su"}, Again: true})
		if got, want := readLog(t, log), "-Su\n-un\n"; got != want {
			t.Errorf("log = %q, want %q", got, want)
		}
	})

	t.Run("failed run is not repeated", func(t *testing.T) {
		dir := t.TempDir()
		log := sandbox(t, dir)
		fakeBin(t, dir, "xbps-install", `echo "$*" >> "$LOG"; exit 1`)
		if code := Run(Cmd{Bin: "xbps-install", Args: []string{"-Su"}, Again: true}); code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if got, want := readLog(t, log), "-Su\n"; got != want {
			t.Errorf("log = %q, want %q", got, want)
		}
	})

	t.Run("without Again", func(t *testing.T) {
		dir := t.TempDir()
		log := sandbox(t, dir)
		fakeBin(t, dir, "xbps-install", `echo "$*" >> "$LOG"`)
		Run(Cmd{Bin: "xbps-install", Args: []string{"-Su"}})
		if got, want := readLog(t, log), "-Su\n"; got != want {
			t.Errorf("log = %q, want %q", got, want)
		}
	})
}

// Ctrl-C goes to vpk and xbps together. vpk must survive it and report what
// xbps exited with, instead of dying and leaving xbps running on its own.
func TestRunSurvivesInterrupt(t *testing.T) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		syscall.Kill(os.Getpid(), syscall.SIGINT)
	}()
	code := Run(Cmd{Bin: "sh", Args: []string{"-c", "sleep 0.4; exit 3"}})
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

func TestEscalatorError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := escalator()
	if err == nil || !strings.Contains(err.Error(), "doas") {
		t.Errorf("err = %v, want a message naming doas", err)
	}
}
