package builders

import (
	"fmt"
	"os"
	"os/exec"

	"k8s.io/klog"
)

type PodmanBuilder struct {
	opts Opts
}

func NewPodmanBuilder(opts Opts) PodmanBuilder {
	return PodmanBuilder{opts: opts}
}

func (p *PodmanBuilder) Build() error {
	if err := makeBinaries(p.opts.RepoRoot); err != nil {
		return err
	}

	if err := p.buildContainer(); err != nil {
		return fmt.Errorf("unable to build container: %w", err)
	}

	if err := p.pushContainer(); err != nil {
		return fmt.Errorf("unable to push container: %w", err)
	}

	return nil
}

func (p *PodmanBuilder) buildContainer() error {
	podmanOpts := []string{"build", "-t", p.opts.FinalPullspec, "--file", p.opts.DockerfileName, "."}
	if p.opts.PullSecretPath != "" {
		podmanOpts = append([]string{"--authfile", p.opts.PullSecretPath}, podmanOpts...)
	}

	cmd := exec.Command("podman", podmanOpts...)
	cmd.Dir = p.opts.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	klog.Infof("Running %s", cmd)
	return cmd.Run()
}

func (p *PodmanBuilder) pushContainer() error {
	cmd := exec.Command("podman", "--authfile", p.opts.PushSecretPath, "push", p.opts.FinalPullspec)
	cmd.Dir = p.opts.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	klog.Infof("Running %s", cmd)
	return cmd.Run()
}
