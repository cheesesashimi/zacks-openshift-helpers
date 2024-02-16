package main

import (
	"fmt"

	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
)

var (
	extractCmd = &cobra.Command{
		Use:   "extract",
		Short: "Extracts the Dockerfile and MachineConfig from an on-cluster build",
		Long:  "",
		RunE:  runExtractCmd,
	}

	extractOpts struct {
		poolName      string
		machineConfig string
		targetDir     string
		noConfigMaps  bool
	}
)

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.PersistentFlags().StringVar(&extractOpts.poolName, "pool", defaultLayeredPoolName, "Pool name to extract")
	extractCmd.PersistentFlags().StringVar(&extractOpts.machineConfig, "machineconfig", "", "MachineConfig name to extract")
	extractCmd.PersistentFlags().StringVar(&extractOpts.targetDir, "dir", "", "Dir to store extract build objects")
}

func runExtractCmd(_ *cobra.Command, _ []string) error {
	common(extractOpts)

	if extractOpts.poolName != "" && extractOpts.machineConfig != "" {
		return fmt.Errorf("either pool name or MachineConfig must be provided; not both")
	}

	targetDir, err := getDir(extractOpts.targetDir)
	if err != nil {
		return err
	}

	cs := framework.NewClientSet("")

	if extractOpts.machineConfig != "" {
		return extractBuildObjectsForRenderedMC(cs, extractOpts.machineConfig, targetDir)
	}

	if extractOpts.poolName != "" {
		return extractBuildObjectsForTargetPool(cs, extractOpts.poolName, targetDir)
	}

	return fmt.Errorf("no pool name or MachineConfig name provided")
}
