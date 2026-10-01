package qwe

import (
	"fmt"
	"strings"
	"unicode"
)

// Split editor configuration into words, supporting quotes and escapes without
// invoking a shell, expanding variables, or executing shell substitutions.
func editorArgs(value string) ([]string, error) {
	var args []string
	var word strings.Builder
	var quote rune
	started, escaped := false, false
	for _, c := range value {
		if escaped {
			word.WriteRune(c)
			escaped = false
			started = true
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				word.WriteRune(c)
			}
			continue
		}
		switch {
		case c == '\'' || c == '"':
			quote = c
			started = true
		case unicode.IsSpace(c):
			if started {
				args = append(args, word.String())
				word.Reset()
				started = false
			}
		default:
			word.WriteRune(c)
			started = true
		}
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("invalid editor configuration: unfinished quote or escape")
	}
	if started {
		args = append(args, word.String())
	}
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("editor executable is empty")
	}
	return args, nil
}
