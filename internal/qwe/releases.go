package qwe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"time"
)

const repository = "bk3/qwe"

type githubReleases struct {
	client *http.Client
	api    string
	useGH  bool
}

func newReleaseSource() *githubReleases {
	return &githubReleases{client: &http.Client{Timeout: 90 * time.Second}, api: "https://api.github.com"}
}

func (s *githubReleases) Latest(ctx context.Context) (release, error) {
	var body bytes.Buffer
	endpoint := "repos/" + repository + "/releases/latest"
	status, err := s.request(ctx, endpoint, "application/vnd.github+json", &body, 1<<20)
	if status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound {
		// GitHub returns 404 for private repositories to anonymous clients.
		// gh handles the user's existing authentication without reading tokens.
		if _, lookupErr := exec.LookPath("gh"); lookupErr != nil {
			return release{}, fmt.Errorf("latest release is unavailable (HTTP %d); for private releases install GitHub CLI and run gh auth login", status)
		}
		body.Reset()
		err = ghDownload(ctx, endpoint, "application/vnd.github+json", &body, 1<<20)
		if err != nil {
			return release{}, fmt.Errorf("check latest release with GitHub CLI: %w; verify gh authentication and repository access", err)
		}
		s.useGH = true
	}
	if err != nil {
		return release{}, fmt.Errorf("check latest qwe release: %w", err)
	}
	var r release
	if err := json.Unmarshal(body.Bytes(), &r); err != nil {
		return release{}, fmt.Errorf("decode latest release: %w", err)
	}
	return r, nil
}

func (s *githubReleases) Download(ctx context.Context, asset releaseAsset, dst io.Writer, limit int64) error {
	endpoint := "repos/" + repository + "/releases/assets/" + strconv.FormatInt(asset.ID, 10)
	if s.useGH {
		return ghDownload(ctx, endpoint, "application/octet-stream", dst, limit)
	}
	_, err := s.request(ctx, endpoint, "application/octet-stream", dst, limit)
	return err
}

func (s *githubReleases) request(ctx context.Context, endpoint, accept string, dst io.Writer, limit int64) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.api+"/"+endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "qwe-upgrade")
	res, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("GitHub request failed")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return res.StatusCode, fmt.Errorf("GitHub returned HTTP %d", res.StatusCode)
	}
	if res.ContentLength > limit {
		return res.StatusCode, fmt.Errorf("release response exceeds size limit")
	}
	n, err := io.Copy(dst, io.LimitReader(res.Body, limit+1))
	if err != nil {
		return res.StatusCode, err
	}
	if n > limit {
		return res.StatusCode, fmt.Errorf("release response exceeds size limit")
	}
	return res.StatusCode, nil
}

type limitedWriter struct {
	dst       io.Writer
	remaining int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("release response exceeds size limit")
	}
	n, err := w.dst.Write(p)
	w.remaining -= int64(n)
	return n, err
}

func ghDownload(ctx context.Context, endpoint, accept string, dst io.Writer, limit int64) error {
	cmd := exec.CommandContext(ctx, "gh", "api", "--hostname", "github.com", endpoint, "-H", "Accept: "+accept)
	cmd.Stdout = &limitedWriter{dst: dst, remaining: limit}
	// Keep credential-bearing redirects and remote error bodies out of errors.
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("GitHub CLI download failed")
	}
	return nil
}
