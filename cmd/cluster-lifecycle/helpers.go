package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/containers"
	"k8s.io/klog"
)

func extractInstaller(releasePullspec string, opts inputOpts) error {
	installerPath := opts.installerPath()

	installerExists, err := isFileExists(installerPath)
	if err != nil {
		return err
	}

	if !installerExists {
		klog.Infof("Extracting installer to %s", opts.workDir)
		start := time.Now()
		defer func() {
			klog.Infof("Installer extracted in %s", time.Since(start))
		}()

		cmd := exec.Command("oc", "adm", "release", "extract", "--registry-config", opts.pullSecretPath, "--command", "openshift-install", releasePullspec, "--to", opts.workDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		klog.Infof("Running %s", cmd)
		return cmd.Run()
	}

	klog.Infof("Found a preexisting openshift-install binary at %s, checking version", installerPath)
	installerVersion, err := getInstallerVersion(opts)
	if err != nil {
		return err
	}

	digestedPullspec, err := containers.ResolveToDigestedPullspec(releasePullspec, opts.pullSecretPath)
	if err != nil {
		return err
	}

	if strings.Contains(installerVersion, digestedPullspec) {
		klog.Infof("Existing installer has version %q, reusing", digestedPullspec)
		return nil
	}

	klog.Infof("Version mismatch, deleting preexisting installer and fetching new one")
	if err := os.Remove(installerPath); err != nil {
		return fmt.Errorf("unable to remove openshift-install: %w", err)
	}

	return extractInstaller(releasePullspec, opts)
}

func installCluster(opts inputOpts) error {
	installerVersion, err := getInstallerVersion(opts)
	if err != nil {
		return err
	}

	klog.Infof(installerVersion)

	cmd := exec.Command(opts.installerPath(), "create", "cluster", "--dir", opts.workDir, "--log-level", "debug")
	cmd.Dir = opts.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	klog.Infof("Running %s", cmd)
	return cmd.Run()
}

func destroyCluster(opts teardownOpts) error {
	installerVersion, err := getInstallerVersion(opts.inputOpts)
	if err != nil {
		return err
	}

	klog.Infof(installerVersion)

	installerPath := filepath.Join(opts.workDir, "openshift-install")
	cmd := exec.Command(installerPath, "destroy", "cluster", "--dir", opts.workDir, "--log-level", "debug")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	klog.Infof("Running %s", cmd)
	return cmd.Run()
}

func getInstallerVersion(opts inputOpts) (string, error) {
	cmd := exec.Command(opts.installerPath(), "version")
	cmd.Dir = opts.workDir
	out := bytes.NewBuffer([]byte{})
	cmd.Stdout = out

	klog.Infof("Running %s", cmd)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return out.String(), nil
}

func ignoreFileNotExistsErr(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

func isFileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}
