package qwe

import (
	"fmt"
	"os"
	"strings"
)

// ScaffoldFile is a file to create inside a new command directory.
type ScaffoldFile struct {
	Name    string
	Content string
	Mode    os.FileMode
}

// CanonicalRuntime resolves the friendly names accepted by create.
func CanonicalRuntime(runtime string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(runtime)) {
	case "bash", "sh", "shell":
		return "bash", nil
	case "node", "nodejs", "javascript", "js":
		return "node", nil
	case "ts", "typescript":
		return "ts", nil
	case "go", "golang":
		return "go", nil
	case "python", "python3", "py":
		return "python", nil
	default:
		return "", fmt.Errorf("unknown runtime %q; choose bash, node, ts, go, or python", runtime)
	}
}

// Scaffold returns starter files; it never touches the filesystem.
func Scaffold(runtime string) ([]ScaffoldFile, error) {
	runtime, err := CanonicalRuntime(runtime)
	if err != nil {
		return nil, err
	}
	var name, invocation, content string
	mode := os.FileMode(0644)
	switch runtime {
	case "bash":
		name, invocation, mode = "script.sh", `bash "$DIR/script.sh"`, 0755
		content = `#!/usr/bin/env bash
set -euo pipefail

printf 'cwd: %s\n' "$PWD"
printf 'args:'
if (( $# )); then
  printf ' %q' "$@"
fi
printf '\n'
`
	case "node":
		name, invocation = "script.js", `node "$DIR/script.js"`
		content = `const args = process.argv.slice(2);

console.log("cwd:", process.cwd());
console.log("args:", args);
`
	case "ts":
		name, invocation = "script.ts", `tsx "$DIR/script.ts"`
		content = `const args: string[] = process.argv.slice(2);

console.log("cwd:", process.cwd());
console.log("args:", args);
`
	case "go":
		name, invocation = "main.go", `go run "$DIR/main.go"`
		content = `package main

import (
    "fmt"
    "os"
)

func main() {
    cwd, err := os.Getwd()
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    fmt.Println("cwd:", cwd)
    fmt.Println("args:", os.Args[1:])
}
`
	case "python":
		name, invocation = "script.py", `python3 "$DIR/script.py"`
		content = `import os
import sys

print("cwd:", os.getcwd())
print("args:", sys.argv[1:])
`
	}
	// cd happens in a subshell so command resources are located without
	// changing the caller's cwd. Clear CDPATH to avoid cd printing a path.
	run := `#!/usr/bin/env bash
set -euo pipefail

DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

exec ` + invocation + ` "$@"
`
	return []ScaffoldFile{
		{Name: "run", Content: run, Mode: 0755},
		{Name: name, Content: content, Mode: mode},
	}, nil
}
