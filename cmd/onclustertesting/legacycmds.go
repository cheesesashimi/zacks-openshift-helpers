package main

import (
	"os"

	"github.com/cheesesashimi/zacks-openshift-helpers/cmd/onclustertesting/internal/legacycmds"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

func addCommandIfEnvVarIsSet(envVarName string, cmdFunc func() *cobra.Command) {
	if _, ok := os.LookupEnv(envVarName); !ok {
		return
	}

	cmd := cmdFunc()

	klog.Warningf("The %q command is deprecated and will be removed in a future release!", cmd.Use)

	rootCmd.AddCommand(cmd)
}

func init() {
	addCommandIfEnvVarIsSet("ENABLE_SET_IMAGE_COMMAND", legacycmds.SetImageCommand)
	addCommandIfEnvVarIsSet("ENABLE_SET_STATUS_COMMAND", legacycmds.SetStatusCommand)
	addCommandIfEnvVarIsSet("ENABLE_EXTRACT_COMMAND", legacycmds.ExtractCommand)
	addCommandIfEnvVarIsSet("ENABLE_CLEAR_STATUS_COMMAND", legacycmds.ClearStatusCommand)
	addCommandIfEnvVarIsSet("ENABLE_MACHINECONFIG_COMMAND", legacycmds.MachineConfigCommand)
	addCommandIfEnvVarIsSet("ENABLE_RENDER_COMMAND", legacycmds.RenderCommand)
}
