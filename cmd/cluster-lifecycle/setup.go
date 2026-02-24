package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/installconfig"
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

func init() {
	setupOpts := inputOpts{}

	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Brings up an OpenShift cluster for testing purposes",
		Long:  "",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSetup(setupOpts)
		},
	}

	setupCmd.PersistentFlags().StringVar(&setupOpts.installConfigPath, "install-config", "", "Path to OpenShift install config to use.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.postInstallManifestPath, "post-install-manifests", "", "Directory containing K8s manifests to apply after successful installation.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.pullSecretPath, "pull-secret-path", "", "Path to a pull secret that can pull from registry.ci.openshift.org")
	setupCmd.PersistentFlags().StringVar(&setupOpts.release.pullspec, "release-pullspec", "", "An arbitrary release pullspec to spin up.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.release.arch, "release-arch", "amd64", fmt.Sprintf("Release arch, one of: %v", sets.List(installconfig.GetSupportedArches())))
	setupCmd.PersistentFlags().StringVar(&setupOpts.release.kind, "release-kind", "ocp", fmt.Sprintf("Release kind, one of: %v", sets.List(installconfig.GetSupportedKinds())))
	setupCmd.PersistentFlags().StringVar(&setupOpts.release.stream, "release-stream", "4.14.0-0.ci", "The release stream to use")
	setupCmd.PersistentFlags().StringVar(&setupOpts.sshKeyPath, "ssh-key-path", "", "Path to an SSH key to embed in the installation config.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.prefix, "prefix", "", "Prefix to add to the cluster name; will use current system user if not set.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.workDir, "work-dir", "", "The directory to use for running openshift-install. Enables vacation and persistent install mode when used in a cron job.")
	setupCmd.PersistentFlags().BoolVar(&setupOpts.enableTechPreview, "enable-tech-preview", false, "Enables Tech Preview features")
	setupCmd.PersistentFlags().StringVar(&setupOpts.variant, "variant", "", fmt.Sprintf("A cluster variant to bring up. One of: %v", sets.List(installconfig.GetSupportedVariants())))
	setupCmd.PersistentFlags().StringVar(&setupOpts.preinstallcfg, "preinstallcfg", "", "Path to a script or binary to run after creating manifests but before installation.")

	rootCmd.AddCommand(setupCmd)
}

func runSetup(setupOpts inputOpts) error {
	_, err := exec.LookPath("oc")
	if err != nil {
		return fmt.Errorf("missing required binary oc")
	}

	if err := setupOpts.validateForSetup(); err != nil {
		return fmt.Errorf("could not validate input options: %w", err)
	}

	if err := setupWorkDir(setupOpts.workDir); err != nil {
		return fmt.Errorf("could not set up workdir: %w", err)
	}

	vacationFile := setupOpts.vacationFilePath()
	if inVacationMode, err := isInVacationMode(setupOpts); inVacationMode {
		klog.Infof("%s detected, in vacation mode.", vacationFile)
		return nil
	} else if err != nil {
		return err
	}

	pullspec, err := getRelease(&setupOpts)
	if err != nil {
		return err
	}

	setupOpts.release.pullspec = pullspec

	if setupOpts.release.stream != "" {
		klog.Infof("Cluster kind: %s. Cluster arch: %s. Release stream: %s", setupOpts.release.kind, setupOpts.release.arch, setupOpts.release.stream)
	} else {
		klog.Infof("Cluster kind: %s. Cluster arch: %s.", setupOpts.release.kind, setupOpts.release.arch)
	}

	klog.Infof("Found release %s", setupOpts.release.pullspec)

	installCfg, err := writeInstallConfig(setupOpts)
	if err != nil {
		return err
	}

	klog.Infof("Cluster name: %s", installCfg.Name)

	if err := extractInstaller(setupOpts.release.pullspec, setupOpts); err != nil {
		return nil
	}

	// If a preinstall config script is given, create the manifests separately,
	// then run the provided script before installation to configure the
	// manifests. The script will be run within the context of the work
	// directory.
	if setupOpts.preinstallcfg != "" {
		if err := generateManifests(setupOpts); err != nil {
			return fmt.Errorf("unable to generate manifests for openshift-install: %w", err)
		}

		if err := runPreinstallCfg(setupOpts); err != nil {
			return fmt.Errorf("could not run preinstall config script: %w", err)
		}
	}

	if err := installCluster(setupOpts); err != nil {
		return fmt.Errorf("unable to run openshift-install: %w", err)
	}

	if err := applyPostInstallManifests(setupOpts); err != nil {
		return err
	}

	klog.Infof("Installation complete!")
	return nil
}

func setupWorkDir(workDir string) error {
	exists, err := isFileExists(workDir)
	if exists {
		klog.Infof("Found existing workdir %s", workDir)
		return nil
	}

	if err != nil {
		return err
	}

	klog.Infof("Workdir %s does not exist, creating", workDir)
	return os.MkdirAll(workDir, 0o755)
}

func writeInstallConfig(opts inputOpts) (*installconfig.ParsedInstallConfig, error) {
	finalInstallConfigPath := filepath.Join(opts.workDir, "install-config.yaml")

	installCfgOpts := opts.toInstallConfigOpts()

	if opts.installConfigPath == "" {
		klog.Infof("Generating new install config")
	} else {
		klog.Infof("Using installconfig from %s", opts.installConfigPath)
	}

	installCfg, err := installconfig.GetInstallConfig(installCfgOpts)
	if err != nil {
		return nil, fmt.Errorf("could not generate install config: %w", err)
	}

	klog.Infof("Writing install config to %s", finalInstallConfigPath)
	if err := os.WriteFile(finalInstallConfigPath, installCfg, 0o755); err != nil {
		return nil, fmt.Errorf("could not write install config: %w", err)
	}

	return installconfig.ParseInstallConfig(installCfg)
}

func applyPostInstallManifests(opts inputOpts) error {
	if opts.postInstallManifestPath == "" {
		klog.Infof("No post-installation manifests to apply")
		return nil
	}

	klog.Infof("Applying post installation manifests from %s", opts.postInstallManifestPath)

	cmd := exec.Command("oc", "apply", "-f", opts.postInstallManifestPath)
	cmd.Env = utils.ToEnvVars(map[string]string{
		"KUBECONFIG": filepath.Join(opts.workDir, "auth", "kubeconfig"),
	})
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	klog.Infof("Running %s", cmd)
	return cmd.Run()
}

func getRelease(opts *inputOpts) (string, error) {
	if opts.release.pullspec != "" {
		return opts.release.pullspec, opts.inferArchAndKindFromPullspec(opts.release.pullspec)
	}

	releaseFileExists, err := isFileExists(opts.releaseFilePath())
	if err != nil {
		return "", err
	}

	if !releaseFileExists {
		return getReleaseFromController(opts.release)
	}

	return getReleaseFromFile(opts)
}

func getReleaseFromController(rel release) (string, error) {
	rc, err := releasecontroller.GetReleaseController(rel.kind, rel.arch)
	if err != nil {
		return "", err
	}

	klog.Infof("Getting latest release for stream %s from %s", rel.stream, rc)

	release, err := rc.ReleaseStream(rel.stream).Latest()
	if err != nil {
		return "", err
	}

	return release.Pullspec, nil
}

func getReleaseFromFile(opts *inputOpts) (string, error) {
	releasePath := filepath.Join(opts.workDir, persistentReleaseFile)
	releaseBytes, err := os.ReadFile(releasePath)
	if err != nil {
		return "", err
	}

	release := string(releaseBytes)
	if release == "" {
		return "", fmt.Errorf("release file %s exists, but is empty", releasePath)
	}

	return release, opts.inferArchAndKindFromPullspec(release)
}

func isInVacationMode(opts inputOpts) (bool, error) {
	vacationFile := opts.vacationFilePath()
	inVacationMode, err := isFileExists(vacationFile)
	if err != nil {
		return false, fmt.Errorf("could not read %s: %w", vacationFile, err)
	}

	return inVacationMode, nil
}
