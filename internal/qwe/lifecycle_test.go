package qwe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func upgradeFixture(t *testing.T, corrupt, wrongVersion bool) (*githubReleases, *int) {
	t.Helper()
	asset := "qwe-" + runtime.GOOS + "-" + runtime.GOARCH
	version := "v0.0.2"
	if wrongVersion {
		version = "v0.0.3"
	}
	binary := []byte("#!/bin/sh\nprintf 'qwe " + version + "\\n'\n")
	digest := sha256.Sum256(binary)
	if corrupt {
		digest[0] ^= 0xff
	}
	manifest := fmt.Sprintf("%x  %s\n", digest, asset)
	r := release{Tag: "v0.0.2", Assets: []releaseAsset{{ID: 1, Name: asset}, {ID: 2, Name: "checksums.txt"}}}
	downloads := new(int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/repos/bk3/qwe/releases/latest":
			json.NewEncoder(w).Encode(r)
		case "/repos/bk3/qwe/releases/assets/1":
			*downloads++
			w.Write(binary)
		case "/repos/bk3/qwe/releases/assets/2":
			*downloads++
			fmt.Fprint(w, manifest)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(server.Close)
	return &githubReleases{client: server.Client(), api: server.URL}, downloads
}

func installedFixture(t *testing.T) (string, os.FileInfo) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "custom install", "qwe")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("original binary"), 0755); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, info
}

func TestUpgradeAtomicallyReplacesConfirmedInstallation(t *testing.T) {
	source, downloads := upgradeFixture(t, false, false)
	path, info := installedFixture(t)
	var out, prompt bytes.Buffer
	if err := upgrade(path, info, "v0.0.1", source, strings.NewReader("YES\n"), &out, &prompt); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "v0.0.2") {
		t.Fatalf("upgrade data=%q err=%v", data, err)
	}
	if *downloads != 2 || !strings.Contains(prompt.String(), path) || !strings.Contains(prompt.String(), "v0.0.1 to v0.0.2") {
		t.Fatalf("downloads=%d prompt=%s", *downloads, prompt.String())
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("staging files leaked: %v", entries)
	}
	mode, _ := os.Stat(path)
	if mode.Mode().Perm() != 0755 {
		t.Fatalf("mode=%v", mode.Mode())
	}
}

func TestUpgradeCancellationAndAlreadyCurrentNeverDownload(t *testing.T) {
	for _, tc := range []struct{ version, answer string }{{"v0.0.1", ""}, {"v0.0.1", "n\n"}, {"v0.0.1", "maybe\n"}, {"v0.0.2", "yes\n"}, {"v0.0.3", "yes\n"}} {
		source, downloads := upgradeFixture(t, false, false)
		path, info := installedFixture(t)
		var out, prompt bytes.Buffer
		if err := upgrade(path, info, tc.version, source, strings.NewReader(tc.answer), &out, &prompt); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		if *downloads != 0 || string(data) != "original binary" {
			t.Fatalf("changed installation without upgrade: %q", data)
		}
		if tc.version != "v0.0.1" && prompt.Len() != 0 {
			t.Fatal("prompted when already current")
		}
	}
}

func TestFailedUpgradePreservesOriginalAndCleansStage(t *testing.T) {
	for _, tc := range []struct {
		name           string
		corrupt, wrong bool
	}{{"checksum", true, false}, {"version", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			source, _ := upgradeFixture(t, tc.corrupt, tc.wrong)
			path, info := installedFixture(t)
			err := upgrade(path, info, "v0.0.1", source, strings.NewReader("yes\n"), io.Discard, io.Discard)
			if err == nil {
				t.Fatal("accepted broken release")
			}
			data, _ := os.ReadFile(path)
			if string(data) != "original binary" {
				t.Fatalf("original changed: %q", data)
			}
			entries, _ := os.ReadDir(filepath.Dir(path))
			if len(entries) != 1 {
				t.Fatalf("staging files leaked: %v", entries)
			}
		})
	}
}

type interruptedSource struct{ releaseSource }

func (s interruptedSource) Download(ctx context.Context, asset releaseAsset, dst io.Writer, limit int64) error {
	if asset.Name != "checksums.txt" {
		io.WriteString(dst, "incomplete binary")
		return fmt.Errorf("connection interrupted")
	}
	return s.releaseSource.Download(ctx, asset, dst, limit)
}

func TestInterruptedUpgradeKeepsInstallation(t *testing.T) {
	source, _ := upgradeFixture(t, false, false)
	path, info := installedFixture(t)
	err := upgrade(path, info, "v0.0.1", interruptedSource{source}, strings.NewReader("yes\n"), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("accepted incomplete download")
	}
	data, _ := os.ReadFile(path)
	entries, _ := os.ReadDir(filepath.Dir(path))
	if string(data) != "original binary" || len(entries) != 1 {
		t.Fatalf("failed upgrade changed installation or leaked stage: data=%q entries=%v", data, entries)
	}
}

func TestVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		newer           bool
	}{
		{"v0.0.2", "v0.0.1", true}, {"v1.0.0", "v0.99.999", true},
		{"v0.10.0", "v0.9.999", true}, {"v0.9.0", "v0.10.0", false},
		{"dev", "v0.0.1", false}, {"v0.0.1", "v0.0.1", false},
	} {
		if got := newerVersion(tc.current, tc.latest); got != tc.newer {
			t.Errorf("%s > %s = %v", tc.current, tc.latest, got)
		}
	}
}

func TestUninstallConfirmationAndConcurrentReplacement(t *testing.T) {
	path, info := installedFixture(t)
	for _, answer := range []string{"", "no\n", "yesterday\n"} {
		if err := uninstall(path, info, strings.NewReader(answer), io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal("cancelled uninstall removed binary")
		}
	}
	if err := uninstall(path, info, strings.NewReader("y\n"), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("binary still exists: %v", err)
	}
	path, info = installedFixture(t)
	other := path + ".new"
	os.WriteFile(other, []byte("another installation"), 0755)
	os.Rename(other, path)
	if err := uninstall(path, info, strings.NewReader("yes\n"), io.Discard, io.Discard); err == nil {
		t.Fatal("deleted replacement installation")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "another installation" {
		t.Fatalf("replacement changed: %q", data)
	}
}

func TestChecksumRejectsMissingMalformedAndDuplicate(t *testing.T) {
	valid := strings.Repeat("ab", 32)
	for _, input := range []string{"", valid + " other", "not-a-checksum qwe-linux-amd64", valid + " qwe-linux-amd64\n" + valid + " qwe-linux-amd64"} {
		if _, err := checksum([]byte(input), "qwe-linux-amd64"); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}

func TestHTTPDownloadLimitsAndErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			http.Error(w, "sensitive remote detail", 500)
			return
		}
		fmt.Fprint(w, "larger than limit")
	}))
	defer server.Close()
	source := &githubReleases{client: server.Client(), api: server.URL}
	if _, err := source.request(context.Background(), "big", "application/octet-stream", io.Discard, 4); err == nil {
		t.Fatal("accepted oversized download")
	}
	if _, err := source.request(context.Background(), "error", "application/json", io.Discard, 100); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("unexpected error=%v", err)
	}
}

func TestPrivateReleaseUsesAuthenticatedGitHubCLI(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	dir := t.TempDir()
	gh := `#!/bin/sh
case "$*" in
  *releases/latest*) printf '{"tag_name":"v0.0.2","assets":[]}' ;;
  *releases/assets/7*) printf 'private asset' ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(gh), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	source := &githubReleases{client: server.Client(), api: server.URL}
	r, err := source.Latest(context.Background())
	if err != nil || r.Tag != "v0.0.2" || !source.useGH {
		t.Fatalf("release=%v err=%v", r, err)
	}
	var downloaded bytes.Buffer
	if err := source.Download(context.Background(), releaseAsset{ID: 7}, &downloaded, 100); err != nil {
		t.Fatal(err)
	}
	if downloaded.String() != "private asset" {
		t.Fatalf("private download=%q", downloaded.String())
	}
	if err := source.Download(context.Background(), releaseAsset{ID: 7}, io.Discard, 4); err == nil {
		t.Fatal("gh accepted oversized download")
	}
}
