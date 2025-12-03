package main

import "fmt"

// TODO: Use Podman API instead.
type podmanClient struct {
	authfile string
}

func newAuthedPodmanClient(authfile string) *podmanClient {
	return &podmanClient{authfile: authfile}
}

func newPodmanClient() *podmanClient {
	return &podmanClient{}
}

func (p *podmanClient) getPodmanPreamble(subCmds ...string) []string {
	out := append([]string{"podman"}, subCmds...)

	if p.authfile == "" {
		return out
	}

	return append(out, []string{"--authfile", p.authfile}...)
}

func (p *podmanClient) Build(pullspec, buildCtxDir, containerfilePath string) error {
	args := p.getPodmanPreamble("build")
	args = append(args, []string{"-t", pullspec, "--file=" + containerfilePath, buildCtxDir}...)

	if err := runCommandWithOutput(args); err != nil {
		return fmt.Errorf("could not build container image: %w", err)
	}

	return nil
}

func (p *podmanClient) PullImage(pullspec string) error {
	args := p.getPodmanPreamble("pull")
	args = append(args, pullspec)

	if err := runCommandWithOutput(args); err != nil {
		return fmt.Errorf("could not pull image %s: %w", pullspec, err)
	}

	return nil
}

func (p *podmanClient) CreateContainer(pullspec, name string) error {
	args := p.getPodmanPreamble("container", "create")
	args = append(args, []string{"--name", name, pullspec}...)

	if err := runCommandWithOutput(args); err != nil {
		return fmt.Errorf("could not create container %s from pullspec %s: %w", name, pullspec, err)
	}

	return nil
}

func (p *podmanClient) DeleteContainer(name string) error {
	if err := runCommandWithOutput([]string{"podman", "container", "rm", name}); err != nil {
		return fmt.Errorf("could not delete container %s: %w", name, err)
	}

	return nil
}

func (p *podmanClient) CopyFilesFromContainer(name, srcdir, destdir string) error {
	if err := runCommandWithOutput([]string{"podman", "container", "cp", fmt.Sprintf("%s:%s", name, srcdir), destdir}); err != nil {
		return fmt.Errorf("could not copy files from container: %w", err)
	}

	return nil
}

func (p *podmanClient) CopyFilesFromImage(pullspec, srcdir, destdir string) error {
	if err := p.PullImage(pullspec); err != nil {
		return err
	}

	containerName := "release-payload-extractor"

	if err := p.CreateContainer(pullspec, containerName); err != nil {
		return err
	}

	if err := p.CopyFilesFromContainer(containerName, srcdir, destdir); err != nil {
		return err
	}

	return p.DeleteContainer(containerName)
}
