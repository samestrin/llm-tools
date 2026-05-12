package multireview

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CreateBundle writes a git bundle containing the full branch history plus
// all tags. A range-only bundle would fail to clone on the remote because
// the prerequisites would be missing — we always ship the full history so
// reviewers can resolve any base ref via tag lookup.
func CreateBundle(repoPath, outBundle string) error {
	if repoPath == "" {
		return fmt.Errorf("bundle: repo path required")
	}
	if outBundle == "" {
		return fmt.Errorf("bundle: output path required")
	}

	cmd := exec.Command("git", "bundle", "create", outBundle, "HEAD", "--branches", "--tags")
	cmd.Dir = repoPath
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git bundle create: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// ShipBundleParams configures end-to-end bundle shipping.
type ShipBundleParams struct {
	// LocalRepo is the path to the source git repository on this machine.
	LocalRepo string
	// Host is the SSH target where the bundle should land.
	Host string
	// RemoteWorkdir is the directory on the remote that will hold the bundle
	// and the clone. Created if absent. Should be unique per run.
	RemoteWorkdir string
	// RepoName is the directory name to clone the bundle into, inside
	// RemoteWorkdir. Reviewers will read from this path.
	RepoName string
	// Timeout caps the total wall-clock for the ship operation.
	Timeout time.Duration
}

// ShipBundleResult reports what was shipped.
type ShipBundleResult struct {
	// LocalBundlePath is the path to the local .git bundle file (kept for
	// inspection until the caller cleans it up).
	LocalBundlePath string
	// RemoteBundlePath is the path of the bundle on the remote.
	RemoteBundlePath string
	// RemoteRepoPath is the path of the clone directory on the remote — this
	// is what reviewers should read.
	RemoteRepoPath string
	// BundleSize is the byte size of the bundle.
	BundleSize int64
}

// ShipBundle bundles a local repo, ships it to a remote host, and clones it
// into the remote workdir under RepoName. Returns the remote clone path so
// reviewers can read from it.
//
// Failure semantics: any failure (bundle, scp, mkdir, clone) returns an
// error wrapping enough context to diagnose. Partial state is not cleaned
// up automatically — caller decides via teardown.
func ShipBundle(ctx context.Context, p ShipBundleParams) (ShipBundleResult, error) {
	if p.LocalRepo == "" {
		return ShipBundleResult{}, fmt.Errorf("ship: local repo required")
	}
	if p.Host == "" {
		return ShipBundleResult{}, fmt.Errorf("ship: host required")
	}
	if p.RemoteWorkdir == "" {
		return ShipBundleResult{}, fmt.Errorf("ship: remote workdir required")
	}
	if p.RepoName == "" {
		return ShipBundleResult{}, fmt.Errorf("ship: repo name required")
	}
	if p.Timeout <= 0 {
		p.Timeout = 5 * time.Minute
	}

	// 1. Create the local bundle in a temp file.
	bundleDir, err := os.MkdirTemp("", "multireview-bundle-")
	if err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: tempdir: %w", err)
	}
	localBundle := filepath.Join(bundleDir, "bundle.git")
	if err := CreateBundle(p.LocalRepo, localBundle); err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: %w", err)
	}
	info, err := os.Stat(localBundle)
	if err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: stat bundle: %w", err)
	}

	// 2. Make remote workdir.
	mkdirRes, err := SSHRun(ctx, SSHParams{
		Host:    p.Host,
		Command: fmt.Sprintf("mkdir -p %s", shellQuote(p.RemoteWorkdir)),
		Timeout: p.Timeout,
	})
	if err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: ssh mkdir: %w", err)
	}
	if mkdirRes.ExitCode != 0 {
		return ShipBundleResult{}, fmt.Errorf("ship: mkdir exit %d, stderr: %s", mkdirRes.ExitCode, mkdirRes.Stderr)
	}

	// 3. SCP bundle to remote.
	remoteBundle := filepath.Join(p.RemoteWorkdir, "bundle.git")
	if _, err := SCPSend(ctx, SCPParams{
		Host:       p.Host,
		LocalPath:  localBundle,
		RemotePath: remoteBundle,
		Timeout:    p.Timeout,
	}); err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: %w", err)
	}

	// 4. Clone the bundle on the remote into RepoName.
	remoteRepo := filepath.Join(p.RemoteWorkdir, p.RepoName)
	cloneCmd := fmt.Sprintf("cd %s && git clone -q bundle.git %s",
		shellQuote(p.RemoteWorkdir), shellQuote(p.RepoName))
	cloneRes, err := SSHRun(ctx, SSHParams{
		Host:    p.Host,
		Command: cloneCmd,
		Timeout: p.Timeout,
	})
	if err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: ssh clone: %w", err)
	}
	if cloneRes.ExitCode != 0 {
		return ShipBundleResult{}, fmt.Errorf("ship: clone exit %d, stderr: %s", cloneRes.ExitCode, cloneRes.Stderr)
	}

	return ShipBundleResult{
		LocalBundlePath:  localBundle,
		RemoteBundlePath: remoteBundle,
		RemoteRepoPath:   remoteRepo,
		BundleSize:       info.Size(),
	}, nil
}

// shellQuote wraps a string in single quotes for safe shell interpolation,
// escaping any embedded single quotes via '\” the classic way.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
