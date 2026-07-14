package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidRepoURL = errors.New("invalid repository URL")
	ErrCloneTimeout   = errors.New("git clone timed out")
)

// repoPathPattern matches "/owner/repo" or "/owner/repo.git" with an optional
// trailing slash, and nothing else (no path traversal, no extra segments).
var repoPathPattern = regexp.MustCompile(`^/[^/]+/[^/]+(\.git)?/?$`)

// shaPattern guards against corrupted `git rev-parse` output.
var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type CloneOptions struct {
	RepoURL     string
	Branch      string // empty = default branch
	GithubToken string // empty = public repo
}

type Workspace struct {
	Dir       string
	CommitSHA string
	Cleanup   func()
}

// validateRepoURL fails fast on anything that isn't a public github.com HTTPS repo.
func validateRepoURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRepoURL, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be https", ErrInvalidRepoURL)
	}
	if u.Host != "github.com" {
		return fmt.Errorf("%w: host must be github.com", ErrInvalidRepoURL)
	}
	if !repoPathPattern.MatchString(u.Path) {
		return fmt.Errorf("%w: path must be /owner/repo", ErrInvalidRepoURL)
	}
	return nil
}

// Clone runs a shallow git clone of opts.RepoURL into a temp dir and resolves
// the HEAD commit SHA. The caller MUST defer the returned Workspace.Cleanup.
func Clone(ctx context.Context, opts CloneOptions) (*Workspace, error) {
	if err := validateRepoURL(opts.RepoURL); err != nil {
		return nil, err
	}

	dir, err := os.MkdirTemp("", "docgoat-clone-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	cloneCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	args := []string{}
	if opts.GithubToken != "" {
		args = append(args, "-c", "http.extraHeader=Authorization: Bearer "+opts.GithubToken)
	}
	args = append(args, "clone", "--depth", "1", "--no-tags", "--single-branch")
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	args = append(args, opts.RepoURL, dir)

	ctx, cancel = context.WithTimeout(cloneCtx, 120*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		cleanup()
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrCloneTimeout
		}
		return nil, fmt.Errorf("git clone: %w: %s", err, stderr.String())
	}

	// Step 5 — resolve the commit SHA (runs after a successful clone).
	var shaBuf strings.Builder
	shaCmd := exec.CommandContext(cloneCtx, "git", "-C", dir, "rev-parse", "HEAD")
	shaCmd.Stdout = &shaBuf
	if err := shaCmd.Run(); err != nil {
		cleanup()
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}

	sha := strings.TrimSpace(shaBuf.String())
	if !shaPattern.MatchString(sha) {
		cleanup()
		return nil, fmt.Errorf("invalid SHA: %q", sha)
	}

	return &Workspace{Dir: dir, CommitSHA: sha, Cleanup: cleanup}, nil
}

func redact(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.Replace(s, "Authorization: Bearer sekret", "Authorization: Bearer [redacted]", 1)
	}
	return out
}
