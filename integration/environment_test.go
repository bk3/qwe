package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedEnvironmentLoadedForExistingCommands(t *testing.T) {
	root, cwd := t.TempDir(), t.TempDir()
	command(t, root, "inspect-env", `printf '<%s>\n' "$QWE_SHARED_TEST" "$QWE_OVERRIDE_TEST" "$QWE_LITERAL_TEST" "$QWE_ROOT" "$QWE_COMMAND_DIR"`)
	write(t, filepath.Join(root, ".env"), "QWE_SHARED_TEST='shared value'\nQWE_OVERRIDE_TEST=file\nQWE_LITERAL_TEST=$(touch surprise)\nQWE_ROOT=wrong\nQWE_COMMAND_DIR=wrong\n", 0600)
	r := invoke(t, root, cwd, "", []string{"QWE_OVERRIDE_TEST=shell"}, "inspect-env")
	assertOK(t, r)
	want := "<shared value>\n<shell>\n<$(touch surprise)>\n<" + root + ">\n<" + filepath.Join(root, "inspect-env") + ">\n"
	if r.stdout != want {
		t.Fatalf("stdout=%q want=%q", r.stdout, want)
	}
	if _, err := os.Stat(filepath.Join(cwd, "surprise")); !os.IsNotExist(err) {
		t.Fatal("env executed shell substitution")
	}
}

func TestMalformedEnvironmentFailsWithoutRunningOrLeakingValues(t *testing.T) {
	root := t.TempDir()
	command(t, root, "do-work", "echo ran")
	for _, content := range []string{"SECRET='private unfinished", "bad-key=private", "SECRET=private\x00"} {
		write(t, filepath.Join(root, ".env"), content, 0600)
		r := invoke(t, root, "", "", nil, "do-work")
		assertError(t, r)
		if r.stdout != "" || strings.Contains(r.stderr, "private") || !strings.Contains(r.stderr, "line 1") {
			t.Fatalf("unexpected error: %+v", r)
		}
	}
}

func TestSilentCommandsAndEditorsReportCompletion(t *testing.T) {
	root := t.TempDir()
	command(t, root, "silent", "exit 0")
	r := invoke(t, root, "", "", nil, "silent")
	assertOK(t, r)
	if r.stdout != "" || !strings.Contains(r.stderr, "completed successfully") {
		t.Fatalf("silent result: %+v", r)
	}
	editor := filepath.Join(t.TempDir(), "silent-editor")
	write(t, editor, "#!/bin/sh\nexit 0\n", 0755)
	r = invoke(t, root, "", "", []string{"EDITOR=" + editor}, "edit", "silent")
	assertOK(t, r)
	if !strings.Contains(r.stderr, "completed successfully") {
		t.Fatalf("editor result: %+v", r)
	}
}

func TestEmptyAndNonRunnableRootsReportNoCommands(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 2; i++ {
		r := invoke(t, root, "", "", nil, "list")
		assertOK(t, r)
		if !strings.Contains(r.stdout, "No runnable commands exist") {
			t.Fatalf("empty output: %+v", r)
		}
		write(t, filepath.Join(root, "disabled", "run"), "#!/bin/sh\n", 0644)
	}
}
