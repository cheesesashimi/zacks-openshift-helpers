package main

import (
	"fmt"

	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
)

var (
	clearStatusCmd = &cobra.Command{
		Use:   "clear-build-status",
		Short: "Tears down the pool for on-cluster build testing",
		Long:  "",
		RunE:  runClearStatusCmd,
	}

	clearStatusOpts struct {
		poolName string
	}
)

func init() {
	rootCmd.AddCommand(clearStatusCmd)
	clearStatusCmd.PersistentFlags().StringVar(&clearStatusOpts.poolName, "pool", defaultLayeredPoolName, "Pool name to clear build status on")
}

func runClearStatusCmd(_ *cobra.Command, _ []string) error {
	common(clearStatusOpts)

	if clearStatusOpts.poolName == "" {
		return fmt.Errorf("no pool name provided")
	}

	return clearBuildStatusesOnPool(framework.NewClientSet(""), clearStatusOpts.poolName)
}
