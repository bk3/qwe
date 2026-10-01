package qwe

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var environmentKey = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Prepare shared configuration without replacing the user's files.
func prepareRoot(root string) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return fmt.Errorf("prepare qwe root: %w", err)
	}
	path := filepath.Join(root, ".env")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		_, err = f.WriteString("# Shared environment for every qwe script. Existing shell values take precedence.\n# EXAMPLE_KEY=your-value\n")
		closeErr := f.Close()
		if err != nil {
			return fmt.Errorf("write shared environment: %w", err)
		}
		if closeErr != nil {
			return fmt.Errorf("close shared environment: %w", closeErr)
		}
	} else if !os.IsExist(err) {
		return fmt.Errorf("create shared environment: %w", err)
	}
	// Keep the user's .config ignore rule alongside other local configuration.
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	if root == filepath.Join(home, ".config", "qwe") {
		if err := appendIgnore(filepath.Join(home, ".config", ".gitignore"), "/qwe/.env"); err != nil {
			return err
		}
	}
	// Also ignore at the root so custom roots and symlinked dotfiles are covered.
	return appendIgnore(filepath.Join(root, ".gitignore"), "/.env")
}

func appendIgnore(ignore, pattern string) error {
	data, err := os.ReadFile(ignore)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read qwe gitignore: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}
	f, err := os.OpenFile(ignore, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open qwe gitignore: %w", err)
	}
	rule := pattern + "\n"
	if len(data) > 0 && data[len(data)-1] != '\n' {
		rule = "\n" + rule
	}
	_, err = f.WriteString(rule)
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("ignore shared environment: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close qwe gitignore: %w", closeErr)
	}
	return nil
}

// Values are data: never source a shell or evaluate substitutions.
func commandEnvironment(root, dir string) ([]string, error) {
	values := map[string]string{}
	f, err := os.Open(filepath.Join(root, ".env"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read shared environment: %w", err)
	}
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "export ") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
			}
			key, raw, ok := strings.Cut(line, "=")
			key = strings.TrimSpace(key)
			if !ok || !environmentKey.MatchString(key) || strings.ContainsRune(raw, 0) {
				return nil, fmt.Errorf("invalid .env assignment on line %d", lineNumber)
			}
			value, err := environmentValue(strings.TrimSpace(raw))
			if err != nil {
				return nil, fmt.Errorf("invalid .env value on line %d: %w", lineNumber, err)
			}
			values[key] = value
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("read shared environment: %w", err)
		}
	}
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	values["QWE_ROOT"], values["QWE_COMMAND_DIR"] = root, dir
	env := make([]string, 0, len(values))
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env, nil
}

func environmentValue(raw string) (string, error) {
	if raw == "" || strings.HasPrefix(raw, "#") {
		return "", nil
	}
	if raw[0] != '\'' && raw[0] != '"' {
		for i := 1; i < len(raw); i++ {
			if raw[i] == '#' && (raw[i-1] == ' ' || raw[i-1] == '\t') {
				return strings.TrimSpace(raw[:i]), nil
			}
		}
		return raw, nil
	}
	quote := raw[0]
	for i := 1; i < len(raw); i++ {
		if quote == '"' && raw[i] == '\\' {
			i++
			continue
		}
		if raw[i] != quote {
			continue
		}
		trailing := strings.TrimSpace(raw[i+1:])
		if trailing != "" && !strings.HasPrefix(trailing, "#") {
			return "", fmt.Errorf("unexpected text after quoted value")
		}
		if quote == '\'' {
			return raw[1:i], nil
		}
		value, err := strconv.Unquote(raw[:i+1])
		if err != nil {
			return "", fmt.Errorf("invalid quoted value")
		}
		if strings.ContainsRune(value, 0) {
			return "", fmt.Errorf("value contains a null byte")
		}
		return value, nil
	}
	return "", fmt.Errorf("unfinished quoted value")
}
