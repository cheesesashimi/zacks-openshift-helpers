package main

import (
	"flag"
	"os"

	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/spf13/cobra"
	"k8s.io/component-base/cli"

	versioncmd "github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/version"
)

const (
	internalRegistryHostname string = "image-registry.openshift-image-registry.svc:5000"
	imagestreamName          string = "machine-config-operator"
	imagestreamPullspec      string = internalRegistryHostname + "/" + ctrlcommon.MCONamespace + "/" + imagestreamName + ":latest"
)

var (
	version = "not-built-properly"
	commit  = "not-built-properly"
	date    = "not-built-properly"
)

var (
	rootCmd = &cobra.Command{
		Use:   "mco-builder",
		Short: "Automates the build and replacement of the machine-config-operator (MCO) image in an OpenShift cluster for testing purposes.",
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
