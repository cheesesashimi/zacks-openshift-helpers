package main

import (
	"fmt"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/containers"
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/rollout"
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
	"k8s.io/klog"
)

var (
	replaceCmd = &cobra.Command{
		Use:   "replace",
		Short: "Replaces the MCO image with the provided container image pullspec",
		Long:  "",
		Run:   runReplaceCmd,
	}

	validatePullspec bool
)

func init() {
	rootCmd.AddCommand(replaceCmd)
	replaceCmd.PersistentFlags().BoolVar(&validatePullspec, "validate-pullspec", false, "Ensures that the supplied pullspec exists.")
}

func runReplaceCmd(_ *cobra.Command, args []string) {
	if err := replace(args); err != nil {
		klog.Fatalln(err)
	}
}

func replace(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no pullspec provided")
	}

	if len(args) > 1 {
		return fmt.Errorf("only one pullspec may be provided")
	}

	pullspec := args[0]

	if validatePullspec {
		digestedPullspec, err := containers.ResolveToDigestedPullspec(pullspec, "")
		if err != nil {
			return fmt.Errorf("could not validate pullspec %s: %w", pullspec, err)
		}

		klog.Infof("Resolved to %s to validate that the pullspec exists", digestedPullspec)
	}

	cs := framework.NewClientSet("")
	if err := rollout.ReplaceMCOImage(cs, pullspec); err != nil {
		return err
	}

	klog.Infof("Successfully replaced the stock MCO image with %s.", pullspec)
	return nil
}
