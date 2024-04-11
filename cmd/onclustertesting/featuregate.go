package main

import (
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
)

var (
	enableFeatureGateCmd = &cobra.Command{
		Use:   "enable-featuregate",
		Short: "Enables the appropriate feature gates for on=cluster layering to work",
		Long:  "",
		RunE:  runEnableFeatureGateCmd,
	}
)

func init() {
	rootCmd.AddCommand(enableFeatureGateCmd)
}

func runEnableFeatureGateCmd(_ *cobra.Command, _ []string) error {
	return enableFeatureGate(framework.NewClientSet(""))
}
