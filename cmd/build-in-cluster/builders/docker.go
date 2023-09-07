package builders

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/containers"
	"k8s.io/klog"
)

type Opts struct {
	RepoRoot       string
	PullSecretPath string
	PushSecretPath string
	FinalPullspec  string
	DockerfileName string
}

type DockerBuilder struct {
	opts Opts
}

func NewDockerBuilder(opts Opts) DockerBuilder {
	return DockerBuilder{opts: opts}
}

func (d *DockerBuilder) Build() (string, error) {
	if err := makeBinaries(d.opts.RepoRoot); err != nil {
		return "", err
	}

	if err := d.buildContainer(); err != nil {
		return "", fmt.Errorf("unable to build container: %w", err)
	}

	if err := d.pushContainer(); err != nil {
		return "", fmt.Errorf("unable to push container: %w", err)
	}

	return containers.ResolveToDigestedPullspec(d.opts.FinalPullspec, d.opts.PushSecretPath)
}

func (d *DockerBuilder) buildContainer() error {
	dockerOpts := []string{"build", "-t", d.opts.FinalPullspec, "--file", d.opts.DockerfileName, "."}

	if d.opts.PullSecretPath != "" {
		pullSecretDir, cleanupFunc, err := d.getConfigDir(d.opts.PullSecretPath)
		if err != nil {
			return fmt.Errorf("could not get pull secret path: %w", err)
		}
		defer func() {
			if err := cleanupFunc(); err != nil {
				klog.Fatalln(err)
			}
		}()

		dockerOpts = append([]string{"--config", pullSecretDir}, dockerOpts...)
	}

	cmd := exec.Command("docker", dockerOpts...)
	cmd.Dir = d.opts.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	klog.Infof("Running %s", cmd)
	return cmd.Run()
}

func (d *DockerBuilder) pushContainer() error {
	// Docker needs the directory containing the push secret path (and expects it
	// to be in a file called "config.json"), instead of the full path to it.
	pushSecretDir, cleanupFunc, err := d.getConfigDir(d.opts.PushSecretPath)
	if err != nil {
		return fmt.Errorf("could not get push secret dir")
	}
	defer func() {
		if err := cleanupFunc(); err != nil {
			klog.Fatalln(err)
		}
	}()

	cmd := exec.Command("docker", "--config", pushSecretDir, "push", d.opts.FinalPullspec)
	cmd.Dir = d.opts.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	klog.Infof("Running %s", cmd)
	return cmd.Run()
}

// Docker needs the directory that a secret exists in, not the full path to the
// secret itself. Additionally, the secret must have the name "config.json".
// This works around that.
func (d *DockerBuilder) getConfigDir(secretPath string) (string, func() error, error) {
	// If we have a "config.json" in this directory, just return the directory
	// path.
	if strings.HasSuffix(secretPath, "/config.json") {
		return filepath.Dir(secretPath), func() error { return nil }, nil
	}

	// Ensure that we were not given a directory
	fi, err := os.Stat(secretPath)
	if err != nil {
		return "", nil, err
	}

	if fi.IsDir() {
		return "", nil, fmt.Errorf("%s is a directory", fi.Name())
	}

	// Copy the push secret to a temporary directory and call it "config.json".
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", nil, fmt.Errorf("could not create temp dir")
	}

	tmpSecretPath := filepath.Join(tempDir, "config.json")
	klog.Infof("Copying push secret from %s to %s to work around Docker's limitation", secretPath, tmpSecretPath)

	inBytes, err := os.ReadFile(secretPath)
	if err != nil {
		return "", nil, err
	}

	if err := os.WriteFile(tmpSecretPath, inBytes, 0o755); err != nil {
		return "", nil, err
	}

	cleanupFunc := func() error {
		klog.Infof("Deleting %s", tempDir)
		return os.RemoveAll(tempDir)
	}

	return tempDir, cleanupFunc, nil
}
