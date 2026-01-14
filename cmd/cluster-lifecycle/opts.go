package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/installconfig"
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"k8s.io/klog"
)

const (
	clusterLifecycleLogFile string = ".cluster-lifecycle-log.yaml"
	currentInstallFile      string = ".current-install.yaml"
	persistentReleaseFile   string = ".release"
	vacationModeFile        string = ".vacation"
)

type release struct {
	arch     string
	kind     string
	pullspec string
	stream   string
}

type inputOpts struct {
	installConfigPath       string
	enableTechPreview       bool
	postInstallManifestPath string
	pullSecretPath          string
	release                 release
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

func (i *inputOpts) loadInstallConfigFromFile() (*installconfig.ParsedInstallConfig, error) {
	cfg, err := os.ReadFile(i.installConfigPath)
	if err != nil {
		return nil, err
	}

	parsed, err := installconfig.ParseInstallConfig(cfg)
	if err != nil {
		return nil, err
	}

	i.release.arch = parsed.Architecture

	return parsed, err
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
	wd, err := filepath.Abs(i.workDir)
	if err != nil {
		return err
	}

	i.workDir = wd
	return nil
}

func (i *inputOpts) inferArchAndKindFromPullspec(pullspec string) error {
	releaseInfo, err := releasecontroller.GetReleaseInfo(pullspec)
	if err != nil {
		return err
	}

	i.release.arch = releaseInfo.Config.Architecture
	releaseName := releaseInfo.References.Name

	switch {
	case strings.Contains(releaseName, "okd-scos"):
		i.release.kind = "okd-scos"
	case strings.Contains(releaseName, "okd"):
		i.release.kind = "okd"
	default:
		i.release.kind = "ocp"
	}

	// Clear the release stream since we won't talk to a release controller here.
	i.release.stream = ""

	klog.Infof("Inferred %s and %s for release %s", i.release.arch, i.release.kind, pullspec)

	return nil
}

func (i *inputOpts) validateForSetup() error {
	wd, err := filepath.Abs(i.workDir)
	if err != nil {
		return err
	}

	i.workDir = wd

	klog.Infof("Workdir: %s", i.workDir)

	sp, err := filepath.Abs(i.sshKeyPath)
	if err != nil {
		return err
	}

	i.sshKeyPath = sp

	klog.Infof("SSH key path: %s", i.sshKeyPath)

	psp, err := filepath.Abs(i.pullSecretPath)
	if err != nil {
		return err
	}

	i.pullSecretPath = psp

	klog.Infof("Pull secret path: %s", i.pullSecretPath)

	if i.prefix == "" && i.installConfigPath == "" {
		u, err := user.Current()
		if err != nil {
			return err
		}

		i.prefix = u.Username
		klog.Infof("Using prefix: %s", i.prefix)
	}

	if i.installConfigPath != "" {
		icp, err := filepath.Abs(i.installConfigPath)
		if err != nil {
			return err
		}

		i.installConfigPath = icp

		if strings.Contains(i.installConfigPath, i.workDir) {
			return fmt.Errorf("provided installconfig cannot be inside workdir")
		}
	}

	if i.release.pullspec == "" {
		if i.release.kind == "okd-scos" && !strings.Contains(i.release.stream, "scos") {
			return fmt.Errorf("invalid release stream %q for kind okd-scos", i.release.stream)
		}

		if i.release.kind == "okd" && strings.Contains(i.release.stream, "scos") {
			return fmt.Errorf("invalid release stream %q for kind okd", i.release.stream)
		}
	} else {
		if i.release.kind == "" {
			klog.Warningf("--release-kind will be ignored because --release-pullspec was used")
		}

		if i.release.arch == "" {
			klog.Warningf("--release-arch will be ignored because --release-pullspec was used")
		}
	}

	return nil
}

func (i *inputOpts) toInstallConfigOpts() installconfig.Opts {
	return installconfig.Opts{
		Arch:              i.release.arch,
		EnableTechPreview: i.enableTechPreview,
		Kind:              i.release.kind,
		Paths: installconfig.Paths{
			InstallConfigPath: i.installConfigPath,
			PullSecretPath:    i.pullSecretPath,
			SSHKeyPath:        i.sshKeyPath,
		},
		Prefix: i.prefix,
	}
}
