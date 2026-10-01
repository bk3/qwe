//go:build linux || darwin

package qwe

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

func execute(name string, args []string) error {
	root, dir, err := entrypoint(name)
	if err != nil {
		return err
	}
	env, err := commandEnvironment(root, dir)
	if err != nil {
		return err
	}
	run := filepath.Join(dir, "run")
	return runAttached(run, args, env, fmt.Sprintf("Command %q", name))
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
	return runAttached(path, append(args[1:], dir), os.Environ(), "Editor")
}

func openFolder(program string, names []string) error {
	candidates := []string{program}
	if program == "vim" {
		candidates = []string{"nvim", "vim"}
	}
	var path string
	var err error
	for _, candidate := range candidates {
		path, err = exec.LookPath(candidate)
		if err == nil {
			program = candidate
			break
		}
	}
	if err != nil {
		if program == "vim" {
			return fmt.Errorf("nvim and vim are unavailable on PATH; install Neovim or Vim")
		}
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
		if err := prepareRoot(dir); err != nil {
			return err
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
	return runAttached(path, []string{dir}, os.Environ(), fmt.Sprintf("%s for %s", program, dir))
}

// Keep streams attached directly so interactive commands and pipelines work.
// Forward signals sent specifically to qwe as well as terminal group signals.
func runAttached(path string, args, env []string, description string) error {
	cmd := exec.Command(path, args...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(signals)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", description, err)
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-signals:
				_ = cmd.Process.Signal(sig)
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	close(done)
	if err == nil {
		fmt.Fprintf(os.Stderr, "qwe: %s completed successfully.\n", description)
		return nil
	}
	if exit, ok := err.(*exec.ExitError); ok {
		fmt.Fprintf(os.Stderr, "qwe: %s failed (%s).\n", description, exit.ProcessState)
		return exit
	}
	return fmt.Errorf("wait for %s: %w", description, err)
}
