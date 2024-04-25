package main

import (
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/rollout"
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
	"k8s.io/klog"
)

var (
	restartCmd = &cobra.Command{
		Use:   "restart",
		Short: "Restarts all of the MCO pods",
		Long:  "",
		Run:   runRestartCmd,
	}

	forceRestart bool
)

func init() {
	rootCmd.AddCommand(restartCmd)
	restartCmd.PersistentFlags().BoolVar(&forceRestart, "force", false, "Deletes the pods to forcefully restart the MCO.")
}

func runRestartCmd(_ *cobra.Command, args []string) {
	if err := restart(args); err != nil {
		klog.Fatalln(err)
	}
}

func restart(_ []string) error {
	cs := framework.NewClientSet("")

	if forceRestart {
		klog.Infof("Will delete pods to force restart")
	}

	if err := rollout.RestartMCO(cs, forceRestart); err != nil {
		return err
	}

	klog.Infof("Successfully restartd the MCO pods")
	return nil
}
