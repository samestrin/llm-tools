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

// ShipBundleParams configures end-to-end bundle shipping. The bundle is
// scp'd to a host-side staging directory and then docker-cp'd into the
// gateway container, where the clone is created. Reviewers run inside the
// container and read from the container-local clone path.
type ShipBundleParams struct {
	// LocalRepo is the path to the source git repository on this machine.
	LocalRepo string
	// Host is the SSH target where the bundle should land.
	Host string
	// GatewayContainer is the docker container name that hosts the reviewers
	// and where the clone must live. Defaults to "openclaw-gateway" when empty.
	GatewayContainer string
	// RemoteWorkdir is the directory INSIDE the container that will hold the
	// bundle and the clone. Created if absent. Should be unique per run.
	RemoteWorkdir string
	// HostStagingDir is a directory on the SSH target host used as a scp
	// landing pad. The bundle is scp'd here, then docker cp'd into the
	// container. Required and must be distinct from RemoteWorkdir.
	HostStagingDir string
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
	// HostStagingBundlePath is the path of the bundle on the SSH target host
	// (scp landing pad). The host staging dir is independently cleaned up.
	HostStagingBundlePath string
	// RemoteRepoPath is the path of the clone directory INSIDE the container.
	// This is what reviewers should read.
	RemoteRepoPath string
	// BundleSize is the byte size of the bundle.
	BundleSize int64
}

// ShipBundle bundles a local repo, ships it to the SSH target host's staging
// dir, then docker-cp's it into the gateway container and clones it inside
// the container. Returns the container clone path — the value reviewers
// (which run inside the container) must use.
//
// Sequence:
//  1. local git bundle
//  2. ssh -- docker exec mkdir <container workdir>
//  3. ssh -- mkdir <host staging dir>
//  4. scp local bundle -> host staging dir
//  5. ssh -- docker cp <host staging>/bundle.git <container>:<workdir>/bundle.git
//  6. ssh -- docker exec git clone (inside the container workdir)
//
// Failure semantics: any failure returns an error wrapping enough context to
// diagnose. Partial state is not cleaned up automatically — caller decides
// via teardown.
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
	if p.HostStagingDir == "" {
		return ShipBundleResult{}, fmt.Errorf("ship: host staging dir required")
	}
	if p.RepoName == "" {
		return ShipBundleResult{}, fmt.Errorf("ship: repo name required")
	}
	if p.GatewayContainer == "" {
		p.GatewayContainer = "openclaw-gateway"
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

	// 2. Make the container workdir. Fails fast if the container is missing
	//    or the user has no write perm under /tmp.
	cMkRes, err := ContainerExec(ctx, ContainerExecParams{
		Host:             p.Host,
		GatewayContainer: p.GatewayContainer,
		Command:          fmt.Sprintf("mkdir -p %s", shellQuote(p.RemoteWorkdir)),
		Timeout:          p.Timeout,
	})
	if err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: container mkdir: %w", err)
	}
	if cMkRes.ExitCode != 0 {
		return ShipBundleResult{}, fmt.Errorf("ship: container mkdir exit %d, stderr: %s", cMkRes.ExitCode, cMkRes.Stderr)
	}

	// 3. Make the host staging dir (scp landing pad).
	hMkRes, err := SSHRun(ctx, SSHParams{
		Host:    p.Host,
		Command: fmt.Sprintf("mkdir -p %s", shellQuote(p.HostStagingDir)),
		Timeout: p.Timeout,
	})
	if err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: host mkdir: %w", err)
	}
	if hMkRes.ExitCode != 0 {
		return ShipBundleResult{}, fmt.Errorf("ship: host mkdir exit %d, stderr: %s", hMkRes.ExitCode, hMkRes.Stderr)
	}

	// 4. scp the bundle to host staging.
	hostStagingBundle := filepath.Join(p.HostStagingDir, "bundle.git")
	if _, err := SCPSend(ctx, SCPParams{
		Host:       p.Host,
		LocalPath:  localBundle,
		RemotePath: hostStagingBundle,
		Timeout:    p.Timeout,
	}); err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: %w", err)
	}

	// 5. docker cp the bundle from host staging into the container workdir.
	//    docker cp is a host-side command (not a container exec), so this is
	//    a raw ssh call. We let the outer shell quote the spec; the path
	//    contains only the bundle.git filename and our controlled workdir.
	containerBundle := filepath.Join(p.RemoteWorkdir, "bundle.git")
	cpCmd := fmt.Sprintf("docker cp %s %s:%s",
		shellQuote(hostStagingBundle),
		shellQuote(p.GatewayContainer),
		shellQuote(containerBundle))
	cpRes, err := SSHRun(ctx, SSHParams{
		Host:    p.Host,
		Command: cpCmd,
		Timeout: p.Timeout,
	})
	if err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: docker cp: %w", err)
	}
	if cpRes.ExitCode != 0 {
		return ShipBundleResult{}, fmt.Errorf("ship: docker cp exit %d, stderr: %s", cpRes.ExitCode, cpRes.Stderr)
	}

	// 6. Clone the bundle inside the container into RepoName.
	containerRepo := filepath.Join(p.RemoteWorkdir, p.RepoName)
	cloneCmd := fmt.Sprintf("cd %s && git clone -q bundle.git %s",
		shellQuote(p.RemoteWorkdir), shellQuote(p.RepoName))
	cloneRes, err := ContainerExec(ctx, ContainerExecParams{
		Host:             p.Host,
		GatewayContainer: p.GatewayContainer,
		Command:          cloneCmd,
		Timeout:          p.Timeout,
	})
	if err != nil {
		return ShipBundleResult{}, fmt.Errorf("ship: container clone: %w", err)
	}
	if cloneRes.ExitCode != 0 {
		return ShipBundleResult{}, fmt.Errorf("ship: container clone exit %d, stderr: %s", cloneRes.ExitCode, cloneRes.Stderr)
	}

	return ShipBundleResult{
		LocalBundlePath:       localBundle,
		HostStagingBundlePath: hostStagingBundle,
		RemoteRepoPath:        containerRepo,
		BundleSize:            info.Size(),
	}, nil
}

// shellQuote wraps a string in single quotes for safe shell interpolation,
// escaping any embedded single quotes via '\” the classic way.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
