package qwe

import (
	"fmt"
	"os"
)

const help = `qwe — personal scripts, available from any directory

Usage:
  qwe <name> [args...]                 Run a command in the caller's directory
  qwe create <name> --runtime <type>   Create a command (or choose interactively)
  qwe delete <name> [--yes|-y]         Delete a command; default answer is no
  qwe list                            List runnable commands
  qwe which <name>                    Print the command directory
  qwe edit <name>                     Open the directory in VISUAL or EDITOR
  qwe code [name]                     Open the root or command folder using code
  qwe vim [name]                      Open the root or command folder using nvim, falling back to vim
  qwe upgrade                         Upgrade to the latest release after confirmation
  qwe uninstall                       Remove this CLI after confirmation (keep scripts)
  qwe version                         Print the version
  qwe help                            Show this help

Templates: bash, node, ts (npx tsx), go, python.
Root: $QWE_ROOT or ~/.config/qwe. Shared script variables: <root>/.env.
Runtimes are installed separately. Completion messages go to stderr.
`

func Run(args []string, version string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stdout, help)
		return nil
	}
	name, rest := args[0], args[1:]
	if reserved[name] && len(rest) == 1 && (rest[0] == "--help" || rest[0] == "-h") {
		fmt.Fprint(os.Stdout, help)
		return nil
	}
	switch name {
	case "code", "vim":
		if len(rest) > 1 {
			return fmt.Errorf("usage: qwe %s [name]", name)
		}
		return openFolder(name, rest)
	case "uninstall", "upgrade":
		if len(rest) != 0 {
			return fmt.Errorf("usage: qwe %s (confirmation required)", name)
		}
		path, info, err := installedExecutable()
		if err != nil {
			return err
		}
		if name == "uninstall" {
			return uninstall(path, info, os.Stdin, os.Stdout, os.Stderr)
		}
		return upgrade(path, info, version, newReleaseSource(), os.Stdin, os.Stdout, os.Stderr)
	case "help", "--help", "-h":
		if len(rest) != 0 {
			return fmt.Errorf("usage: qwe help")
		}
		fmt.Fprint(os.Stdout, help)
		return nil
	case "version", "--version":
		if len(rest) != 0 {
			return fmt.Errorf("usage: qwe version")
		}
		fmt.Fprintln(os.Stdout, "qwe "+version)
		return nil
	case "create":
		name, runtime, _, err := parseManagement(rest, true)
		if err != nil {
			return err
		}
		return create(name, runtime)
	case "delete":
		name, _, yes, err := parseManagement(rest, false)
		if err != nil {
			return err
		}
		return remove(name, yes)
	case "list":
		if len(rest) != 0 {
			return fmt.Errorf("usage: qwe list")
		}
		return list()
	case "which", "edit":
		if len(rest) != 1 {
			return fmt.Errorf("usage: qwe %s <name>", name)
		}
		_, dir, err := entrypoint(rest[0])
		if err != nil {
			return err
		}
		if name == "edit" {
			return edit(dir)
		}
		fmt.Fprintln(os.Stdout, dir)
		return nil
	default:
		return execute(name, rest)
	}
}

// Built-in flags may precede or follow the name. Script arguments never enter
// this parser and therefore retain their original order and spelling.
func parseManagement(args []string, creating bool) (name, runtime string, yes bool, err error) {
	op := "delete"
	usage := "usage: qwe delete <name> [--yes|-y]"
	if creating {
		op = "create"
		usage = "usage: qwe create <name> [--runtime <type>]"
	}
	runtimeSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case creating && (arg == "--runtime" || arg == "-r"):
			if runtimeSet || i+1 >= len(args) {
				err = fmt.Errorf("%s", usage)
				return
			}
			i++
			runtime = args[i]
			runtimeSet = true
			if runtime == "" {
				err = fmt.Errorf("--runtime requires a value")
				return
			}
		case creating && len(arg) >= 10 && arg[:10] == "--runtime=":
			if runtimeSet {
				err = fmt.Errorf("--runtime may only be specified once")
				return
			}
			runtime = arg[10:]
			runtimeSet = true
			if runtime == "" {
				err = fmt.Errorf("--runtime requires a value")
				return
			}
		case !creating && (arg == "--yes" || arg == "-y"):
			yes = true
		case len(arg) > 0 && arg[0] == '-':
			err = fmt.Errorf("unknown %s option %q; %s", op, arg, usage)
			return
		default:
			if name != "" {
				err = fmt.Errorf("%s", usage)
				return
			}
			name = arg
		}
	}
	if name == "" {
		err = fmt.Errorf("%s", usage)
	}
	return
}
