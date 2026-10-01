package qwe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareRootPreservesConfigurationAndIgnoresSecrets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".config", "qwe")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	ignore := filepath.Join(home, ".config", ".gitignore")
	if err := os.WriteFile(ignore, []byte("existing-rule"), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := prepareRoot(root); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(ignore)
	if err != nil || string(data) != "existing-rule\n/qwe/.env\n" {
		t.Fatalf("ignore=%q err=%v", data, err)
	}
	path := filepath.Join(root, ".env")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("env permissions: %v %v", info, err)
	}
	if err := os.WriteFile(path, []byte("SECRET=keep-me\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareRoot(root); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "SECRET=keep-me\n" {
		t.Fatalf("env overwritten: %q %v", data, err)
	}
	data, err = os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil || string(data) != "/.env\n" {
		t.Fatalf("root ignore: %q %v", data, err)
	}
}

func TestEnvironmentValues(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"", ""}, {"# empty value with comment", ""}, {"plain value # comment", "plain value"}, {"a#b", "a#b"},
		{"' literal $HOME # value ' # comment", " literal $HOME # value "},
		{`"hello\nworld"`, "hello\nworld"}, {`"$(touch surprise)"`, "$(touch surprise)"},
	} {
		got, err := environmentValue(tc.raw)
		if err != nil || got != tc.want {
			t.Errorf("%q: got %q, %v; want %q", tc.raw, got, err, tc.want)
		}
	}
	for _, raw := range []string{"'unfinished", `"unfinished`, `"value" trailing`, `"bad\xZZ"`, `"\x00"`} {
		if _, err := environmentValue(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}

func TestCommandEnvironmentPrecedenceAndMissingFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("QWE_ENV_TEST_OVERRIDE", "shell")
	t.Setenv("QWE_ENV_TEST_EMPTY", "")
	content := "export QWE_ENV_TEST_VALUE='shared value'\nQWE_ENV_TEST_OVERRIDE=file\nQWE_ENV_TEST_EMPTY=file\nQWE_ROOT=wrong\nQWE_COMMAND_DIR=wrong\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	env, err := commandEnvironment(root, root+"/script")
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	for key, want := range map[string]string{"QWE_ENV_TEST_VALUE": "shared value", "QWE_ENV_TEST_OVERRIDE": "shell", "QWE_ENV_TEST_EMPTY": "", "QWE_ROOT": root, "QWE_COMMAND_DIR": root + "/script"} {
		if values[key] != want {
			t.Errorf("%s=%q want=%q", key, values[key], want)
		}
	}
	if _, err := commandEnvironment(t.TempDir(), "/script"); err != nil {
		t.Fatalf("missing optional env: %v", err)
	}
}
