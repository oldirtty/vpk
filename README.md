[![Void Linux](https://img.shields.io/badge/Void_Linux-%23458161.svg?style=plastic&logo=voidlinux&logoColor=white)](#)

# Void Package Keeper

An [apk](https://wiki.alpinelinux.org/wiki/Alpine_Package_Keeper)-style command line for [XBPS](https://github.com/void-linux/xbps), the Void Linux package manager.

```sh
vpk add firefox          # xbps-install firefox
vpk del firefox          # xbps-remove -R firefox
vpk update && vpk upgrade
vpk search '^vim'  --regex
vpk why libX11           # what needs it?
```

`vpk` is a thin wrapper. It does not reimplement anything: every command builds an `xbps-*` command line and runs it. What it adds is a friendlier interface:

- Short, consistent verbs (`add`, `del`, `update`, `upgrade`, `search`, `info`, `list`...).
- Aliases from apt, dnf and apk, so muscle memory keeps working (`install`, `remove`, `rm`, `show`, `autoremove`, `why`...).
- Tab completion for commands, flags, flag values and **package names**, with descriptions.
- It asks for root only when the xbps command needs it, through `doas`, `sudo` or `run0`.

> **Status:** early development. Developed on Void Linux, x86_64, glibc.

## Build from source

You need Go 1.22 or newer and `just` (optional).

```sh
sudo xbps-install -S go just
git clone https://github.com/oldirtty/vpk
cd vpk
just install        # builds and copies to /usr/local/bin/vpk
```

Without `just`:

```sh
go build -o vpk .
sudo install -m 755 vpk /usr/local/bin/vpk
```

Install a **copy** owned by root, not a symlink to a file you can write: whatever you run with `sudo` should not be replaceable by your own user.

You do not need `sudo vpk`. Run `vpk` as yourself: it escalates only the xbps command that needs it.

### Completion

```sh
vpk completion zsh  > ~/.zfunc/_vpk        # then: fpath+=(~/.zfunc) before compinit
vpk completion bash > ~/.local/share/bash-completion/completions/vpk
vpk completion fish > ~/.config/fish/completions/vpk.fish
```

Package names for `vpk add <TAB>` come from the repositories. The list is cached in `~/.cache/vpk` and refreshed when a repository index changes.

## Commands

| Command | Aliases | Runs |
|---|---|---|
| `add <pkg>...` | `install`, `in`, `i` | `xbps-install` |
| `del <pkg>...` | `remove`, `rm`, `erase`, `uninstall` | `xbps-remove -R` |
| `update` | `refresh`, `makecache`, `sync` | `xbps-install -S` |
| `upgrade` | `up`, `dist-upgrade`, `full-upgrade`, `dup` | `xbps-install -Su` |
| `search <term>` | `find`, `se`, `s` | `xbps-query -Rs` |
| `list` | `ls` | `xbps-query -l`, `-m`, `-O`, `-H`, `-Rs` |
| `info <pkg>` | `show` | `xbps-query -RS` |
| `files <pkg>` | `contents` | `xbps-query -f` |
| `owns <file>` | `provides`, `whatprovides` | `xbps-query -o` |
| `deps <pkg>` | `depends` | `xbps-query -Rx` |
| `rdeps <pkg>` | `why`, `needed-by` | `xbps-query -X` |
| `reinstall <pkg>...` | `fix` | `xbps-install -f` |
| `autoremove` | `orphans`, `prune` | `xbps-remove -o` |
| `clean` | `autoclean`, `cache clean` | `xbps-remove -O` |
| `hold <pkg>...` / `unhold <pkg>...` | `pin` / `unpin` | `xbps-pkgdb -m hold` / `unhold` |
| `mark auto\|manual\|repolock\|repounlock <pkg>...` | | `xbps-pkgdb -m` |
| `verify [pkg...]` | `check` | `xbps-pkgdb` |
| `reconfigure <pkg>... \| --all` | `configure` | `xbps-reconfigure` |
| `repo list` | `repo ls` | `xbps-query -L` |
| `alternatives list\|set` | `alt` | `xbps-alternatives` |

Run `vpk <command> -h` for the flags of each one. `vpk -V` prints the version.

### Flags

| Flag | Where | Meaning |
|---|---|---|
| `-s`, `--simulate` (also `-n`, `--dry-run`) | add, del, upgrade, reinstall, autoremove, clean | show what would happen, change nothing |
| `-y`, `--yes` | commands that change the system | assume yes to all questions |
| `-v`, `--verbose` | most commands | verbose output |
| `-p`, `--root <dir>` | most commands | use another system root |
| `-X`, `--repository <url>` | add, update, upgrade | extra repository, repeatable |
| `-U`, `--update-cache` | add | sync the index first |
| `-u`, `--upgrade` | add | upgrade the named packages if installed |
| `-f`, `--force` | add | force reinstall |
| `-A`, `--automatic` | add | mark as installed automatically |
| `-D`, `--download-only` | add, upgrade | download and check, nothing else |
| `--force-revdeps` | del | remove even if other packages depend on it |
| `--all` | clean | also remove packages no longer installed (`xbps-remove -O -O`) |
| `-I`, `--installed` / `-a`, `--available` | info, deps / rdeps, files, list | installed packages or repositories |
| `-m`, `-o`, `-H`, `-u` | list | manual, orphaned, held, upgradable |
| `--property <name>` | info, search | show only one property (completes the names) |
| `--regex` | search | the term is a regular expression |
| `--tree` | deps | full dependency tree |

## Coming from apt, dnf or apk

| vpk | apk | apt | dnf |
|---|---|---|---|
| `add` | `add` | `install` | `install` |
| `del` | `del` | `remove` | `remove` |
| `update` | `update` | `update` | `makecache` |
| `upgrade` | `upgrade` | `upgrade` | `upgrade` |
| `search` | `search` | `search` | `search` |
| `info` | `info` | `show` | `info` |
| `list` | `list -I` | `list --installed` | `list --installed` |
| `list -u` | `version -l '<'` | `list --upgradable` | `check-update` |
| `files` | `info -L` | `dpkg -L` | `repoquery -l` |
| `owns` | `info -W` | `dpkg -S` | `provides` |
| `rdeps` | `info -r` | `apt-cache rdepends` | `repoquery --whatrequires` |
| `autoremove` | (`del` does it) | `autoremove` | `autoremove` |
| `clean` | `cache clean` | `autoclean` | `clean packages` |
| `reinstall` | `fix --reinstall` | `reinstall` | `reinstall` |
| `mark manual` / `mark auto` | | `apt-mark manual` / `auto` | `mark install` / `remove` |
| `hold` / `unhold` | | `apt-mark hold` / `unhold` | `versionlock` |

**`update` only refreshes the index**, like apk and apt. In dnf, `dnf update` upgrades packages: the equivalent here is `vpk upgrade`.

## Behavior worth knowing

- **Package names come after `--`.** A name that starts with `-` is never taken as a flag.
- **`info` and `deps` look at the repositories** by default, so you can inspect a package before installing it. Add `-I` for the installed one. **`rdeps` and `files` look at the installed packages**; add `-a` for the repositories.
- **`del` removes the dependencies nothing else needs** (`xbps-remove -R`).
- **`upgrade` may run twice.** XBPS updates itself in a separate transaction. If updates are still pending after the first run, `vpk` runs it again once. A dry-run (`-s`), a download-only run (`-D`) or another root (`-p`) never repeats.
- **Dry-runs that do not sync need no root.** `vpk add -s htop` does not ask for a password.
- **Ctrl-C is safe.** `vpk` keeps waiting while xbps cleans up, and exits with xbps's own exit code (128+n if it was killed by signal n).
- **`list -u` shows what the last synced index says.** Run `vpk update` first.
- **`vpk add` fails if XBPS itself is out of date**, as `xbps-install` does. Run `vpk upgrade` first.

## Development

```sh
just check     # gofmt, go vet, go test
just build     # ./vpk, stamped with the git version
```

Layout:

```
main.go
internal/xbps/   running xbps (root escalation, exit codes, signals) and package lists for completion
internal/cli/    the commands
```

Every command is one entry in the `specs` table in `internal/cli/commands.go`: its name, aliases, flags, completion source and a pure function that returns the xbps command line. The help, the aliases and the completion all come from that table, and the tests run the real command tree with xbps stubbed out, so they check the exact command that would run.

To add a command, add an entry to `specs`, a row to `TestCommands`, and run `just check`.

## Roadmap

- A readable summary of the transaction (what changes, download size, disk use) with a confirmation prompt.
- A way to print the exact xbps command before running it.
- Support to xbps-src.

## Credits

[XBPS](https://github.com/void-linux/xbps) and Void Linux. The command line follows [apk](https://wiki.alpinelinux.org/wiki/Alpine_Package_Keeper). Inspired by [vpm](https://github.com/bahamas10/vpm) and [xtools](https://github.com/leahneukirchen/xtools).

## License

MIT. See [LICENSE](LICENSE).
