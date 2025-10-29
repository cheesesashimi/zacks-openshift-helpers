package main

import (
	"fmt"
	"strings"

	"k8s.io/klog"
)

type localImageRegistry struct {
	containerName string
	port          int
}

func newLocalImageRegistry(name string, port int) *localImageRegistry {
	return &localImageRegistry{
		containerName: name,
		port:          port,
	}
}

func (l *localImageRegistry) start() error {
	if err := l.stopAndRemove(); err != nil {
		return err
	}

	pullspec := "docker.io/library/registry:latest"

	err := runCommandsWithoutOutput([][]string{
		{"podman", "pull", pullspec},
		{"podman", "run", "-dt", "--rm", "-p", fmt.Sprintf("%d:%d", l.port, l.port), "--name", l.containerName, pullspec},
	})

	if err != nil {
		return err
	}

	klog.Infof("Local image registry started")
	return nil
}

func (l *localImageRegistry) stop() error {
	exists, err := l.exists()
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	err = runCommandsWithoutOutput([][]string{
		{"podman", "stop", l.containerName},
	})

	if err != nil {
		return err
	}

	klog.Infof("Local image registry stopped")

	return nil
}

func (l *localImageRegistry) remove() error {
	exists, err := l.exists()
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	err = runCommandsWithoutOutput([][]string{
		{"podman", "rm", l.containerName},
	})

	if err != nil {
		return err
	}

	klog.Infof("Local image registry removed")

	return nil
}

func (l *localImageRegistry) stopAndRemove() error {
	if err := l.stop(); err != nil {
		return err
	}

	if err := l.remove(); err != nil {
		return err
	}

	return nil
}

func (l *localImageRegistry) exists() (bool, error) {
	stdoutBuf, stderrBuf, err := runCommandWithOutput([]string{"podman", "container", "inspect", l.containerName})
	if err == nil {
		return true, nil
	}

	if err != nil && strings.Contains(stdoutBuf.String(), "no such container") || strings.Contains(stderrBuf.String(), "no such container") {
		return false, nil
	}

	return false, err
}
