package main

import (
	"flag"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/component-base/cli"
)

const (
	defaultUser       string = "$USER"
	defaultWorkDir    string = "$HOME/.openshift-installer"
	defaultSSHKeyPath string = "$HOME/.ssh/id_ed25519.pub"
	// TODO: There are better ways to infer this in the github.com/containers repo.
	defaultPullSecretPath string = "$HOME/.docker/config.json"
)

var (
	releaseArches = []string{"amd64", "arm64", "multi"}
	releaseKinds  = []string{"ocp", "okd", "okd-scos"}

	rootCmd = &cobra.Command{
		Use:   "cluster-lifecycle",
		Short: "Helps bring up and tear down an OpenShift cluster for testing and development purposes",
		Long:  "",
	}
)

func init() {
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
}

func main() {
	os.Exit(cli.Run(rootCmd))
}
