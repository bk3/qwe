# qwe

Personal scripts, available from any directory. `qwe` is a small standalone CLI for Linux and macOS; your commands can use any language.

```sh
qwe create repo-summary --runtime bash
qwe edit repo-summary
cd ~/Code/my-project
qwe repo-summary --dry-run
```

A command is simply an executable `~/.config/qwe/<name>/run`. No registry or manifest is required. `qwe` preserves your current directory, arguments, environment, terminal streams, exit status, and signals.

## Install and get started

Install from this checkout now (Go 1.24 or newer; a current supported release is recommended):

```sh
mkdir -p "$HOME/.local/bin"
go build -trimpath -o "$HOME/.local/bin/qwe" ./cmd/qwe
export PATH="$HOME/.local/bin:$PATH"
qwe --version
qwe create hello --runtime bash
qwe hello one "two three"
```

The standalone installed binary needs no Go runtime. Add the PATH line to your shell configuration if you want it to persist across terminal sessions.

Install the latest published release:

```sh
curl -fsSL https://raw.githubusercontent.com/bk3/qwe/main/scripts/install.sh | sh
```

It installs to `~/.local/bin`, verifies the release's SHA-256 checksum, checks that the binary runs, and atomically replaces any existing binary. It supports Linux and macOS on amd64 and arm64. It requires `curl` or `wget` and `sha256sum` or `shasum`. It prints a PATH notice when needed and never modifies shell configuration. Checksums protect against corrupt downloads; the installer and release assets come from the same trusted repository.

The curl installer requires publicly accessible release assets. If the repository is private, use authenticated GitHub Release downloads (for example, `gh release download v0.0.2 --repo bk3/qwe`) or install from an authenticated source checkout. Keep the matching binary for your platform, verify its SHA-256 checksum against `checksums.txt`, and place it on PATH.

Pin a release or choose another destination:

```sh
curl -fsSL https://raw.githubusercontent.com/bk3/qwe/main/scripts/install.sh |
  QWE_VERSION=v0.1.0 QWE_INSTALL_DIR="$HOME/bin" sh
```

`QWE_VERSION` defaults to `latest`. `QWE_RELEASE_BASE_URL` overrides the release base URL for mirrors and local installer tests; its default is `https://github.com/bk3/qwe/releases`.

## Commands

| Command | Behavior |
| --- | --- |
| `qwe <name> [args...]` | Execute the command's `run` entrypoint, forwarding every argument unchanged. |
| `qwe create <name> --runtime <runtime>` | Create an executable scaffold without overwriting existing files. |
| `qwe create <name>` | Ask for a runtime in an interactive terminal. Use `--runtime` in automation. |
| `qwe delete <name>` | Ask for confirmation, defaulting to no, then remove the entire command directory. |
| `qwe delete <name> --yes` | Delete without prompting; `-y` also works. |
| `qwe list` | Print runnable commands alphabetically, or explain that none exist. |
| `qwe which <name>` | Print the absolute command directory. |
| `qwe edit <name>` | Open the command directory with `VISUAL`, falling back to `EDITOR`. |
| `qwe code [name]` | Open the command root or a named command folder using `code` on PATH. |
| `qwe vim [name]` | Open the command root or a named command folder using `nvim` on PATH, falling back to `vim` if Neovim is unavailable. |
| `qwe upgrade` | Confirm and replace the running CLI with the latest stable release. |
| `qwe uninstall` | Confirm and remove the running CLI executable, keeping personal scripts. |
| `qwe help` / `qwe --help` | Show usage. |
| `qwe version` / `qwe --version` | Show the build version. |

Built-in flags may appear before or after the command name. Script flags belong to the script and pass through untouched. Names use letters, digits, underscores, or hyphens and must start with a letter or digit. Built-in names are reserved.

Configure your editor, including arguments if needed:

```sh
export EDITOR='code --wait'
qwe edit hello
```

Editor arguments support quoting, but do not undergo shell expansion or evaluation. If neither `VISUAL` nor `EDITOR` is configured, `qwe edit` reports how to set one.

Open all commands or an individual command directly:

```sh
qwe code           # Open ~/.config/qwe in the editor providing `code`
qwe code hello     # Open ~/.config/qwe/hello
qwe vim            # Open the command root in Neovim (or Vim)
qwe vim hello      # Open the hello folder in Neovim (or Vim)
```

These commands use executables on PATH, independently of `VISUAL` and `EDITOR`. `qwe vim` tries `nvim` first, then `vim` if `nvim` is unavailable. A failing Neovim session reports its failure without launching Vim. `code` can be provided by VS Code, Cursor, or another editor. If it is unavailable, qwe reports an error explaining that the executable must be installed or its shell command enabled. Shell aliases and functions are not executables on PATH.

Both commands honor `QWE_ROOT`, including symlinked dotfiles roots. Opening the root creates it and prepares the shared `.env` if needed; opening a named command requires an existing directory. A command folder can be opened even if its `run` entrypoint is missing or broken. `code` and `vim` are reserved built-in names, available starting in v0.0.2.

### Upgrade and uninstall

```sh
qwe upgrade
qwe uninstall
```

Both commands show exactly which executable will change and require `y` or `yes` at a `[y/N]` prompt. Enter, EOF, and other responses cancel. There is no `--yes` bypass for these commands. `upgrade` shows the installed and latest versions before confirmation; if the installed version is already current or newer, it exits without changing anything. Development builds can install the latest release after confirmation.

Upgrade downloads the matching Linux/macOS asset, verifies its SHA-256 checksum and `--version` output, then replaces the executable atomically in its current directory. Failed downloads or validation keep the existing installation. It updates custom installation paths too, without relying on `QWE_INSTALL_DIR`, and requires permission to write to the executable's directory.

For private GitHub releases, install and authenticate [GitHub CLI](https://cli.github.com/) with repository access (`gh auth login`). Upgrade uses that authentication when the release is unavailable anonymously. Public releases work without GitHub CLI.

Uninstall removes the running executable, including its resolved target when invoked through a symlink. Your command root, dotfiles, other copies of the CLI, shell configuration, and any invocation symlink remain untouched. It does not recursively delete installation directories. For package-manager installations, use the package manager to remove its own records and links.

`upgrade` and `uninstall` are reserved built-in names, available starting in v0.0.2. When upgrading from v0.0.1, reinstall the newer release first: v0.0.1 does not include an `upgrade` command.

## Write a command

The scaffold separates the entrypoint from the script:

```text
~/.config/qwe/
├── hello/
│   ├── run
│   └── script.sh
└── repo-summary/
    ├── run
    ├── script.ts
    └── config.json
```

Change `script.sh` or `script.ts` and run the command immediately. You can also write a single-file command directly:

```sh
mkdir -p "$HOME/.config/qwe/status"
cat > "$HOME/.config/qwe/status/run" <<'SCRIPT'
#!/bin/sh
exec git status --short "$@"
SCRIPT
chmod +x "$HOME/.config/qwe/status/run"

cd ~/Code/my-project
qwe status
```

The only contract is a regular, executable `run` file with a valid shebang or a native executable. Everything else in the directory belongs to you. Commands execute with your user's permissions.

### Runtime templates

| `--runtime` | Script | Runtime required by the command |
| --- | --- | --- |
| `bash` | `script.sh` | Bash |
| `node` | `script.js` | Node.js |
| `ts` | `script.ts` | Node.js and npm (`npx tsx`) |
| `go` | `main.go` | Go (`go run`) |
| `python` | `script.py` | Python 3 |

All generated `run` wrappers use Bash, so Bash is required alongside the selected runtime. Aliases include `js`/`javascript`, `typescript`, `golang`, and `py`/`python3`. Templates locate their own script files while preserving the caller's current directory. TypeScript invokes `npx tsx`, which can download `tsx` if it is not already available. Install required runtimes yourself, or customize `run` to invoke your preferred environment. `qwe` does not install runtimes or virtual environments; TypeScript package resolution is handled by `npx`.

### Current directory and command resources

If you run `qwe repo-summary` from `~/Code/my-project`, the script's working directory is `~/Code/my-project`. Arguments retain spaces and special characters, stdin/stdout/stderr connect directly to your terminal, and the command's exit code becomes `qwe`'s exit code.

The script receives two environment variables:

- `QWE_ROOT`: the absolute command root.
- `QWE_COMMAND_DIR`: the absolute directory containing this command.

Use `QWE_COMMAND_DIR` to read command-owned resources without changing the working directory:

```sh
cat "$QWE_COMMAND_DIR/config.json"
printf 'Project directory: %s\n' "$PWD"
```

Every script also receives variables from `<root>/.env`; existing shell environment values take precedence, including explicitly empty values. `qwe` replaces `QWE_ROOT` and `QWE_COMMAND_DIR` with the resolved paths regardless of their values in the shell or `.env`.

### Shared environment

`qwe create` and opening the root with `qwe code` or `qwe vim` prepare `~/.config/qwe/.env` (or `$QWE_ROOT/.env`) without overwriting existing files. Newly created `.env` files have permissions `0600`. Edit this file to define values shared by all your commands:

```dotenv
API_TOKEN=your-token
export PROJECT_NAME="my project"
DATA_DIR='/path/with spaces'
EMPTY_VALUE=
```

The file is loaded on every script invocation, so changes take effect immediately for both existing and newly created commands, in any runtime. A missing `.env` is allowed. Blank lines, comments, optional `export`, and single or double quoted values are supported. Double quotes support escapes such as `\n`, `\t`, `\\`, and `\"`; single quotes preserve literal text. Unquoted inline comments begin with whitespace followed by `#`. Variables and shell substitutions remain literal: the file is never executed as shell code. Malformed assignments report their line number and prevent the script from running without printing secret values.

Access values through the runtime's normal environment API: `$API_TOKEN` in Bash, `process.env.API_TOKEN` in Node or TypeScript, `os.Getenv("API_TOKEN")` in Go, or `os.environ.get("API_TOKEN")` in Python. Built-in commands and editors use your shell environment; `.env` applies to scripts launched through `qwe`.

For the default root, qwe adds `/qwe/.env` to `~/.config/.gitignore`, preserving existing rules. It also adds `/.env` to the command root's `.gitignore` so custom roots and roots symlinked into a dotfiles repository are covered. Existing tracked `.env` files must be untracked with `git rm --cached` if necessary.

### Command feedback

Built-in commands always print a result, prompt, or error. Empty or missing command roots report that no runnable commands exist. Scripts and editors print a completion or failure message to stderr after exiting, even if they produce no output themselves. Stdout stays available for command data and pipelines. qwe waits for scripts and editors as child processes, forwards interrupt and termination signals, and preserves their exit status.

## Storage and dotfiles

Commands default to `~/.config/qwe` on both macOS and Linux. Set `QWE_ROOT` to use a different root; relative values resolve against the directory where you invoke `qwe`:

```sh
QWE_ROOT=/tmp/my-commands qwe create example --runtime bash
QWE_ROOT=/tmp/my-commands qwe example
```

Keep commands in a dotfiles repository by symlinking the root (move existing commands first):

```sh
mkdir -p "$HOME/dotfiles/qwe/commands" "$HOME/.config"
ln -s "$HOME/dotfiles/qwe/commands" "$HOME/.config/qwe"
```

Symlinked command directories also work. Deleting a command that is a symlink removes only the link and preserves its target. Deleting a normal command directory removes all of its contents after confirmation. Invalid names and traversal paths are rejected.

The CLI binary stays separate from your scripts, normally at `~/.local/bin/qwe`.

## Develop and release

No third-party Go dependencies are required:

```sh
go test ./...
go test -race ./...
go vet ./...
sh -n scripts/install.sh scripts/install_test.sh
sh scripts/install_test.sh
mkdir -p bin
go build -o bin/qwe ./cmd/qwe
```

The installer integration test uses Python 3 to serve isolated HTTP fixtures; it verifies pinned/latest downloads, paths with spaces, checksum rejection, preservation of an existing installation, and staging cleanup. CI runs on Linux and macOS and cross-builds all four supported platform combinations.

To publish a release after pushing the implementation, create and push a version tag such as `v0.1.0`. The release workflow tests the project, embeds the tag using `-X main.version`, builds with `CGO_ENABLED=0`, and publishes these GitHub Release assets:

```text
qwe-linux-amd64
qwe-linux-arm64
qwe-darwin-amd64
qwe-darwin-arm64
checksums.txt
```

Source builds show a development version unless you supply a version via linker flags. See [the implementation plan](QWE_IMPLEMENTATION_PLAN.md) for the refined scope and decisions; [the kickoff](QWE_PROJECT_KICKOFF.md) preserves the original brainstorming.
