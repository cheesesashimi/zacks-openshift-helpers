package main

import (
	"flag"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/component-base/cli"

	versioncmd "github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/version"
)

const (
	defaultUser       string = "$USER"
	defaultWorkDir    string = "$HOME/.openshift-installer"
	defaultSSHKeyPath string = "$HOME/.ssh/id_ed25519.pub"
	// TODO: There are better ways to infer this in the github.com/containers repo.
	defaultPullSecretPath string = "$HOME/.docker/config.json"
)

var (
	version = "not-built-properly"
	commit  = "not-built-properly"
	date    = "not-built-properly"
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
	rootCmd.AddCommand(versioncmd.Command(version, commit, date))
}

func main() {
	os.Exit(cli.Run(rootCmd))
}
