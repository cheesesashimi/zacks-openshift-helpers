package main

import (
	"flag"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/component-base/cli"

	versioncmd "github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/version"
)

var (
	rootCmd = &cobra.Command{
		Use:   "cluster-lifecycle",
		Short: "Helps bring up and tear down an OpenShift cluster for testing and development purposes",
		Long:  "",
	}
)

func init() {
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	rootCmd.AddCommand(versioncmd.Command())
}

func main() {
	os.Exit(cli.Run(rootCmd))
}
