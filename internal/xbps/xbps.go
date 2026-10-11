package xbps

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

// Cmd is one xbps invocation, before privilege escalation.
type Cmd struct {
	Bin  string
	Args []string
	Root bool // needs root

	// Again asks for a second run when updates are still pending after the
	// first one: xbps updates itself in a separate transaction.
	Again bool
}

// geteuid is a variable so tests can pretend to be a normal user.
var geteuid = os.Geteuid

// Run executes c and returns its exit code (128+n if it died from signal n).
func Run(c Cmd) int {
	// Ctrl-C reaches vpk and xbps together. Catch it here so vpk keeps waiting
	// while xbps cleans up. (signal.Ignore would be inherited by xbps.)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	code := run(c)
	if c.Again && code == 0 && pending() {
		fmt.Fprintln(os.Stderr, "vpk: updates are still pending, running the upgrade again")
		code = run(c)
	}
	return code
}

func run(c Cmd) int {
	name, args := c.Bin, c.Args
	if c.Root && geteuid() != 0 {
		esc, err := escalator()
		if err != nil {
			fmt.Fprintln(os.Stderr, "vpk:", err)
			return 1
		}
		name, args = esc, append([]string{c.Bin}, c.Args...)
	}

	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return exitErr.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "vpk:", err)
		return 1
	}
	return 0
}

// pending reports whether a dry-run upgrade still has something to do.
func pending() bool {
	out, err := exec.Command("xbps-install", "-un").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func escalator() (string, error) {
	for _, name := range []string{"doas", "sudo", "run0"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("root required, but none of doas, sudo, run0 was found")
}
