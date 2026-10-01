package qwe

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

func installedExecutable() (string, os.FileInfo, error) {
	path, err := os.Executable()
	if err != nil {
		return "", nil, fmt.Errorf("locate installed qwe: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", nil, fmt.Errorf("resolve installed qwe: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, fmt.Errorf("inspect installed qwe: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("installed qwe is not a regular file")
	}
	return path, info, nil
}

func confirm(input io.Reader, prompt io.Writer, message string) (bool, error) {
	fmt.Fprint(prompt, message+"\n[y/N]: ")
	answer, err := bufio.NewReader(io.LimitReader(input, 4097)).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	if len(answer) > 4096 {
		return false, nil
	}
	answer = strings.TrimSpace(answer)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

func unchanged(path string, original os.FileInfo) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect installed qwe before changing it: %w", err)
	}
	if !info.Mode().IsRegular() || !os.SameFile(info, original) {
		return fmt.Errorf("installed qwe changed during this operation; retry")
	}
	return nil
}

func uninstall(path string, info os.FileInfo, input io.Reader, out, prompt io.Writer) error {
	ok, err := confirm(input, prompt, fmt.Sprintf("Uninstall qwe?\nThis will remove the CLI binary: %s\nYour personal commands and shell configuration will be kept.", path))
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(out, "Cancelled.")
		return nil
	}
	if err := unchanged(path, info); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("uninstall qwe: %w", err)
	}
	fmt.Fprintf(out, "Uninstalled qwe from %s. Your personal commands were kept.\n", path)
	return nil
}

type releaseAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type release struct {
	Tag        string         `json:"tag_name"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

type releaseSource interface {
	Latest(context.Context) (release, error)
	Download(context.Context, releaseAsset, io.Writer, int64) error
}

var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func newerVersion(current, latest string) bool {
	if !releaseTag.MatchString(current) {
		return false
	}
	a, b := strings.Split(current[1:], "."), strings.Split(latest[1:], ".")
	for i := range a {
		x, y := strings.TrimLeft(a[i], "0"), strings.TrimLeft(b[i], "0")
		if len(x) != len(y) {
			return len(x) > len(y)
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func upgrade(path string, info os.FileInfo, version string, source releaseSource, input io.Reader, out, prompt io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	fmt.Fprintln(out, "Checking the latest qwe release...")
	r, err := source.Latest(ctx)
	if err != nil {
		return err
	}
	if r.Draft || r.Prerelease || !releaseTag.MatchString(r.Tag) {
		return fmt.Errorf("latest release has an invalid or unstable version %q", r.Tag)
	}
	if version == r.Tag {
		fmt.Fprintf(out, "qwe %s is already up to date.\n", version)
		return nil
	}
	if newerVersion(version, r.Tag) {
		fmt.Fprintf(out, "Installed qwe %s is newer than latest release %s; no upgrade needed.\n", version, r.Tag)
		return nil
	}
	assetName := "qwe-" + runtime.GOOS + "-" + runtime.GOARCH
	var binary, checksums releaseAsset
	for _, asset := range r.Assets {
		switch asset.Name {
		case assetName:
			if binary.ID != 0 {
				return fmt.Errorf("release contains duplicate %s assets", assetName)
			}
			binary = asset
		case "checksums.txt":
			if checksums.ID != 0 {
				return fmt.Errorf("release contains duplicate checksum assets")
			}
			checksums = asset
		}
	}
	if binary.ID <= 0 || checksums.ID <= 0 {
		return fmt.Errorf("release %s is missing %s or checksums.txt", r.Tag, assetName)
	}
	ok, err := confirm(input, prompt, fmt.Sprintf("Upgrade qwe from %s to %s?\nThis will replace the CLI binary: %s\nYour personal commands will be kept.", version, r.Tag, path))
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(out, "Cancelled.")
		return nil
	}
	// Give downloads their own deadline: time spent at the prompt is unbounded.
	cancel()
	ctx, cancelDownloads := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelDownloads()
	var manifest bytes.Buffer
	if err := source.Download(ctx, checksums, &manifest, 1<<20); err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	expected, err := checksum(manifest.Bytes(), assetName)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".qwe-upgrade-*")
	if err != nil {
		return fmt.Errorf("stage upgrade beside installed qwe: %w", err)
	}
	stage := f.Name()
	defer os.Remove(stage)
	defer f.Close()
	hash := sha256.New()
	if err := source.Download(ctx, binary, io.MultiWriter(f, hash), 128<<20); err != nil {
		return fmt.Errorf("download qwe: %w", err)
	}
	if !bytes.Equal(expected, hash.Sum(nil)) {
		return fmt.Errorf("checksum mismatch; installed qwe was preserved")
	}
	if err := f.Chmod(0755); err != nil {
		return fmt.Errorf("set upgraded binary permissions: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync upgraded binary: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close upgraded binary: %w", err)
	}
	verifyCtx, cancelVerify := context.WithTimeout(ctx, 10*time.Second)
	defer cancelVerify()
	verified, err := exec.CommandContext(verifyCtx, stage, "--version").Output()
	if err != nil || strings.TrimSpace(string(verified)) != "qwe "+r.Tag {
		return fmt.Errorf("downloaded binary failed version verification; installed qwe was preserved")
	}
	if err := unchanged(path, info); err != nil {
		return err
	}
	if err := os.Rename(stage, path); err != nil {
		return fmt.Errorf("replace installed qwe: %w", err)
	}
	fmt.Fprintf(out, "Upgraded qwe to %s at %s.\n", r.Tag, path)
	return nil
}

func checksum(manifest []byte, asset string) ([]byte, error) {
	var result []byte
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != asset {
			continue
		}
		if result != nil {
			return nil, fmt.Errorf("duplicate checksum for %s", asset)
		}
		decoded, err := hex.DecodeString(fields[0])
		if err != nil || len(decoded) != sha256.Size {
			return nil, fmt.Errorf("invalid checksum for %s", asset)
		}
		result = decoded
	}
	if result == nil {
		return nil, fmt.Errorf("release checksum is missing for %s", asset)
	}
	return result, nil
}
