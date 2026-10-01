//go:build linux || darwin

package qwe

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func execute(name string, args []string) error {
	root, dir, err := entrypoint(name)
	if err != nil {
		return err
	}
	env := make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "QWE_ROOT=") && !strings.HasPrefix(value, "QWE_COMMAND_DIR=") {
			env = append(env, value)
		}
	}
	env = append(env, "QWE_ROOT="+root, "QWE_COMMAND_DIR="+dir)
	run := filepath.Join(dir, "run")
	if err := syscall.Exec(run, append([]string{run}, args...), env); err != nil {
		return fmt.Errorf("execute command %q: %w (entrypoint needs a valid shebang or executable binary)", name, err)
	}
	return nil
}

func edit(dir string) error {
	editor := os.Getenv("VISUAL")
	if strings.TrimSpace(editor) == "" {
		editor = os.Getenv("EDITOR")
	}
	if strings.TrimSpace(editor) == "" {
		return fmt.Errorf("set VISUAL or EDITOR to open a command directory")
	}
	args, err := editorArgs(editor)
	if err != nil {
		return err
	}
	path, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("find editor %q: %w", args[0], err)
	}
	if err := syscall.Exec(path, append(args, dir), os.Environ()); err != nil {
		return fmt.Errorf("open editor: %w", err)
	}
	return nil
}

func openFolder(program string, names []string) error {
	path, err := exec.LookPath(program)
	if err != nil {
		return fmt.Errorf("%s is unavailable on PATH; install it or enable its shell command", program)
	}
	var dir string
	if len(names) == 0 {
		dir, err = Root()
		if err != nil {
			return err
		}
		// Opening the root also works before the first command is created. Check
		// editor availability first so a missing editor causes no filesystem writes.
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("prepare qwe root: %w", err)
		}
	} else {
		_, dir, err = commandPath(names[0])
		if err != nil {
			return err
		}
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			return fmt.Errorf("command %q does not exist", names[0])
		}
		if err != nil {
			return fmt.Errorf("inspect command folder: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("command %q is not a directory", names[0])
		}
		// Opening a command must work even when run is broken or missing, so the
		// user can repair it. Execution validation is intentionally unnecessary.
	}
	if err := syscall.Exec(path, []string{program, dir}, os.Environ()); err != nil {
		return fmt.Errorf("open folder with %s: %w", program, err)
	}
	return nil
}
