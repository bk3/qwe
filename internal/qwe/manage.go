package qwe

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func create(name, runtime string) (err error) {
	root, dir, err := commandPath(name)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(dir); err == nil {
		return fmt.Errorf("command %q already exists at %s", name, dir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect command: %w", err)
	}
	if runtime == "" {
		if !isTerminal(os.Stdin.Fd()) {
			return fmt.Errorf("specify a runtime for non-interactive creation: qwe create %s --runtime bash", name)
		}
		fmt.Fprint(os.Stderr, "Runtime [bash/node/ts/go/python] (default bash): ")
		answer, readErr := bufio.NewReader(io.LimitReader(os.Stdin, 4096)).ReadString('\n')
		if readErr != nil && !(readErr == io.EOF && len(answer) > 0) {
			return fmt.Errorf("read runtime selection; use --runtime for non-interactive creation")
		}
		runtime = strings.TrimSpace(answer)
		if runtime == "" {
			runtime = "bash"
		}
	}
	files, err := Scaffold(runtime)
	if err != nil {
		return err
	}
	if err = prepareRoot(root); err != nil {
		return err
	}
	if err = os.Mkdir(dir, 0755); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("command %q already exists at %s", name, dir)
		}
		return fmt.Errorf("create command directory: %w", err)
	}
	defer func() {
		if err != nil {
			if cleanupErr := os.RemoveAll(dir); cleanupErr != nil {
				err = fmt.Errorf("%w; remove partial scaffold: %v", err, cleanupErr)
			}
		}
	}()
	for _, file := range files {
		path := filepath.Join(dir, file.Name)
		f, openErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, file.Mode)
		if openErr != nil {
			return fmt.Errorf("create scaffold: %w", openErr)
		}
		_, writeErr := io.WriteString(f, file.Content)
		closeErr := f.Close()
		if writeErr != nil {
			return fmt.Errorf("write scaffold: %w", writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close scaffold: %w", closeErr)
		}
		// Preserve executable file permissions even with a restrictive file umask.
		if err = os.Chmod(path, file.Mode); err != nil {
			return fmt.Errorf("set scaffold permissions: %w", err)
		}
	}
	fmt.Fprintf(os.Stdout, "Created command %q at %s\nRun it with: qwe %s\n", name, dir, name)
	return nil
}

func remove(name string, yes bool) error {
	_, dir, err := commandPath(name)
	if err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return fmt.Errorf("command %q does not exist", name)
	}
	if err != nil {
		return fmt.Errorf("inspect command: %w", err)
	}
	if !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("command %q is not a directory", name)
	}
	if !yes {
		fmt.Fprintf(os.Stderr, "Delete command %q?\nThis will remove: %s\nCommand symlinks are unlinked; their targets are kept.\n[y/N]: ", name, dir)
		answer, readErr := bufio.NewReader(io.LimitReader(os.Stdin, 4096)).ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return fmt.Errorf("read confirmation: %w", readErr)
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(os.Stdout, "Cancelled.")
			return nil
		}
	}
	// RemoveAll never follows symlinks, including nested ones. In particular,
	// never EvalSymlinks(dir) here: the target can be outside the root.
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("delete command: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Deleted command %q.\n", name)
	return nil
}

func list() error {
	root, err := Root()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		fmt.Fprintln(os.Stdout, "No runnable commands exist. Create one with: qwe create <name> --runtime bash")
		return nil
	}
	if err != nil {
		return fmt.Errorf("read command root: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, item := range entries {
		if _, _, err := entrypoint(item.Name()); err == nil {
			names = append(names, item.Name())
		}
	}
	if len(names) == 0 {
		fmt.Fprintln(os.Stdout, "No runnable commands exist. Create one with: qwe create <name> --runtime bash")
		return nil
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintln(os.Stdout, name)
	}
	return nil
}
