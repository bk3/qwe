package qwe

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCanonicalRuntime(t *testing.T) {
	for canonical, aliases := range map[string][]string{
		"bash":   {"bash", "sh", "shell"},
		"node":   {"node", "nodejs", "javascript", "js"},
		"ts":     {"ts", "typescript"},
		"go":     {"go", "golang"},
		"python": {"python", "python3", "py"},
	} {
		for _, alias := range aliases {
			t.Run(alias, func(t *testing.T) {
				got, err := CanonicalRuntime(alias)
				if err != nil || got != canonical {
					t.Fatalf("CanonicalRuntime(%q) = %q, %v", alias, got, err)
				}
				original, _ := Scaffold(canonical)
				aliased, err := Scaffold(alias)
				if err != nil || !reflect.DeepEqual(original, aliased) {
					t.Fatalf("alias %q produces different scaffold: %v", alias, err)
				}
			})
		}
	}
	if got, err := CanonicalRuntime(" TypeScript "); err != nil || got != "ts" {
		t.Fatalf("normalized alias = %q, %v", got, err)
	}
	for _, invalid := range []string{"", "ruby", "../bash", "node --eval"} {
		if files, err := Scaffold(invalid); err == nil || files != nil {
			t.Errorf("Scaffold(%q) = %v, %v; want error and no files", invalid, files, err)
		}
	}
}

func TestScaffoldFiles(t *testing.T) {
	for runtime, script := range map[string]string{
		"bash": "script.sh", "node": "script.js", "ts": "script.ts", "go": "main.go", "python": "script.py",
	} {
		t.Run(runtime, func(t *testing.T) {
			files, err := Scaffold(runtime)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 2 || files[0].Name != "run" || files[1].Name != script {
				t.Fatalf("unexpected scaffold files: %+v", files)
			}
			if files[0].Mode != 0755 || files[1].Mode&0600 != 0600 || files[1].Mode&0022 != 0 {
				t.Fatalf("unexpected file permissions: %+v", files)
			}
			if !strings.HasPrefix(files[0].Content, "#!/usr/bin/env bash\n") || !strings.Contains(files[0].Content, ` "$@"`) {
				t.Fatal("run must have a Bash shebang and preserve argument boundaries")
			}
			if runtime == "ts" && !strings.Contains(files[0].Content, `exec npx tsx "$DIR/script.ts" "$@"`) {
				t.Fatal("TypeScript wrapper must invoke npx tsx and forward script arguments")
			}
			if !strings.Contains(files[1].Content, "cwd:") || !strings.Contains(files[1].Content, "args:") {
				t.Fatal("starter must demonstrate cwd and arguments")
			}
		})
	}
}

// Exercise the real wrappers with stub interpreters so every runtime can be
// verified without requiring Node, npm, tsx, Go, or Python to be installed.
// Stubbing npx also prevents the TypeScript test from downloading packages.
func TestScaffoldWrappers(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	for _, runtime := range []string{"bash", "node", "ts", "go", "python"} {
		t.Run(runtime, func(t *testing.T) {
			base := t.TempDir()
			base, err := filepath.EvalSymlinks(base)
			if err != nil {
				t.Fatal(err)
			}
			commandDir := filepath.Join(base, "command with spaces")
			callerDir := filepath.Join(base, "caller")
			binDir := filepath.Join(base, "bin")
			for _, dir := range []string{commandDir, callerDir, binDir} {
				if err := os.Mkdir(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			files, err := Scaffold(runtime)
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range files {
				if err := os.WriteFile(filepath.Join(commandDir, file.Name), []byte(file.Content), file.Mode); err != nil {
					t.Fatal(err)
				}
			}
			probe := "#!/bin/sh\nprintf 'cwd:<%s>\\n' \"$PWD\"\nprintf 'arg:<%s>\\n' \"$@\"\n"
			var wantArgs []string
			if runtime == "bash" {
				// Bash uses the actual interpreter and a replacement user script.
				if err := os.WriteFile(filepath.Join(commandDir, files[1].Name), []byte(probe), 0755); err != nil {
					t.Fatal(err)
				}
			} else {
				interpreter := map[string]string{"node": "node", "ts": "npx", "go": "go", "python": "python3"}[runtime]
				if err := os.WriteFile(filepath.Join(binDir, interpreter), []byte(probe), 0755); err != nil {
					t.Fatal(err)
				}
				if runtime == "ts" {
					wantArgs = append(wantArgs, "tsx")
				}
				if runtime == "go" {
					wantArgs = append(wantArgs, "run")
				}
				wantArgs = append(wantArgs, filepath.Join(commandDir, files[1].Name))
			}
			args := []string{"", "two words", "--flag=value", "$(touch unwanted)", "*"}
			wantArgs = append(wantArgs, args...)
			cmd := exec.Command(filepath.Join(commandDir, "run"), args...)
			cmd.Dir = callerDir
			cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"), "CDPATH="+base)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("wrapper execution: %v\n%s", err, output)
			}
			want := "cwd:<" + callerDir + ">\n"
			for _, arg := range wantArgs {
				want += "arg:<" + arg + ">\n"
			}
			if string(output) != want {
				t.Fatalf("wrapper output = %q; want %q", output, want)
			}
		})
	}
}
