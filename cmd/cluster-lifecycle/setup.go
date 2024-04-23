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

var (
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Brings up an OpenShift cluster for testing purposes",
		Long:  "",
		Run:   runSetupCmd,
	}

	setupOpts = inputOpts{}
)

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.PersistentFlags().StringVar(&setupOpts.postInstallManifestPath, "post-install-manifests", "", "Directory containing K8s manifests to apply after successful installation.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.pullSecretPath, "pull-secret-path", defaultPullSecretPath, "Path to a pull secret that can pull from registry.ci.openshift.org")
	setupCmd.PersistentFlags().StringVar(&setupOpts.releasePullspec, "release-pullspec", "", "An arbitrary release pullspec to spin up.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.releaseArch, "release-arch", "amd64", fmt.Sprintf("Release arch, one of: %v", sets.List(installconfig.GetSupportedArches())))
	setupCmd.PersistentFlags().StringVar(&setupOpts.releaseKind, "release-kind", "ocp", fmt.Sprintf("Release kind, one of: %v", sets.List(installconfig.GetSupportedKinds())))
	setupCmd.PersistentFlags().StringVar(&setupOpts.releaseStream, "release-stream", "4.14.0-0.ci", "The release stream to use")
	setupCmd.PersistentFlags().StringVar(&setupOpts.sshKeyPath, "ssh-key-path", defaultSSHKeyPath, "Path to an SSH key to embed in the installation config.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.prefix, "prefix", "$USER", "Prefix to add to the cluster name; will use current system user if not set.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.workDir, "work-dir", defaultWorkDir, "The directory to use for running openshift-install. Enables vacation and persistent install mode when used in a cron job.")
	setupCmd.PersistentFlags().BoolVar(&setupOpts.writeLogFile, "write-log-file", false, "Keeps track of cluster setups and teardown by writing to "+clusterLifecycleLogFile)
	setupCmd.PersistentFlags().BoolVar(&setupOpts.enableTechPreview, "enable-tech-preview", false, "Enables Tech Preview features")
	setupCmd.PersistentFlags().StringVar(&setupOpts.variant, "variant", "", fmt.Sprintf("A cluster variant to bring up. One of: %v", sets.List(installconfig.GetSupportedVariants())))
}

func runSetupCmd(_ *cobra.Command, _ []string) {
	if err := runSetup(); err != nil {
		klog.Fatalln(err)
	}
}

func runSetup() error {
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

	releasePullspec, err := getRelease(&setupOpts)
	if err != nil {
		return err
	}

	klog.Infof("Cluster name: %s", setupOpts.clusterName())

	if setupOpts.releaseStream != "" {
		klog.Infof("Cluster kind: %s. Cluster arch: %s. Release stream: %s", setupOpts.releaseKind, setupOpts.releaseArch, setupOpts.releaseStream)
	} else {
		klog.Infof("Cluster kind: %s. Cluster arch: %s.", setupOpts.releaseKind, setupOpts.releaseArch)
	}

	klog.Infof("Found release %s", releasePullspec)

	logEntry := newSetupLogEntry(releasePullspec, setupOpts)
	if err := logEntry.writeToCurrentInstallPath(setupOpts); err != nil {
		return fmt.Errorf("unable to write log entry: %w", err)
	}

	defer func() {
		// We write this twice to collect info that we don't have available until
		// after the installation is complete, e.g., elapsed time, etc.
		if err := logEntry.writeToCurrentInstallPath(setupOpts); err != nil {
			klog.Fatalf("unable to write current install file: %s", err)
		}

		if err := logEntry.appendToLogFile(setupOpts); err != nil {
			klog.Fatalf("unable to write log file: %s", err)
		}
	}()

	if err := writeInstallConfig(setupOpts); err != nil {
		return err
	}

	if err := extractInstaller(releasePullspec, setupOpts); err != nil {
		return nil
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

func writeInstallConfig(opts inputOpts) error {
	installConfigPath := filepath.Join(opts.workDir, "install-config.yaml")

	installConfig, err := installconfig.GetInstallConfig(opts.toInstallConfigOpts())
	if err != nil {
		return fmt.Errorf("could not get install config: %w", err)
	}

	klog.Infof("Writing install config to %s", installConfigPath)
	if err := os.WriteFile(installConfigPath, installConfig, 0o755); err != nil {
		return fmt.Errorf("could not write install config: %w", err)
	}

	return nil
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
	if opts.releasePullspec != "" {
		return opts.releasePullspec, opts.inferArchAndKindFromPullspec(opts.releasePullspec)
	}

	releaseFileExists, err := isFileExists(opts.releaseFilePath())
	if err != nil {
		return "", err
	}

	if !releaseFileExists {
		return getReleaseFromController(*opts)
	}

	return getReleaseFromFile(opts)
}

func getReleaseFromController(opts inputOpts) (string, error) {
	rc, err := releasecontroller.GetReleaseController(opts.releaseKind, opts.releaseArch)
	if err != nil {
		return "", err
	}

	klog.Infof("Getting latest release for stream %s from %s", opts.releaseStream, rc)

	release, err := rc.GetLatestReleaseForStream(opts.releaseStream)
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
