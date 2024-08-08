package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	provider2 "github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/shim"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/coreos/stream-metadata-go/stream"

	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	"k8s.io/component-base/cli"
	"k8s.io/klog"

	ign3types "github.com/coreos/ignition/v2/config/v3_4/types"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
)

const (
	binName string = "local-mcd-testing"
)

var (
	initOpts = define.InitOptions{
		Name:     "zacks-new-qemu-vm",
		CPUS:     uint64(1),
		Memory:   uint64(2048),
		Username: "core",
		DiskSize: uint64(100),
		// Image:    "./rhcos-417.94.202407010929-0-qemu.x86_64.qcow2",
	}

	rootCmd = &cobra.Command{
		Use:   binName,
		Short: "Automates MCD VM stuff using Podman lib.",
		Long:  "",
	}

	startCmd = &cobra.Command{
		Use: "start",
		RunE: func(_ *cobra.Command, args []string) error {
			return tryStartWithShim(initOpts)
		},
	}

	stopCmd = &cobra.Command{
		Use: "stop",
		RunE: func(_ *cobra.Command, args []string) error {
			return tryStopWithShim(initOpts)
		},
	}

	restartCmd = &cobra.Command{
		Use: "restart",
		RunE: func(_ *cobra.Command, args []string) error {
			if err := tryStopWithShim(initOpts); err != nil {
				return err
			}

			return tryStartWithShim(initOpts)
		},
	}

	//bootImagesCmd = &cobra.Command{
	//	Use: "bootimages",
	//	RunE: func(_ *cobra.Command, args []string) error {
	//		_, err := getOSImageFromClusterConfigMap()
	//		return err
	//	},
	//}
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	//rootCmd.AddCommand(bootImagesCmd)
}

func main() {
	os.Exit(cli.Run(rootCmd))
}

type shimOpts struct {
	InitOptions define.InitOptions
	Dirs        *define.MachineDirs
	Provider    vmconfigs.VMProvider
}

func newShimOpts(initOpts define.InitOptions) (shimOpts, error) {
	provider, err := provider2.Get()
	if err != nil {
		return shimOpts{}, err
	}

	dirs, err := env.GetMachineDirs(provider.VMType())
	if err != nil {
		return shimOpts{}, err
	}

	return shimOpts{
		InitOptions: initOpts,
		Dirs:        dirs,
		Provider:    provider,
	}, nil
}

var (
	emptyIgnCfg ign3types.Config = ctrlcommon.NewIgnConfig()
)

func getIgnitionConfigFromRunningCluster() (ign3types.Config, *mcfgv1.MachineConfig, error) {
	cs := framework.NewClientSet("")

	mcp, err := cs.MachineconfigurationV1Interface.MachineConfigPools().Get(context.TODO(), "worker", metav1.GetOptions{})
	if err != nil {
		return emptyIgnCfg, nil, err
	}

	mc, err := cs.MachineconfigurationV1Interface.MachineConfigs().Get(context.TODO(), mcp.Spec.Configuration.Name, metav1.GetOptions{})
	if err != nil {
		return emptyIgnCfg, nil, err
	}

	ign, err := ctrlcommon.ParseAndConvertConfig(mc.Spec.Config.Raw)
	return ign, mc, err
}

func getGeneratedIgnitionConfig(mc *vmconfigs.MachineConfig) (ign3types.Config, error) {
	ignFile, err := mc.IgnitionFile()
	if err != nil {
		return emptyIgnCfg, err
	}

	ignBytes, err := os.ReadFile(ignFile.Path)
	if err != nil {
		return emptyIgnCfg, err
	}

	return ctrlcommon.ParseAndConvertConfig(ignBytes)
}

func mergeIgnitionConfigs(clusterIgnConfig, generatedIgnConfig ign3types.Config, mc *mcfgv1.MachineConfig) (ign3types.Config, error) {
	merged := ctrlcommon.NewIgnConfig()
	merged.Passwd.Users = generatedIgnConfig.Passwd.Users
	merged.Passwd.Users[0].SSHAuthorizedKeys = append(clusterIgnConfig.Passwd.Users[0].SSHAuthorizedKeys, generatedIgnConfig.Passwd.Users[0].SSHAuthorizedKeys...)
	merged.Storage.Files = append(clusterIgnConfig.Storage.Files, generatedIgnConfig.Storage.Files...)

	clusterUnitNames := loadUnitsIntoSet(clusterIgnConfig.Systemd)
	merged.Systemd.Units = clusterIgnConfig.Systemd.Units

	for _, unit := range generatedIgnConfig.Systemd.Units {
		if !clusterUnitNames.Has(unit.Name) {
			merged.Systemd.Units = append(merged.Systemd.Units, unit)
		}
	}

	ignJSONBytes, err := json.Marshal(merged)
	if err != nil {
		return emptyIgnCfg, err
	}

	mc.Spec.Config.Raw = ignJSONBytes

	mcJSONBytes, err := json.Marshal(mc)
	if err != nil {
		return emptyIgnCfg, err
	}

	filename := "/etc/ignition-machine-config-encapsulated.json"
	merged.Storage.Files = append(merged.Storage.Files, ctrlcommon.NewIgnFileBytes(filename, mcJSONBytes))

	return merged, nil
}

func loadUnitsIntoSet(systemd ign3types.Systemd) sets.Set[string] {
	out := sets.Set[string]{}

	for _, unit := range systemd.Units {
		out.Insert(unit.Name)
	}

	return out
}

func replaceIgnitionFile(mc *vmconfigs.MachineConfig) error {
	clusterIgnConfig, ocpMC, err := getIgnitionConfigFromRunningCluster()
	if err != nil {
		return err
	}

	generatedIgnConfig, err := getGeneratedIgnitionConfig(mc)
	if err != nil {
		return err
	}

	merged, err := mergeIgnitionConfigs(clusterIgnConfig, generatedIgnConfig, ocpMC)
	if err != nil {
		return err
	}

	mergedBytes, err := json.Marshal(merged)
	if err != nil {
		return err
	}

	if _, err := ctrlcommon.ParseAndConvertConfig(mergedBytes); err != nil {
		return fmt.Errorf("meregd ignition could not be parsed: %w", err)
	}

	ignFile, err := mc.IgnitionFile()
	if err != nil {
		return err
	}

	if err := os.WriteFile(ignFile.Path, mergedBytes, 0o755); err != nil {
		return err
	}

	klog.Infof("Ignition file replaced")

	return nil
}

func getQEMUOSImagePathFromClusterConfigMap() (string, error) {
	stream, err := getOSImageFromClusterConfigMap()
	if err != nil {
		return "", err
	}

	// Get our current architecture.
	arch, err := stream.GetArchitecture("x86_64")
	if err != nil {
		return "", err
	}

	return arch.Artifacts["qemu"].Formats["qcow2.gz"].Disk.Location, nil
}

func getOSImageFromClusterConfigMap() (*stream.Stream, error) {
	// https://github.com/coreos/stream-metadata-go/tree/main/stream
	cs := framework.NewClientSet("")

	cm, err := cs.CoreV1Interface.ConfigMaps(ctrlcommon.MCONamespace).Get(context.TODO(), "coreos-bootimages", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// First, convert to bytes:
	cmDataBytes := []byte(cm.Data["stream"])

	// Next, try to deserialize into the stream struct
	out := &stream.Stream{}

	if err := json.Unmarshal(cmDataBytes, out); err != nil {
		return nil, err
	}

	return out, nil
}

func tryStartWithShim(initOpts define.InitOptions) error {
	cleanupWithPodman(initOpts)

	shimOpts, err := newShimOpts(initOpts)
	if err != nil {
		return fmt.Errorf("could not instantiate shimOpts: %w", err)
	}

	qemuImageURL, err := getQEMUOSImagePathFromClusterConfigMap()
	if err != nil {
		return err
	}

	initOpts.Image = qemuImageURL
	shimOpts.InitOptions.Image = qemuImageURL

	mc, exists, err := shim.VMExists(initOpts.Name, []vmconfigs.VMProvider{shimOpts.Provider})
	if err != nil {
		return fmt.Errorf("could not determine if VM already exists: %w", err)
	}

	if !exists {
		if err := shim.Init(shimOpts.InitOptions, shimOpts.Provider); err != nil {
			return fmt.Errorf("could not initialize machine %s: %w", shimOpts.InitOptions.Name, err)
		}

		klog.Infof("VM initialized")
	} else {
		klog.Infof("Using preexisting VM with same name")
	}

	if mc == nil {
		loaded, err := vmconfigs.LoadMachineByName(initOpts.Name, shimOpts.Dirs)
		if err != nil {
			return err
		}

		mc = loaded
	}

	if err := replaceIgnitionFile(mc); err != nil {
		return err
	}

	klog.Infof("Starting VM...")

	if err := shim.Start(mc, shimOpts.Provider, shimOpts.Dirs, machine.StartOptions{}); err != nil {
		return fmt.Errorf("could not start machine %s: %w", shimOpts.InitOptions.Name, err)
	}

	spew.Dump(shimOpts.Provider.State(mc, false))

	klog.Infof("VM started!")

	return nil
}

func tryStopWithShim(initOpts define.InitOptions) error {
	shimOpts, err := newShimOpts(initOpts)
	if err != nil {
		return fmt.Errorf("could not instantiate shimOpts: %w", err)
	}

	mc, exists, err := shim.VMExists(initOpts.Name, []vmconfigs.VMProvider{shimOpts.Provider})
	if err != nil {
		return fmt.Errorf("could not determine if VM already exists: %w", err)
	}

	if !exists {
		klog.Infof("VM does not exist, nothing to do")
		return nil
	}

	klog.Infof("Stopping VM...")

	if err := shim.Stop(mc, shimOpts.Provider, shimOpts.Dirs, false); err != nil {
		return fmt.Errorf("machine could not be stopped: %w", err)
	}

	klog.Infof("VM stopped")

	if err := shim.Remove(mc, shimOpts.Provider, shimOpts.Dirs, machine.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("could nto remove machine %s: %w", shimOpts.InitOptions.Name, err)
	}

	klog.Infof("VM removed")

	return nil
}

func cleanupWithPodman(initOpts define.InitOptions) error {
	klog.Infof("Cleaning up using podman")
	cmd := exec.Command("podman", "machine", "rm", "--log-level=debug", "-f", initOpts.Name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		klog.Errorf("Got %s when running %s", err.Error(), cmd.String())
	}

	return err
}
