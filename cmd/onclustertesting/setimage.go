package main

import (
	"fmt"

	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/retry"
)

var (
	setImageCmd = &cobra.Command{
		Use:   "set-image",
		Short: "Sets an image pullspec on a MachineConfigPool",
		Long:  "",
		RunE:  runSetImageCmd,
	}

	setImageOpts struct {
		poolName  string
		imageName string
	}
)

func init() {
	rootCmd.AddCommand(setImageCmd)
	setImageCmd.PersistentFlags().StringVar(&setImageOpts.poolName, "pool", defaultLayeredPoolName, "Pool name to set build status on")
	setImageCmd.PersistentFlags().StringVar(&setImageOpts.imageName, "image", "", "The image pullspec to set")
}

func runSetImageCmd(_ *cobra.Command, _ []string) error {
	common(setImageOpts)

	if setImageOpts.poolName == "" {
		return fmt.Errorf("no pool name provided")
	}

	if setImageOpts.imageName == "" {
		return fmt.Errorf("no image name provided")
	}

	return setImageOnPool(framework.NewClientSet(""), setImageOpts.poolName, setImageOpts.imageName)
}

func setImageOnPool(cs *framework.ClientSet, targetPool, pullspec string) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := optInPool(cs, targetPool); err != nil {
			return err
		}

		return addImageToLayeredPool(cs, pullspec, targetPool)
	})
}
