package integration

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

var binary string

func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		os.Exit(0)
	}
	dir, err := os.MkdirTemp("", "qwe-integration-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "qwe")
	build := exec.Command("go", "build", "-o", binary, "../cmd/qwe")
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build integration binary: %v\n%s", err, output)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type result struct {
	stdout, stderr string
	code           int
}

func baseEnv(root string) []string {
	env := make([]string, 0, len(os.Environ())+4)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		switch key {
		case "QWE_ROOT", "QWE_COMMAND_DIR", "VISUAL", "EDITOR":
			continue
		}
		env = append(env, value)
	}
	return append(env, "QWE_ROOT="+root)
}

func invoke(t *testing.T, root, cwd, input string, extraEnv []string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = append(baseEnv(root), extraEnv...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(input)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return result{out.String(), stderr.String(), code}
}

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func command(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, name)
	write(t, filepath.Join(path, "run"), "#!/bin/sh\n"+body+"\n", 0755)
	return path
}

func assertOK(t *testing.T, r result) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("exit %d; stdout=%q stderr=%q", r.code, r.stdout, r.stderr)
	}
}

func assertError(t *testing.T, r result) {
	t.Helper()
	if r.code != 1 || !strings.Contains(r.stderr, "qwe:") {
		t.Fatalf("want CLI error exit 1, got %+v", r)
	}
}

// macOS exposes temporary directories through /var -> /private/var. A child's
// getcwd (and shell PWD) may use the physical path even when cmd.Dir used a link.
func physicalPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestExecutionPreservesProcessContract(t *testing.T) {
	root := filepath.Join(t.TempDir(), "commands")
	cwd := t.TempDir()
	dir := command(t, root, "inspect", `printf 'cwd=%s\n' "$PWD"
printf 'root=%s\ndir=%s\ninherited=%s\n' "$QWE_ROOT" "$QWE_COMMAND_DIR" "$QWE_TEST_VALUE"
for arg do printf 'arg=<%s>\n' "$arg"; done
cat
printf 'child stderr\n' >&2`)
	args := []string{"inspect", "--help", "--runtime", "two words", "", "$(touch should-not-exist)", "a'b\"c", "--"}
	r := invoke(t, root, cwd, "stdin payload\n", []string{"QWE_COMMAND_DIR=wrong", "QWE_TEST_VALUE=inherited value"}, args...)
	assertOK(t, r)
	want := fmt.Sprintf("cwd=%s\nroot=%s\ndir=%s\ninherited=inherited value\n", physicalPath(t, cwd), root, dir)
	for _, arg := range args[1:] {
		want += "arg=<" + arg + ">\n"
	}
	want += "stdin payload\n"
	if r.stdout != want || r.stderr != "child stderr\n" {
		t.Fatalf("stdout=%q stderr=%q; want stdout=%q", r.stdout, r.stderr, want)
	}
	if _, err := os.Stat(filepath.Join(cwd, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatalf("argument was evaluated: %v", err)
	}
}

func TestRelativeRootAndChildExit(t *testing.T) {
	cwd := t.TempDir()
	root := filepath.Join(cwd, "commands")
	command(t, root, "status", `printf '%s\n' "$QWE_ROOT"; exit 37`)
	r := invoke(t, "commands", cwd, "", nil, "status")
	if r.code != 37 || r.stdout != physicalPath(t, root)+"\n" || r.stderr != "" {
		t.Fatalf("unexpected result: %+v", r)
	}
}

func TestSignalReachesExecutedCommand(t *testing.T) {
	for _, signal := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			root := t.TempDir()
			command(t, root, "waiter", `printf 'ready\n'; exec sleep 30`)
			cmd := exec.Command(binary, "waiter")
			cmd.Env = baseEnv(root)
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer cmd.Process.Kill()
			ready := make(chan bool, 1)
			go func() { ready <- bufio.NewScanner(out).Scan() }()
			select {
			case ok := <-ready:
				if !ok {
					t.Fatal("command did not announce readiness")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("command did not start")
			}
			if err := cmd.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				exit, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatalf("want signaled exit, got %v", err)
				}
				status := exit.Sys().(syscall.WaitStatus)
				if !status.Signaled() || status.Signal() != signal {
					t.Fatalf("unexpected status %v", status)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("%s did not terminate command", signal)
			}
		})
	}
}

func TestCreateRuntimesAndFlags(t *testing.T) {
	for _, language := range []string{"bash", "node", "ts", "go", "python"} {
		t.Run(language, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "commands")
			for i, args := range [][]string{{"create", "first", "--runtime", language}, {"create", "--runtime=" + language, "second"}} {
				r := invoke(t, root, "", "", nil, args...)
				assertOK(t, r)
				name := []string{"first", "second"}[i]
				st, err := os.Stat(filepath.Join(root, name, "run"))
				if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0111 == 0 {
					t.Fatalf("missing executable scaffold: stat=%v err=%v", st, err)
				}
			}
		})
	}
}

func TestCreateNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	dir := command(t, root, "existing", "echo original")
	marker := filepath.Join(dir, "custom.txt")
	write(t, marker, "precious content", 0600)
	assertError(t, invoke(t, root, "", "", nil, "create", "existing", "--runtime", "bash"))
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "precious content" {
		t.Fatalf("existing files altered: %q %v", data, err)
	}
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	assertError(t, invoke(t, root, "", "", nil, "create", "linked", "--runtime", "bash"))
	if _, err := os.Lstat(filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
}

func TestCreateErrorsLeaveNoScaffold(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"create", "unknown", "--runtime", "ruby"}, {"create", "no-runtime"}, {"create", "missing-value", "--runtime"}, {"create", "extra", "--runtime", "bash", "spare"}, {"create", "badflag", "--unknown"}} {
		assertError(t, invoke(t, root, "", "", nil, args...))
		if _, err := os.Lstat(filepath.Join(root, args[1])); !os.IsNotExist(err) {
			t.Fatalf("failed create left directory %s: %v", args[1], err)
		}
	}
}

func TestListOnlyRunnableSorted(t *testing.T) {
	root := t.TempDir()
	command(t, root, "zebra", "exit 0")
	command(t, root, "Alpha", "exit 0")
	write(t, filepath.Join(root, "disabled", "run"), "#!/bin/sh\n", 0644)
	if err := os.MkdirAll(filepath.Join(root, "directory", "run"), 0755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "loose-file"), "x", 0755)
	if err := os.Symlink(filepath.Join(root, "not-there"), filepath.Join(root, "broken")); err != nil {
		t.Fatal(err)
	}
	r := invoke(t, root, "", "", nil, "list")
	assertOK(t, r)
	if r.stdout != "Alpha\nzebra\n" {
		t.Fatalf("unexpected list: %q", r.stdout)
	}
	absent := filepath.Join(t.TempDir(), "absent")
	r = invoke(t, absent, "", "", nil, "list")
	assertOK(t, r)
	if r.stdout != "" {
		t.Fatalf("missing root list: %q", r.stdout)
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatalf("list created root: %v", err)
	}
}

func TestWhichAndSymlinkedRoot(t *testing.T) {
	realRoot := t.TempDir()
	command(t, realRoot, "hello", "echo works")
	linkedRoot := filepath.Join(t.TempDir(), "dotfiles")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}
	r := invoke(t, linkedRoot, "", "", nil, "hello")
	assertOK(t, r)
	if r.stdout != "works\n" {
		t.Fatalf("symlink execution: %+v", r)
	}
	r = invoke(t, linkedRoot, "", "", nil, "which", "hello")
	assertOK(t, r)
	path := strings.TrimSuffix(r.stdout, "\n")
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != physicalPath(t, filepath.Join(realRoot, "hello")) {
		t.Fatalf("which path=%q resolved=%q err=%v", path, resolved, err)
	}
}

func TestDeleteConfirmationAndFlagPlacement(t *testing.T) {
	root := t.TempDir()
	for _, input := range []string{"", "\n", "n\n", "no\n", "maybe\n"} {
		command(t, root, "keep", "exit 0")
		assertOK(t, invoke(t, root, "", input, nil, "delete", "keep"))
		if _, err := os.Stat(filepath.Join(root, "keep")); err != nil {
			t.Fatalf("non-affirmative deleted command: %q %v", input, err)
		}
	}
	command(t, root, "confirmed", "exit 0")
	assertOK(t, invoke(t, root, "", "y\n", nil, "delete", "confirmed"))
	if _, err := os.Stat(filepath.Join(root, "confirmed")); !os.IsNotExist(err) {
		t.Fatalf("affirmative did not delete: %v", err)
	}
	for i, args := range [][]string{{"delete", "first", "--yes"}, {"delete", "--yes", "second"}, {"delete", "-y", "third"}, {"delete", "fourth", "-y"}} {
		name := []string{"first", "second", "third", "fourth"}[i]
		command(t, root, name, "exit 0")
		assertOK(t, invoke(t, root, "", "", nil, args...))
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("delete did not remove %s: %v", name, err)
		}
	}
}

func TestDeletingCommandSymlinkKeepsTarget(t *testing.T) {
	root, target := t.TempDir(), t.TempDir()
	write(t, filepath.Join(target, "run"), "#!/bin/sh\necho target\n", 0755)
	write(t, filepath.Join(target, "valuable"), "preserve", 0644)
	link := filepath.Join(root, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	assertOK(t, invoke(t, root, "", "", nil, "linked"))
	assertOK(t, invoke(t, root, "", "", nil, "delete", "linked", "--yes"))
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("link not removed: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(target, "valuable")); err != nil || string(data) != "preserve" {
		t.Fatalf("target altered: %q %v", data, err)
	}
	if err := os.Symlink(filepath.Join(target, "missing"), filepath.Join(root, "broken")); err != nil {
		t.Fatal(err)
	}
	assertOK(t, invoke(t, root, "", "", nil, "delete", "broken", "--yes"))
}

func TestDeletingDirectoryWithNestedSymlinksKeepsTargets(t *testing.T) {
	root, target := t.TempDir(), t.TempDir()
	dir := command(t, root, "nested", "exit 0")
	write(t, filepath.Join(target, "valuable"), "preserve", 0644)
	if err := os.MkdirAll(filepath.Join(dir, "resources"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, link := range []struct{ name, target string }{
		{"external-directory", target},
		{"external-file", filepath.Join(target, "valuable")},
		{"root-link", root},
	} {
		if err := os.Symlink(link.target, filepath.Join(dir, "resources", link.name)); err != nil {
			t.Fatal(err)
		}
	}
	assertOK(t, invoke(t, root, "", "", nil, "delete", "nested", "--yes"))
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("command directory not removed: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(target, "valuable")); err != nil || string(data) != "preserve" {
		t.Fatalf("nested symlink target altered: %q %v", data, err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("nested root symlink removed root: %v", err)
	}
}

func TestNameSafety(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "commands")
	write(t, filepath.Join(parent, "outside", "marker"), "keep", 0644)
	for _, name := range []string{"..", ".", "../outside", "a/b", "/tmp", "space name", "-leading", "name.dot", ""} {
		for _, op := range []string{"create", "delete", "which", "edit"} {
			args := []string{op, name}
			if op == "create" {
				args = append(args, "--runtime", "bash")
			}
			if op == "delete" {
				args = append(args, "--yes")
			}
			assertError(t, invoke(t, root, "", "", nil, args...))
		}
		assertError(t, invoke(t, root, "", "", nil, name))
	}
	for _, name := range []string{"create", "delete", "list", "which", "edit", "help", "version"} {
		assertError(t, invoke(t, root, "", "", nil, "create", name, "--runtime", "bash"))
	}
	data, err := os.ReadFile(filepath.Join(parent, "outside", "marker"))
	if err != nil || string(data) != "keep" {
		t.Fatalf("traversal changed outside files: %q %v", data, err)
	}
}

func TestMissingAndMalformedCommands(t *testing.T) {
	root := t.TempDir()
	assertError(t, invoke(t, root, "", "", nil, "missing"))
	assertError(t, invoke(t, root, "", "", nil, "which", "missing"))
	write(t, filepath.Join(root, "disabled", "run"), "#!/bin/sh\necho bad\n", 0644)
	assertError(t, invoke(t, root, "", "", nil, "disabled"))
	if err := os.MkdirAll(filepath.Join(root, "directory", "run"), 0755); err != nil {
		t.Fatal(err)
	}
	assertError(t, invoke(t, root, "", "", nil, "directory"))
	write(t, filepath.Join(root, "wrong-interpreter", "run"), "#!/qwe/nonexistent/interpreter\n", 0755)
	assertError(t, invoke(t, root, "", "", nil, "wrong-interpreter"))
}

func TestEditorPreferenceAndQuotedArguments(t *testing.T) {
	root := t.TempDir()
	dir := command(t, root, "notes", "exit 0")
	editor := filepath.Join(t.TempDir(), "editor with spaces")
	write(t, editor, "#!/bin/sh\nfor arg do printf '<%s>\\n' \"$arg\"; done\n", 0755)
	visual := "'" + editor + "' --wait 'two words' \"double words\" '$(touch surprise)'"
	r := invoke(t, root, "", "", []string{"VISUAL=" + visual, "EDITOR=/nonexistent/editor"}, "edit", "notes")
	assertOK(t, r)
	want := "<--wait>\n<two words>\n<double words>\n<$(touch surprise)>\n<" + dir + ">\n"
	if r.stdout != want {
		t.Fatalf("editor argv=%q want=%q", r.stdout, want)
	}
	r = invoke(t, root, "", "", []string{"EDITOR='" + editor + "' --fallback"}, "edit", "notes")
	assertOK(t, r)
	if r.stdout != "<--fallback>\n<"+dir+">\n" {
		t.Fatalf("fallback editor args: %q", r.stdout)
	}
	assertError(t, invoke(t, root, "", "", nil, "edit", "notes"))
	assertError(t, invoke(t, root, "", "", []string{"EDITOR='unterminated"}, "edit", "notes"))
}

func TestHelpVersionAndUsageErrors(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"help"}, {"--help"}, {"version"}, {"--version"}} {
		r := invoke(t, root, "", "", nil, args...)
		assertOK(t, r)
		if strings.TrimSpace(r.stdout) == "" {
			t.Fatalf("empty output for %v", args)
		}
	}
	for _, args := range [][]string{{"create"}, {"delete"}, {"which"}, {"edit"}, {"list", "unexpected"}, {"delete", "thing", "--bad"}} {
		assertError(t, invoke(t, root, "", "", nil, args...))
	}
}

func TestDefaultRootUsesHome(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".config", "qwe")
	command(t, root, "home-command", "echo home works")
	// QWE_ROOT is empty so the default root must come from the caller's HOME.
	r := invoke(t, "", "", "", []string{"HOME=" + home}, "home-command")
	assertOK(t, r)
	if r.stdout != "home works\n" {
		t.Fatalf("unexpected default-root output: %+v", r)
	}
}

// Python's standard-library PTY support exercises the actual interactive branch
// without making any third-party package or system utility a test dependency.
func TestInteractiveRuntimeSelection(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable for optional PTY test")
	}
	const driver = `import os, subprocess, sys
master, slave = os.openpty()
proc = subprocess.Popen([sys.argv[1], 'create', sys.argv[3]], stdin=slave, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
os.close(slave)
try:
    os.write(master, sys.argv[2].encode())
    output, errors = proc.communicate(timeout=5)
    sys.stdout.buffer.write(output)
    sys.stderr.buffer.write(errors)
    sys.exit(proc.returncode)
finally:
    if proc.poll() is None:
        proc.kill()
        proc.wait()
    os.close(master)
`
	for _, tc := range []struct{ name, answer, script string }{{"default", "\n", "script.sh"}, {"selected", "python\n", "script.py"}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cmd := exec.Command(python, "-c", driver, binary, tc.answer, tc.name)
			cmd.Env = baseEnv(root)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("interactive create: %v\n%s", err, output)
			}
			if !bytes.Contains(output, []byte("Runtime")) {
				t.Fatalf("missing runtime prompt: %q", output)
			}
			if _, err := os.Stat(filepath.Join(root, tc.name, tc.script)); err != nil {
				t.Fatalf("wrong selected runtime: %v", err)
			}
		})
	}
}
