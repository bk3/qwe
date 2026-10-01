package qwe

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRootResolution(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("QWE_ROOT", "")
	root, err := Root()
	if err != nil || root != filepath.Join(home, ".config", "qwe") {
		t.Fatalf("default root = %q, %v", root, err)
	}
	t.Setenv("QWE_ROOT", "relative/../commands")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err = Root()
	if err != nil || root != filepath.Join(cwd, "commands") {
		t.Fatalf("relative root = %q, %v", root, err)
	}
}

func TestEditorWords(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []string
	}{
		{`code --wait`, []string{"code", "--wait"}},
		{`'/path with spaces/editor' "two words" ''`, []string{"/path with spaces/editor", "two words", ""}},
		{`editor escaped\ space 'literal\backslash'`, []string{"editor", "escaped space", `literal\backslash`}},
		{`editor '$HOME' '$(echo danger)'`, []string{"editor", "$HOME", "$(echo danger)"}},
	} {
		got, err := editorArgs(tc.input)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("editorArgs(%q) = %#v, %v; want %#v", tc.input, got, err, tc.want)
		}
	}
	for _, input := range []string{"", "  ", `'' --wait`, `editor 'unclosed`, "editor \\"} {
		if _, err := editorArgs(input); err == nil {
			t.Errorf("accepted invalid configuration %q", input)
		}
	}
}
