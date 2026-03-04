package main

import (
	"fmt"
	"os"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"github.com/spf13/cobra"
)

var controller string

var rootCmd = &cobra.Command{
	Use:   "rcctl",
	Short: "rcctl is a CLI for viewing releasestream information from an OpenShift release controller",
	Long: `
The intent of this CLI tool is that it will be used as part of scripts and
other automation which rely on querying the release controller. Therefore, all
data returned from it will be returned as JSON to stdout.`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&controller, "controller", string(releasecontroller.Amd64OcpReleaseController), "Override the default release controller")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
