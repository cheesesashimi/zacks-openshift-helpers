package main

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/installconfig"
	"k8s.io/klog"
)

const (
	clusterLifecycleLogFile string = ".cluster-lifecycle-log.yaml"
	currentInstallFile      string = ".current-install.yaml"
	persistentReleaseFile   string = ".release"
	vacationModeFile        string = ".vacation"
)

type inputOpts struct {
	postInstallManifestPath string
	pullSecretPath          string
	releaseArch             string
	releaseKind             string
	releaseStream           string
	sshKeyPath              string
	username                string
	workDir                 string
	writeLogFile            bool
}

func (i *inputOpts) appendWorkDir(path string) string {
	return filepath.Join(i.workDir, path)
}

func (i *inputOpts) vacationFilePath() string {
	return i.appendWorkDir(vacationModeFile)
}

func (i *inputOpts) releaseFilePath() string {
	return i.appendWorkDir(persistentReleaseFile)
}

func (i *inputOpts) clusterName() string {
	cfgOpts := i.toInstallConfigOpts()
	return cfgOpts.ClusterName()
}

func (i *inputOpts) logPath() string {
	return i.appendWorkDir(clusterLifecycleLogFile)
}

func (i *inputOpts) currentInstallPath() string {
	return i.appendWorkDir(currentInstallFile)
}

func (i *inputOpts) installerPath() string {
	return i.appendWorkDir("openshift-install")
}

func (i *inputOpts) validateForTeardown() error {
	return fixProvidedPath(&i.workDir)
}

func (i *inputOpts) validateForSetup() error {
	if err := fixProvidedPath(&i.workDir); err != nil {
		return err
	}

	klog.Infof("Workdir: %s", i.workDir)

	if err := fixProvidedPath(&i.sshKeyPath); err != nil {
		return err
	}

	klog.Infof("SSH key path: %s", i.sshKeyPath)

	if err := fixProvidedPath(&i.pullSecretPath); err != nil {
		return err
	}

	klog.Infof("Pull secret path: %s", i.pullSecretPath)

	if i.username == defaultUser {
		u, err := user.Current()
		if err != nil {
			return err
		}

		i.username = u.Username
	}

	klog.Infof("Username: %s", i.username)

	if i.releaseKind == "okd-scos" && !strings.Contains(i.releaseStream, "scos") {
		return fmt.Errorf("invalid release stream %q for kind okd-scos", i.releaseStream)
	}

	if i.releaseKind == "okd" && strings.Contains(i.releaseStream, "scos") {
		return fmt.Errorf("invalid release stream %q for kind okd", i.releaseStream)
	}

	klog.Infof("Cluster name: %s", i.clusterName())
	klog.Infof("Cluster kind: %s. Cluster arch: %s. Release stream: %s", i.releaseKind, i.releaseArch, i.releaseStream)

	return nil
}

func (i *inputOpts) toInstallConfigOpts() installconfig.Opts {
	return installconfig.Opts{
		Arch:           i.releaseArch,
		Kind:           i.releaseKind,
		PullSecretPath: i.pullSecretPath,
		SSHKeyPath:     i.sshKeyPath,
		Username:       i.username,
	}
}

func fixProvidedPath(path *string) error {
	pathCopy := *path
	if !strings.Contains(pathCopy, "$HOME") {
		return nil
	}

	u, err := user.Current()
	if err != nil {
		return err
	}

	out := strings.ReplaceAll(pathCopy, "$HOME/", "")
	out = filepath.Join(u.HomeDir, out)
	*path = out
	return nil
}
