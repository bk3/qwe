package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallRemovesOnlyRunningCLI(t *testing.T) {
	root := t.TempDir()
	command(t, root, "keep", "echo precious")
	installed := filepath.Join(t.TempDir(), "qwe")
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, data, 0755); err != nil {
		t.Fatal(err)
	}
	for _, answer := range []string{"\n", "yes\n"} {
		cmd := exec.Command(installed, "uninstall")
		cmd.Env = baseEnv(root)
		cmd.Stdin = strings.NewReader(answer)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("uninstall: %v\n%s", err, out.String())
		}
		if !strings.Contains(out.String(), physicalPath(t, filepath.Dir(installed))) {
			t.Fatalf("prompt omitted executable location: %s", out.String())
		}
		_, err := os.Stat(installed)
		if answer == "\n" && err != nil {
			t.Fatal("cancelled uninstall removed binary")
		}
		if answer == "yes\n" && !os.IsNotExist(err) {
			t.Fatalf("confirmed uninstall kept binary: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "keep", "run")); err != nil {
			t.Fatal("uninstall changed commands")
		}
	}
}

func TestLifecycleCommandsRequirePromptAndReserveNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"upgrade", "uninstall"} {
		assertError(t, invoke(t, root, "", "", nil, "create", name, "--runtime", "bash"))
		for _, flag := range []string{"--yes", "-y", "extra"} {
			assertError(t, invoke(t, root, "", "", nil, name, flag))
		}
		assertOK(t, invoke(t, root, "", "", nil, name, "--help"))
	}
}
