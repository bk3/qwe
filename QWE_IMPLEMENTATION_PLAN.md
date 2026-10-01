# qwe implementation plan

## Outcome

A small, dependency-free Go CLI for personal scripts on Linux and macOS. A command is an executable `<root>/<name>/run`; its other files remain entirely user-owned. No registry, metadata, runtime manager, or daemon is needed.

## Decisions

- Resolve `QWE_ROOT` first, then `~/.config/qwe`. Normalize relative overrides to absolute paths. Support symlinked roots and command directories for dotfiles.
- Run entrypoints with Unix process replacement. Preserve the caller's cwd, stdin, stdout, stderr, arguments, environment, exit status, and signals. Replace inherited `QWE_ROOT` and `QWE_COMMAND_DIR` values with the resolved paths. Never interpret script arguments through a shell.
- Names match `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`; built-in names are reserved. Require a regular, executable entrypoint and provide useful errors for missing or malformed commands.
- Implement `create`, `delete`, `list`, `which`, `edit`, `help`, and `version`. Built-in parsing accepts flags before or after the name; script flags are forwarded untouched. `which` prints the command directory.
- Create supports Bash, Node, TypeScript, Go, and Python plus obvious aliases. Explicit `--runtime` works without a terminal. Without it, prompt only on a terminal; otherwise report how to specify a runtime. Never overwrite an existing directory or symlink. Remove partial scaffolds on failure.
- Templates locate their own files while preserving cwd. TypeScript uses installed `tsx`, avoiding the kickoff's implicit `npx --yes` download. Runtime installation is the user's responsibility.
- Delete requires an explicit affirmative response or `--yes`. Validate a direct child path and use link-aware removal: deleting a command symlink removes only the link, never its target. Allow deleting a broken command so users can clean it up. These are personal, user-owned directories; defending against a hostile process concurrently replacing the entire root is outside the v1 threat model.
- List sorted runnable commands only; missing roots produce an empty list. Edit uses `VISUAL`, then `EDITOR`, supports quoted editor arguments without shell evaluation, and opens the directory. Missing editor configuration is an actionable error.
- Errors go to stderr with `qwe:` prefix. Child failures remain the child's native failures. CLI errors exit 1; help and cancellation exit 0.

## Work packages

1. Bootstrap `go.mod`, `cmd/qwe`, and a compact `internal/qwe` package. Implement dispatch, validation, root lookup, management, and Unix execution.
2. Build templates independently behind `Scaffold(runtime) ([]ScaffoldFile, error)`, with `ScaffoldFile{Name, Content string; Mode os.FileMode}`. Test template content, permissions, aliases, and runtime execution where available.
3. Add a POSIX installer and GitHub Actions CI/release workflows. Build static binaries for Linux/macOS on amd64/arm64. Verify SHA-256 before installation, stage replacement atomically, support a pinned release and custom install directory, and never edit shell startup files.
4. Test the compiled CLI as a subprocess: cwd, exact arguments, environment overrides, streaming IO, exit status, signals, create collisions, traversal rejection, symlink boundaries, prompts, and built-in flag handling.
5. Document installation from source and released assets, all commands, dotfiles, dependencies, editor configuration, and release procedure. Build a local binary so the tool is immediately usable in this workspace.

## Verification and completion

Run `go test ./...`, race tests, `go vet ./...`, shell syntax checks, installer smoke tests with local HTTP artifacts, and cross-build all four targets. Exercise the binary end-to-end using an isolated command root. CI repeats tests on Linux and macOS.

Deliver the separate plan, complete source, passing checks, README, installer, release workflow, and a working local binary. Release publication requires pushing the implementation and a version tag; do not claim the public curl installer works before those assets exist. Publishing a release is separate from building the ready-to-use local tool.
