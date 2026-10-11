# vpk: dev tasks. Run `just` to list them.

# "dev" when there is no git history yet
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
module := `go list -m`
ldflags := "-X " + module + "/internal/cli.version=" + version

# list the recipes
default:
  @just --list

# format, vet and test
check: fmt
  go vet ./...
  go test ./...

# format every Go file
fmt:
  gofmt -w .

# build ./vpk
build:
  go build -ldflags "{{ldflags}}" -o vpk .

# A root-owned copy, not a symlink: a symlink to ~/go/bin would let anything
# running as you replace the binary that sudo runs.

# build and copy to /usr/local/bin
install: build
  sudo install -m 755 vpk /usr/local/bin/vpk

# regenerate the zsh completion
completion:
  vpk completion zsh > ~/.cache/zsh/completions/_vpk
