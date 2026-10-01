# qwe CLI — Project Kickoff Specification

## 1. Project Summary

Build a small command-line tool named `qwe` that makes it easy to create, edit, manage, and run personal scripts from anywhere on the local system.

The primary use case is:

- I frequently need small one-off or reusable scripts.
- Those scripts may be written in Bash, Node.js, TypeScript, Go, Python, or another language.
- I want those scripts to live in one predictable location.
- I want to invoke any script from any directory or project using a short command.
- The script should execute as though it was launched from the directory where I ran `qwe`.
- I want the scripts to be easy to version-control as part of my dotfiles.
- I want `qwe` itself to be easy to install on multiple machines with a single `curl | sh` style command.

Example:

```bash
cd ~/Code/my-project

qwe cleanup-branches
qwe release --dry-run
qwe find-todos src/
```

The CLI should resolve the command by looking under:

```text
~/.config/qwe/<command-name>/
```

and execute that command's entrypoint.

---

## 2. High-Level Goals

`qwe` should optimize for:

1. **Simplicity**
   - Small implementation.
   - Minimal dependencies.
   - Easy to understand and maintain.
   - Avoid unnecessary configuration files or registries.

2. **Fast installation**
   - Single binary for `qwe`.
   - No runtime dependency required for the CLI itself.
   - Installable via one shell command.

3. **Language-agnostic commands**
   - Individual commands may use Bash, JavaScript, TypeScript, Go, Python, or anything else available on the machine.
   - `qwe` should not contain language-specific execution logic beyond scaffolding templates.

4. **Portable personal automation**
   - Commands should be suitable for version control in a dotfiles repository.
   - Moving to another machine should be straightforward.

5. **Unix-like behavior**
   - Preserve current working directory.
   - Preserve environment variables.
   - Forward stdin, stdout, and stderr.
   - Forward arguments.
   - Propagate the child process exit code.

6. **Predictable filesystem convention**
   - The filesystem should effectively act as the command registry.
   - Avoid a database or manifest unless future requirements clearly justify one.

---

## 3. Recommended Technology Choice

Build the `qwe` CLI in **Go**.

### Why Go

Go is preferred over Node.js for the CLI itself because:

- It compiles to a standalone native binary.
- The target machine does not need Node.js or Go installed to run `qwe`.
- Cross-compilation for macOS/Linux and ARM64/AMD64 is straightforward.
- Startup is fast.
- The standard library already provides everything required for:
  - filesystem operations,
  - process execution,
  - argument parsing,
  - environment handling,
  - terminal IO,
  - path handling.

Raw execution performance is not the main reason for choosing Go. The primary advantage is **distribution simplicity**.

Individual user commands can still use Node, TypeScript, Bash, Go, Python, etc.

---

## 4. Core Architectural Principle

A `qwe` command is simply:

```text
~/.config/qwe/<command-name>/run
```

The `run` file is the only file that `qwe` must understand.

Everything else inside the command directory belongs to the command itself.

Example:

```text
~/.config/qwe/
├── cleanup-branches/
│   ├── run
│   └── script.ts
├── find-todos/
│   ├── run
│   └── script.sh
├── repo-info/
│   └── run
└── release/
    ├── run
    ├── script.ts
    ├── config.json
    └── prompt.md
```

This is intentionally loose.

`qwe` should **not** require:

```text
command.json
manifest.yaml
registry.json
```

The filesystem itself is the registry.

---

## 5. Filesystem Layout

### User command storage

Default root:

```text
~/.config/qwe
```

Each command lives at:

```text
~/.config/qwe/<command-name>
```

Required entrypoint:

```text
~/.config/qwe/<command-name>/run
```

### CLI binary

The `qwe` binary should live separately from its configuration.

Recommended installation location:

```text
~/.local/bin/qwe
```

Do not install the `qwe` binary into:

```text
~/.config/qwe
```

The distinction should remain:

```text
Application:
~/.local/bin/qwe

User-owned commands:
~/.config/qwe/
```

---

## 6. Dotfiles Strategy

The intended long-term setup is for the user's commands to be version-controlled in a dotfiles repository.

Example:

```text
~/dotfiles/
├── qwe/
│   └── commands/
│       ├── cleanup-branches/
│       │   ├── run
│       │   └── script.ts
│       ├── find-todos/
│       │   ├── run
│       │   └── script.sh
│       └── ...
├── nvim/
├── zsh/
└── ...
```

Then:

```text
~/.config/qwe
```

can be a symlink to:

```text
~/dotfiles/qwe/commands
```

Example:

```bash
ln -s ~/dotfiles/qwe/commands ~/.config/qwe
```

`qwe` should work normally whether `~/.config/qwe` is a regular directory or a symlink.

Do not make dotfiles management part of the initial CLI implementation. The CLI only needs to operate against the resolved command root.

---

## 7. MVP Commands

The first version should support exactly these primary operations:

```bash
qwe create <name>
qwe delete <name>
qwe <name> [...args]
```

Useful secondary commands may also be included in the MVP if they remain trivial:

```bash
qwe list
qwe edit <name>
qwe which <name>
qwe version
qwe help
```

The first three are mandatory.

---

# 8. Command Specification

## 8.1 `qwe <name> [...args]`

Runs an existing user command.

Example:

```bash
cd ~/Code/example-project

qwe cleanup-branches --dry-run
```

Resolve:

```text
~/.config/qwe/cleanup-branches/run
```

Then execute it with:

```text
arguments:
--dry-run

working directory:
~/Code/example-project
```

### Required execution semantics

The child command must inherit:

- current working directory
- environment variables
- stdin
- stdout
- stderr

Arguments after the command name must be forwarded unchanged.

Example:

```bash
qwe foo one "two three" --bar=baz
```

must behave equivalently to executing:

```bash
~/.config/qwe/foo/run one "two three" --bar=baz
```

from the user's current directory.

### Critical cwd behavior

Do **not** change the child working directory to the command directory.

If the user runs:

```bash
cd ~/Code/project-a
qwe foo
```

then inside `foo`:

```text
process.cwd()
```

or:

```bash
pwd
```

should report:

```text
~/Code/project-a
```

This is central to the tool's usefulness.

### Exit codes

The `qwe` process should exit with the same exit code as the child command.

Example:

```bash
qwe lint
echo $?
```

If `lint` exits with:

```text
1
```

then `qwe` must exit with:

```text
1
```

### Signals

Where practical, process behavior should feel native.

At minimum:

- Ctrl+C should interrupt the underlying command.
- Interactive programs should continue to work because stdin/stdout/stderr are directly attached.

Do not capture child output only to re-print it later.

Attach the process directly to the parent streams.

---

## 8.2 Environment Variables Exposed by `qwe`

In addition to inheriting the current environment, inject:

```text
QWE_ROOT
QWE_COMMAND_DIR
```

Example:

```text
QWE_ROOT=/Users/example/.config/qwe
QWE_COMMAND_DIR=/Users/example/.config/qwe/release
```

This solves an important distinction:

```text
Where the command's own files live:
$QWE_COMMAND_DIR

Where the user invoked qwe:
current working directory
```

A command may therefore access local resources:

```text
~/.config/qwe/release/config.json
```

without losing awareness of the project it is operating on.

---

## 8.3 `qwe create <name>`

Creates a new command directory.

Basic behavior:

```bash
qwe create foo
```

should:

1. Validate the command name.
2. Fail if the command already exists.
3. Ask which runtime/template to use if no runtime was provided.
4. Create:

```text
~/.config/qwe/foo/
```

5. Scaffold the appropriate files.
6. Ensure `run` is executable.
7. Print the created path and basic usage instructions.

### Non-interactive form

Support:

```bash
qwe create foo --runtime bash
qwe create foo --runtime node
qwe create foo --runtime ts
qwe create foo --runtime go
```

Aliases may be accepted where obvious:

```text
typescript -> ts
javascript -> node
js -> node
golang -> go
```

Canonical internal values should remain simple.

### Default supported templates

Initial templates:

- Bash
- Node.js
- TypeScript
- Go

Python may be added if trivial, but it is not required for the first implementation.

---

## 8.4 Bash Scaffold

Recommended output:

```text
~/.config/qwe/foo/
├── run
└── script.sh
```

`run`:

```bash
#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

exec "$DIR/script.sh" "$@"
```

`script.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "cwd: $(pwd)"
printf 'args:'
printf ' %q' "$@"
printf '\n'
```

Make both files executable if useful, though only `run` is required by `qwe`.

---

## 8.5 Node Scaffold

Recommended output:

```text
~/.config/qwe/foo/
├── run
└── script.js
```

`run`:

```bash
#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

exec node "$DIR/script.js" "$@"
```

`script.js`:

```javascript
const args = process.argv.slice(2);

console.log("cwd:", process.cwd());
console.log("args:", args);
```

---

## 8.6 TypeScript Scaffold

Recommended output:

```text
~/.config/qwe/foo/
├── run
└── script.ts
```

`run`:

```bash
#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

exec npx --yes tsx "$DIR/script.ts" "$@"
```

`script.ts`:

```typescript
const args = process.argv.slice(2);

console.log("cwd:", process.cwd());
console.log("args:", args);
```

### Important note

`qwe` itself must not attempt to install or bundle the TypeScript runtime.

The command owns its runtime requirements.

If `tsx`, Node, Go, Python, etc. are not installed, normal shell/process errors are acceptable.

A future `qwe doctor` command may help identify missing dependencies.

---

## 8.7 Go Scaffold

Recommended output:

```text
~/.config/qwe/foo/
├── run
└── main.go
```

A simple `run` implementation may use:

```bash
#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

exec go run "$DIR/main.go" "$@"
```

`main.go`:

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	fmt.Println("cwd:", cwd)
	fmt.Println("args:", os.Args[1:])
}
```

Do not prematurely optimize Go scripts into per-command compiled binaries.

That may be introduced later if useful.

---

## 8.8 Single-File Commands Must Also Work

Although scaffolding may use:

```text
run + script
```

the command contract must only require `run`.

This must be valid:

```text
~/.config/qwe/foo/
└── run
```

Example:

```bash
#!/usr/bin/env bash
set -euo pipefail

git status --short
```

This keeps the abstraction flexible and avoids making the scaffolding convention part of the execution contract.

---

# 9. `qwe delete <name>`

Deletes a command directory.

Example:

```bash
qwe delete foo
```

Prompt:

```text
Delete command "foo"?

This will remove:
/Users/example/.config/qwe/foo

[y/N]:
```

Default answer should be **No**.

Accepted affirmative forms may include:

```text
y
Y
yes
YES
```

Anything else should cancel safely.

### Non-interactive deletion

Support:

```bash
qwe delete foo --yes
```

Optional alias:

```bash
qwe delete foo -y
```

### Delete behavior

Delete the entire directory:

```text
~/.config/qwe/foo
```

Do not delete a command unless:

- its name passed validation,
- its resolved path is confirmed to be a direct child of the configured qwe root.

Filesystem deletion must be implemented conservatively.

---

# 10. Command Name Validation

Command names are used directly in filesystem paths, so validation is security-critical.

Recommended allowed pattern:

```regex
^[a-zA-Z0-9][a-zA-Z0-9_-]*$
```

Valid examples:

```text
foo
cleanup-branches
release_prod
123
v2-release
```

Invalid examples:

```text
../foo
foo/bar
/foo
.
..
foo bar
foo/../bar
```

Never allow command names to inject path traversal.

### Reserved command names

Reserve internal CLI command names.

At minimum:

```text
create
delete
list
edit
which
help
version
```

Therefore:

```bash
qwe create create
```

must fail with a useful error.

This avoids future namespace collisions because:

```bash
qwe <anything>
```

is also the script execution syntax.

---

# 11. Recommended CLI Parsing

The initial implementation does **not** require Cobra or another framework.

The Go standard library is sufficient.

Keep dependencies minimal unless a library materially improves behavior.

Conceptually:

```go
switch firstArg {
case "create":
    // create
case "delete":
    // delete
case "list":
    // optional
case "edit":
    // optional
case "which":
    // optional
case "help", "--help", "-h":
    // help
case "version", "--version":
    // version
default:
    // execute user command
}
```

If argument handling becomes substantially more complicated, a CLI framework may be introduced later.

---

# 12. Recommended Go Project Structure

Keep the implementation modest.

Suggested structure:

```text
qwe/
├── cmd/
│   └── qwe/
│       └── main.go
├── internal/
│   ├── command/
│   │   ├── create.go
│   │   ├── delete.go
│   │   ├── execute.go
│   │   ├── list.go
│   │   ├── edit.go
│   │   └── which.go
│   ├── config/
│   │   └── config.go
│   ├── names/
│   │   └── validate.go
│   └── scaffold/
│       ├── bash.go
│       ├── node.go
│       ├── typescript.go
│       └── golang.go
├── scripts/
│   └── install.sh
├── .github/
│   └── workflows/
│       └── release.yml
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

Do not force this structure if an even simpler organization is more appropriate during implementation.

The objective is maintainability, not architectural ceremony.

---

# 13. Configuration Root

Default:

```text
~/.config/qwe
```

Centralize root resolution in one package/function.

Example concept:

```go
func Root() (string, error)
```

For the first version, use the home directory plus:

```text
.config/qwe
```

### Optional environment override

It would be useful to support:

```text
QWE_ROOT
```

as an override.

Example:

```bash
QWE_ROOT=/tmp/qwe-test qwe create foo --runtime bash
```

This is particularly helpful for:

- automated tests,
- isolated experimentation,
- alternate setups.

Resolution precedence:

```text
1. QWE_ROOT environment variable, if non-empty
2. ~/.config/qwe
```

This is recommended for the MVP because it makes end-to-end testing significantly easier.

---

# 14. Execution Implementation

The core execution logic should roughly follow this shape:

```go
func Execute(name string, args []string) error {
	root, err := config.Root()
	if err != nil {
		return err
	}

	if err := names.Validate(name); err != nil {
		return err
	}

	commandDir := filepath.Join(root, name)
	entrypoint := filepath.Join(commandDir, "run")

	info, err := os.Stat(entrypoint)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("qwe command %q does not exist", name)
		}

		return err
	}

	if info.IsDir() {
		return fmt.Errorf("command entrypoint is a directory: %s", entrypoint)
	}

	cmd := exec.Command(entrypoint, args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = append(
		os.Environ(),
		"QWE_ROOT="+root,
		"QWE_COMMAND_DIR="+commandDir,
	)

	return cmd.Run()
}
```

Important:

```go
cmd.Dir
```

should generally remain unset.

That causes the child to inherit the caller's working directory.

---

# 15. Exit Code Handling

Do not treat a child exit code such as `1` as an internal qwe failure that becomes some unrelated qwe exit code.

If:

```text
foo/run
```

returns:

```text
42
```

then:

```bash
qwe foo
```

should return:

```text
42
```

The `main` package should distinguish:

- CLI/internal errors
- child process exit status

and terminate appropriately.

Tests should explicitly cover this.

---

# 16. Error UX

Errors should be concise and actionable.

Examples:

```text
qwe: command "foo" does not exist
```

```text
qwe: invalid command name "../foo"
```

```text
qwe: command "foo" already exists at /Users/example/.config/qwe/foo
```

```text
qwe: "create" is reserved and cannot be used as a command name
```

```text
qwe: command "foo" is missing its run entrypoint
```

Avoid stack traces for expected user errors.

---

# 17. Optional MVP Quality-of-Life Commands

## `qwe list`

List available commands by reading direct child directories under the qwe root.

Example:

```bash
qwe list
```

Output:

```text
cleanup-branches
find-todos
release
repo-info
```

Sort alphabetically.

Only show valid command directories.

It is reasonable to require a `run` entrypoint before listing a directory as a runnable command.

---

## `qwe which <name>`

Example:

```bash
qwe which release
```

Output:

```text
/Users/example/.config/qwe/release
```

This is useful for shell composition and debugging.

---

## `qwe edit <name>`

Open the command directory or primary script using `$EDITOR`.

Simple initial behavior:

```text
$EDITOR ~/.config/qwe/<name>
```

If `$EDITOR` is not defined, return a useful error instead of guessing.

This command is optional for the first pass.

---

# 18. Installation Requirements

The target installation UX should be:

```bash
curl -fsSL https://raw.githubusercontent.com/<owner>/qwe/main/scripts/install.sh | sh
```

The installer should:

1. Detect operating system.
2. Detect CPU architecture.
3. Determine the appropriate GitHub Release artifact.
4. Download the binary.
5. Install it to:

```text
~/.local/bin/qwe
```

6. Mark it executable.
7. Verify installation if possible.
8. Warn if `~/.local/bin` does not appear to be in `PATH`.
9. Avoid silently modifying shell configuration unless explicitly designed and documented.

Initial platform targets:

```text
darwin/arm64
darwin/amd64
linux/arm64
linux/amd64
```

Windows is not required for the initial implementation.

---

# 19. Releases and CI

Use GitHub Actions to publish release binaries.

Recommended trigger:

```text
Git tag:
v0.1.0
v0.2.0
...
```

Release artifacts should use predictable names such as:

```text
qwe-darwin-arm64
qwe-darwin-amd64
qwe-linux-arm64
qwe-linux-amd64
```

Checksums are recommended:

```text
checksums.txt
```

The install script should ideally verify checksums before installing the binary.

A tool such as GoReleaser may be used if it materially simplifies release automation, but it is not mandatory.

Prefer low complexity.

---

# 20. Runtime Responsibility

`qwe` manages commands.

It does **not** manage language runtimes.

For example:

```text
TypeScript command
    ↓
requires Node + tsx

Go command
    ↓
requires Go if run via `go run`

Python command
    ↓
requires Python
```

This is intentional.

Do not attempt to:

- bundle Node,
- install Python,
- install Go,
- automatically install `tsx`,
- create virtual environments,
- manage npm dependencies globally.

Those concerns belong to the command itself or the machine environment.

---

# 21. Security and Safety Requirements

This CLI can create, execute, and recursively delete files. Treat filesystem boundaries carefully.

Mandatory requirements:

1. Validate all command names.
2. Never blindly concatenate untrusted names into destructive paths.
3. Use `filepath.Join`.
4. Ensure destructive operations resolve inside the qwe root.
5. Ensure deletion targets a direct child of the qwe root.
6. Never recursively delete the root itself.
7. Reject:
   - `.`
   - `..`
   - absolute paths
   - names containing separators
8. Require confirmation before destructive deletion unless `--yes` is passed.
9. Do not execute arbitrary fallback paths when `run` is missing.
10. Do not shell-concatenate user arguments.
11. Use `exec.Command` argument arrays so user arguments remain distinct and are not reinterpreted by an additional shell.

Where symbolic links are involved, be deliberate about deletion behavior and cover it with tests.

Deleting a command directory that is itself a symlink should not accidentally recursively remove the external target.

---

# 22. Testing Strategy

Use Go's built-in `testing` package.

Prefer temporary directories using:

```go
t.TempDir()
```

and set:

```text
QWE_ROOT
```

during tests.

## Unit tests

Cover:

### Name validation

Valid:

```text
foo
foo-bar
foo_bar
foo123
123foo
```

Invalid:

```text
.
..
../foo
foo/bar
/foo
foo bar
```

Reserved names:

```text
create
delete
list
edit
which
help
version
```

### Root resolution

Verify:

```text
QWE_ROOT override
```

and fallback behavior.

### Scaffold generation

For every runtime:

- correct files are created,
- `run` exists,
- `run` is executable,
- expected starter content exists,
- existing commands are never overwritten.

### Delete safety

Verify:

- no deletion without confirmation,
- `--yes` deletes,
- nonexistent commands fail cleanly,
- path traversal cannot escape the root,
- root itself cannot be deleted.

---

## Integration tests

Create actual temporary `run` files and invoke the CLI behavior.

Test:

### Working directory preservation

A command containing:

```bash
pwd
```

must return the directory from which qwe was invoked.

### Argument forwarding

Input:

```bash
qwe foo a "b c" --thing=value
```

The script should observe exactly three arguments:

```text
a
b c
--thing=value
```

### Environment forwarding

A parent environment variable should be visible inside the script.

### qwe environment variables

Verify:

```text
QWE_ROOT
QWE_COMMAND_DIR
```

### stdout

Child stdout should be directly visible.

### stderr

Child stderr should be directly visible.

### exit status

A child exiting `37` should cause qwe to exit `37`.

---

# 23. README Requirements

The project README should document:

1. What `qwe` is.
2. Installation.
3. Basic usage.
4. Creating commands.
5. Executing commands.
6. Deleting commands.
7. Command filesystem structure.
8. Runtime templates.
9. `QWE_ROOT`.
10. `QWE_COMMAND_DIR`.
11. Dotfiles integration example.
12. Runtime dependency model.
13. Development instructions.
14. Release process.

Keep the README practical and example-heavy.

---

# 24. Non-Goals for v1

Do **not** build these unless they become necessary during implementation:

- command metadata files,
- YAML configuration,
- JSON command registries,
- embedded databases,
- plugin systems,
- remote package registries,
- automatic runtime installation,
- automatic npm dependency management,
- per-project qwe configuration,
- cloud synchronization,
- GUI,
- command sharing marketplace,
- elaborate dependency injection,
- daemon/background process,
- command telemetry,
- shell aliases for every script.

The goal of v1 is a small, reliable personal CLI.

---

# 25. Future Enhancements

Design so these remain possible, but do not implement unless they are trivial or explicitly needed.

Potential future features:

```bash
qwe list
qwe edit <name>
qwe which <name>
qwe doctor
qwe rename <old> <new>
qwe copy <source> <target>
qwe create <name> --runtime python
qwe create <name> --runtime bun
qwe create <name> --runtime deno
qwe create <name> --runtime ruby
```

Possible later improvements:

- shell completions,
- fuzzy command selection,
- command descriptions,
- tags/categories,
- project-local commands,
- user-global vs repository-local commands,
- template customization,
- command dependency checks,
- optional metadata,
- `qwe update`,
- Homebrew distribution,
- version pinning,
- remote command collections.

These should not compromise the simple core:

```text
qwe <name>
    ↓
~/.config/qwe/<name>/run
```

---

# 26. Suggested Implementation Order

The AI agent should implement this project in the following order.

## Phase 1 — Bootstrap

1. Initialize Go module.
2. Create `cmd/qwe/main.go`.
3. Implement root resolution.
4. Implement command-name validation.
5. Implement useful errors.

## Phase 2 — Command Execution

Implement:

```bash
qwe <name> [...args]
```

Verify:

- path lookup,
- cwd inheritance,
- environment inheritance,
- stdin forwarding,
- stdout forwarding,
- stderr forwarding,
- argument forwarding,
- injected qwe environment variables,
- exit code propagation.

This is the most important part of the project.

## Phase 3 — Create

Implement:

```bash
qwe create <name>
qwe create <name> --runtime <runtime>
```

Add scaffolds for:

- bash,
- node,
- ts,
- go.

Ensure:

- directories are created safely,
- existing commands are not overwritten,
- `run` is executable.

Interactive runtime selection should be simple; avoid pulling in a large dependency solely for a fancy prompt.

## Phase 4 — Delete

Implement:

```bash
qwe delete <name>
qwe delete <name> --yes
```

Focus heavily on path safety and confirmation behavior.

## Phase 5 — Small Utilities

Implement:

```bash
qwe list
qwe which <name>
```

Implement `qwe edit` if straightforward.

## Phase 6 — Tests

Add unit and integration coverage for all important behavior.

Tests should be runnable with:

```bash
go test ./...
```

## Phase 7 — Distribution

Add:

```text
scripts/install.sh
.github/workflows/release.yml
```

Produce release artifacts for:

```text
darwin/arm64
darwin/amd64
linux/arm64
linux/amd64
```

## Phase 8 — Documentation

Write README examples and development/release instructions.

---

# 27. Acceptance Criteria

The project should be considered functionally complete when all of the following work.

## Installation

A released version can be installed from a clean supported machine with one command resembling:

```bash
curl -fsSL https://raw.githubusercontent.com/<owner>/qwe/main/scripts/install.sh | sh
```

and results in a working:

```bash
qwe --version
```

---

## Creating a Bash command

```bash
qwe create hello --runtime bash
```

creates:

```text
~/.config/qwe/hello/
├── run
└── script.sh
```

and:

```bash
qwe hello
```

successfully runs the script.

---

## Creating a TypeScript command

```bash
qwe create inspect --runtime ts
```

creates:

```text
~/.config/qwe/inspect/
├── run
└── script.ts
```

and on a machine with the required Node/tsx runtime:

```bash
qwe inspect foo bar
```

runs successfully.

---

## Invocation from arbitrary directories

Given:

```bash
cd ~/Code/project-a
qwe current-project
```

the child command observes:

```text
~/Code/project-a
```

as its cwd.

Given:

```bash
cd ~/Code/project-b
qwe current-project
```

the same qwe command observes:

```text
~/Code/project-b
```

---

## Arguments

```bash
qwe example foo "bar baz" --hello=world
```

preserves all arguments exactly.

---

## Environment

A variable such as:

```bash
EXAMPLE=hello qwe foo
```

is visible to the child process.

The child additionally receives:

```text
QWE_ROOT
QWE_COMMAND_DIR
```

---

## IO

Interactive scripts can read from stdin.

stdout and stderr appear directly in the terminal.

---

## Exit codes

If:

```text
~/.config/qwe/fail/run
```

exits with code:

```text
17
```

then:

```bash
qwe fail
echo $?
```

prints:

```text
17
```

---

## Deletion

```bash
qwe delete hello
```

requires confirmation.

```bash
qwe delete hello --yes
```

removes the command without prompting.

No malicious or malformed command name can cause deletion outside:

```text
~/.config/qwe
```

---

## Dotfiles

The tool works normally when:

```text
~/.config/qwe
```

is a symlink to a directory inside a git-backed dotfiles repository.

---

# 28. Example End-to-End User Experience

```bash
$ qwe create repo-summary --runtime ts

Created qwe command "repo-summary":

  ~/.config/qwe/repo-summary/run
  ~/.config/qwe/repo-summary/script.ts

Run it with:

  qwe repo-summary
```

User edits:

```text
~/.config/qwe/repo-summary/script.ts
```

Then:

```bash
$ cd ~/Code/project-one
$ qwe repo-summary
Repository: project-one
Branch: main
Files changed: 4
```

Later:

```bash
$ cd ~/Code/project-two
$ qwe repo-summary
Repository: project-two
Branch: feature/foo
Files changed: 12
```

No command installation step is required after editing or adding files.

The presence of:

```text
~/.config/qwe/repo-summary/run
```

makes the command immediately available.

---

# 29. Design Principle to Preserve

The most important architectural rule is:

```text
qwe does not need to understand the user's script.
```

It only needs to understand:

```text
~/.config/qwe/<command>/run
```

Everything beyond that belongs to the command author.

Keep this invariant intact unless a future feature presents a compelling reason to change it.

The ideal implementation should remain conceptually reducible to:

```text
qwe <command> [...args]

        │
        ▼

~/.config/qwe/<command>/run [...args]

        │
        ├── cwd: inherited
        ├── environment: inherited
        ├── stdin: inherited
        ├── stdout: inherited
        ├── stderr: inherited
        ├── args: forwarded exactly
        ├── QWE_ROOT: injected
        ├── QWE_COMMAND_DIR: injected
        └── exit code: propagated
```

---

# 30. Instructions to the Implementing AI Agent

Treat this document as the project specification.

The objective is to produce a working, tested, documented Go CLI rather than merely scaffolding an incomplete codebase.

Use sound judgment when small implementation details are unspecified, but preserve the core design described above.

Priorities, in order:

1. Correct process-execution semantics.
2. Filesystem safety.
3. Simple user experience.
4. Easy installation.
5. Reliable tests.
6. Maintainable code.
7. Minimal dependencies.

Do not over-engineer the project.

When there is a choice between:

```text
a clever abstraction
```

and:

```text
a small obvious implementation
```

prefer the small obvious implementation.

The finished project should be usable immediately as a personal CLI utility and should provide a solid foundation for future expansion without requiring those future features today.
