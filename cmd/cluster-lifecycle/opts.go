package main

import (
	"encoding/json"
	"fmt"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/installconfig"
	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/releasecontroller"
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
	releasePullspec         string
	releaseStream           string
	sshKeyPath              string
	prefix                  string
	workDir                 string
	writeLogFile            bool
	variant                 string
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

func (i *inputOpts) inferArchAndKindFromPullspec(pullspec string) error {
	releaseInfoBytes, err := releasecontroller.GetReleaseInfo(pullspec)
	if err != nil {
		return err
	}

	releaseInfo := interface{}(nil)

	if err := json.Unmarshal(releaseInfoBytes, &releaseInfo); err != nil {
		return err
	}

	arch, err := jsonpath.Get(`config.architecture`, releaseInfo)
	if err != nil {
		return err
	}

	i.releaseArch = arch.(string)

	if i.releaseArch == "" {
		return fmt.Errorf("empty architecture field for release %s", pullspec)
	}

	releaseName, err := jsonpath.Get(`references.metadata.name`, releaseInfo)
	if err != nil {
		return err
	}

	releaseNameStr := releaseName.(string)

	if releaseNameStr == "" {
		return fmt.Errorf("empty release name field for release %s", pullspec)
	}

	if strings.Contains(releaseNameStr, "okd-scos") {
		i.releaseKind = "okd-scos"
	} else if strings.Contains(releaseNameStr, "okd") {
		i.releaseKind = "okd"
	} else {
		i.releaseKind = "ocp"
	}

	// Clear the release stream since we won't talk to a release controller here.
	i.releaseStream = ""

	klog.Infof("Inferred %s and %s for release %s", i.releaseArch, i.releaseKind, pullspec)

	return nil
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

	if i.prefix == defaultUser {
		u, err := user.Current()
		if err != nil {
			return err
		}

		i.prefix = u.Username
	}

	klog.Infof("Using prefix: %s", i.prefix)

	if i.releasePullspec == "" {
		if i.releaseKind == "okd-scos" && !strings.Contains(i.releaseStream, "scos") {
			return fmt.Errorf("invalid release stream %q for kind okd-scos", i.releaseStream)
		}

		if i.releaseKind == "okd" && strings.Contains(i.releaseStream, "scos") {
			return fmt.Errorf("invalid release stream %q for kind okd", i.releaseStream)
		}
	} else {
		if i.releaseKind == "" {
			klog.Warningf("--release-kind will be ignored because --release-pullspec was used")
		}

		if i.releaseArch == "" {
			klog.Warningf("--release-arch will be ignored because --release-pullspec was used")
		}
	}

	return nil
}

func (i *inputOpts) toInstallConfigOpts() installconfig.Opts {
	return installconfig.Opts{
		Arch:           i.releaseArch,
		Kind:           i.releaseKind,
		PullSecretPath: i.pullSecretPath,
		SSHKeyPath:     i.sshKeyPath,
		Prefix:         i.prefix,
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
