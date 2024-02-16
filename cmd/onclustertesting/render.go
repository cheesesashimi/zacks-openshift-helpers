package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	renderCmd = &cobra.Command{
		Use:   "render",
		Short: "Renders the on-cluster build Dockerfile to disk",
		Long:  "",
		RunE:  runRenderCmd,
	}

	renderOpts struct {
		poolName             string
		includeMachineConfig bool
		targetDir            string
	}
)

func init() {
	rootCmd.AddCommand(renderCmd)
	renderCmd.PersistentFlags().StringVar(&renderOpts.poolName, "pool", defaultLayeredPoolName, "Pool name to render")
	renderCmd.PersistentFlags().StringVar(&renderOpts.targetDir, "dir", "", "Dir to store rendered Dockerfile and MachineConfig in")
}

func runRenderCmd(_ *cobra.Command, _ []string) error {
	common(renderOpts)

	if renderOpts.poolName == "" {
		return fmt.Errorf("no pool name provided")
	}

	cs := framework.NewClientSet("")

	targetDir, err := getDir(renderOpts.targetDir)
	if err != nil {
		return err
	}

	dir := filepath.Join(targetDir, renderOpts.poolName)

	if err := renderDockerfileToDisk(cs, renderOpts.poolName, dir); err != nil {
		return err
	}

	mcp, err := cs.MachineConfigPools().Get(context.TODO(), renderOpts.poolName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	return storeMachineConfigOnDisk(cs, mcp, dir)
}
