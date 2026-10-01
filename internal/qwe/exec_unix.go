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
