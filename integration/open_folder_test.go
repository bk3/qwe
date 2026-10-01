package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectEditorsOpenRootAndCommandFolders(t *testing.T) {
	for _, editor := range []string{"code", "vim"} {
		t.Run(editor, func(t *testing.T) {
			tools := filepath.Join(t.TempDir(), "tools with spaces")
			write(t, filepath.Join(tools, editor), "#!/bin/sh\nprintf 'count=%s\\n' \"$#\"\nprintf 'folder=<%s>\\n' \"$1\"\nprintf 'cwd=%s\\n' \"$PWD\"\n", 0755)
			root := filepath.Join(t.TempDir(), "qwe root with spaces")
			cwd := t.TempDir()
			env := []string{"PATH=" + tools, "VISUAL=/nonexistent/visual", "EDITOR=/nonexistent/editor"}
			r := invoke(t, root, cwd, "", env, editor)
			assertOK(t, r)
			want := "count=1\nfolder=<" + root + ">\ncwd=" + physicalPath(t, cwd) + "\n"
			if r.stdout != want {
				t.Fatalf("root editor output=%q want=%q", r.stdout, want)
			}
			folder := filepath.Join(root, "repair-me")
			if err := os.Mkdir(folder, 0755); err != nil {
				t.Fatal(err)
			}
			// No run file: opening the folder must still permit repairing it.
			r = invoke(t, root, cwd, "", env, editor, "repair-me")
			assertOK(t, r)
			want = "count=1\nfolder=<" + folder + ">\ncwd=" + physicalPath(t, cwd) + "\n"
			if r.stdout != want {
				t.Fatalf("command editor output=%q want=%q", r.stdout, want)
			}
			for _, args := range [][]string{{editor, "missing"}, {editor, "../outside"}, {editor, "one", "two"}, {editor, ""}} {
				assertError(t, invoke(t, root, cwd, "", env, args...))
			}
			if _, err := os.Stat(filepath.Join(root, "missing")); !os.IsNotExist(err) {
				t.Fatal("opening missing command created it")
			}
			assertError(t, invoke(t, root, cwd, "", nil, "create", editor, "--runtime", "bash"))
		})
	}
}

func TestMissingDirectEditorsFailWithoutCreatingRoot(t *testing.T) {
	for _, editor := range []string{"code", "vim"} {
		root := filepath.Join(t.TempDir(), "missing-root")
		r := invoke(t, root, "", "", []string{"PATH=" + t.TempDir()}, editor)
		assertError(t, r)
		if !strings.Contains(r.stderr, "unavailable on PATH") {
			t.Fatalf("missing executable error: %q", r.stderr)
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("missing executable created root")
		}
	}
}

func TestDirectEditorsDefaultRootSymlinksAndExitStatus(t *testing.T) {
	for _, editor := range []string{"code", "vim"} {
		tools := t.TempDir()
		write(t, filepath.Join(tools, editor), "#!/bin/sh\nprintf '%s\\n' \"$1\"\nexit 37\n", 0755)
		home := t.TempDir()
		env := []string{"PATH=" + tools, "HOME=" + home}
		r := invoke(t, "", "", "", env, editor)
		want := filepath.Join(home, ".config", "qwe")
		if r.code != 37 || r.stdout != want+"\n" {
			t.Fatalf("default root/exit: %+v", r)
		}
		linked := filepath.Join(t.TempDir(), "dotfiles")
		if err := os.Symlink(want, linked); err != nil {
			t.Fatal(err)
		}
		r = invoke(t, linked, "", "", env, editor)
		if r.code != 37 || r.stdout != linked+"\n" {
			t.Fatalf("symlink root/exit: %+v", r)
		}
	}
}

func TestVimPrefersNeovimAndDoesNotFallbackOnFailure(t *testing.T) {
	for _, available := range []string{"both", "nvim-only"} {
		tools, root := t.TempDir(), t.TempDir()
		write(t, filepath.Join(tools, "nvim"), "#!/bin/sh\necho neovim\nexit 37\n", 0755)
		if available == "both" {
			write(t, filepath.Join(tools, "vim"), "#!/bin/sh\necho vim\n", 0755)
		}
		for _, args := range [][]string{{"vim"}, {"vim", "repair-me"}} {
			if err := os.MkdirAll(filepath.Join(root, "repair-me"), 0755); err != nil {
				t.Fatal(err)
			}
			r := invoke(t, root, "", "", []string{"PATH=" + tools}, args...)
			if r.code != 37 || r.stdout != "neovim\n" || !strings.Contains(r.stderr, "failed") {
				t.Fatalf("%s %v: %+v", available, args, r)
			}
		}
	}
}
