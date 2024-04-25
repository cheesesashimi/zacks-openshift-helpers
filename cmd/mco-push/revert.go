package main

import (
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/rollout"
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
	"k8s.io/klog"
)

var (
	revertCmd = &cobra.Command{
		Use:   "revert",
		Short: "Reverts the MCO image to the one in the OpenShift release",
		Long:  "",
		Run:   runRevertCmd,
	}
)

func init() {
	rootCmd.AddCommand(revertCmd)
	revertCmd.PersistentFlags().BoolVar(&forceRestart, "force", false, "Deletes the pods to forcefully restart the MCO.")
}

func runRevertCmd(_ *cobra.Command, _ []string) {
	if err := revert(); err != nil {
		klog.Fatalln(err)
	}
}

func revert() error {
	cs := framework.NewClientSet("")
	if err := rollout.RevertToOriginalMCOImage(cs, forceRestart); err != nil {
		return err
	}

	klog.Infof("Successfully rolled back to the original MCO image")
	return nil
}
