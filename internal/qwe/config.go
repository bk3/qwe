package qwe

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
var reserved = map[string]bool{"create": true, "delete": true, "list": true, "edit": true, "which": true, "help": true, "version": true, "uninstall": true, "upgrade": true, "code": true, "vim": true}

func ValidateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid command name %q (use letters, digits, underscores, or hyphens)", name)
	}
	if reserved[name] {
		return fmt.Errorf("%q is reserved and cannot be used as a command name", name)
	}
	return nil
}

func Root() (string, error) {
	root := os.Getenv("QWE_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
		root = filepath.Join(home, ".config", "qwe")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve command root: %w", err)
	}
	return filepath.Clean(root), nil
}

func commandPath(name string) (string, string, error) {
	if err := ValidateName(name); err != nil {
		return "", "", err
	}
	root, err := Root()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(root, name)
	// Defense in depth for destructive operations: only a lexical direct child.
	// Do not resolve a command symlink, since deletion must unlink it instead.
	if filepath.Dir(dir) != root || dir == root {
		return "", "", fmt.Errorf("command path is not a direct child of the root")
	}
	return root, dir, nil
}

func entrypoint(name string) (string, string, error) {
	root, dir, err := commandPath(name)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return "", "", fmt.Errorf("command %q does not exist", name)
	}
	if err != nil {
		return "", "", fmt.Errorf("inspect command %q: %w", name, err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("command %q is not a directory", name)
	}
	run := filepath.Join(dir, "run")
	info, err = os.Stat(run)
	if os.IsNotExist(err) {
		return "", "", fmt.Errorf("command %q is missing its run entrypoint", name)
	}
	if err != nil {
		return "", "", fmt.Errorf("inspect entrypoint: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("command entrypoint is not a regular file: %s", run)
	}
	if info.Mode().Perm()&0111 == 0 {
		return "", "", fmt.Errorf("command entrypoint is not executable; run chmod +x %s", run)
	}
	return root, dir, nil
}
