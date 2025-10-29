package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"k8s.io/klog"
)

func getReleaseInfo(pullspec string, cfg config) (*releasecontroller.ReleaseInfo, error) {
	if cfg.srcRepoAuthfilePath != "" {
		return releasecontroller.GetReleaseInfoWithAuthfile(pullspec, cfg.srcRepoAuthfilePath)
	}

	return releasecontroller.GetReleaseInfo(pullspec)
}

func runCommandsWithoutOutput(cmds [][]string) error {
	for _, cmd := range cmds {
		fmt.Println("Running:", strings.Join(cmd, " "))
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("could not run %q: %w", strings.Join(cmd, " "), err)
		}
	}

	return nil
}

func runCommandWithOutput(cmd []string) (*bytes.Buffer, *bytes.Buffer, error) {
	c := exec.Command(cmd[0], cmd[1:]...)
	stdoutBuf := bytes.NewBuffer([]byte{})
	stderrBuf := bytes.NewBuffer([]byte{})
	c.Stdout = stdoutBuf
	c.Stderr = stderrBuf

	klog.Infof("Running %s", strings.Join(cmd, " "))

	err := c.Run()

	return stdoutBuf, stderrBuf, err
}

func imageExists(pullspec string) (bool, error) {
	stdoutBuf, stderrBuf, err := runCommandWithOutput([]string{"skopeo", "inspect", "docker://" + pullspec})
	if err == nil {
		return true, nil
	}

	imageNotFoundLines := []string{
		"manifest unknown",
		"was deleted or has expired",
	}

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	for _, line := range imageNotFoundLines {
		if strings.Contains(stdoutStr, line) {
			return false, nil
		}

		if strings.Contains(stderrStr, line) {
			return false, nil
		}
	}

	return false, fmt.Errorf("could not determine whether the image exists: %s: %w", stderrBuf.String(), err)
}
