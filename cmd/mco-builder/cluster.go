package main

import (
	"fmt"
	"os"

	"github.com/cheesesashimi/zacks-openshift-helpers/cmd/mco-builder/builders"
	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/repo"
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

type clusterBuildOpts struct {
	repoRoot    string
	followBuild bool
	skipRollout bool
}

func (c *clusterBuildOpts) validate() error {
	if c.repoRoot == "" {
		return fmt.Errorf("--repo-root must be provided")
	}

	if _, err := os.Stat(c.repoRoot); err != nil {
		return err
	}

	return nil
}

var (
	clusterCmd = &cobra.Command{
		Use:   "cluster",
		Short: "Performs the build operation within the sandbox cluster using an OpenShift Image Build",
		Long:  "",
		RunE:  runClusterCmd,
	}

	clusterOpts clusterBuildOpts
)

func init() {
	rootCmd.AddCommand(clusterCmd)
	clusterCmd.PersistentFlags().BoolVar(&clusterOpts.followBuild, "follow", true, "Stream build logs")
	clusterCmd.PersistentFlags().StringVar(&clusterOpts.repoRoot, "repo-root", "", "Path to the local MCO Git repo")
}

func runClusterCmd(_ *cobra.Command, _ []string) error {
	cs := framework.NewClientSet("")

	if err := clusterOpts.validate(); err != nil {
		return err
	}

	if err := createImagestream(cs, imagestreamName); err != nil {
		return err
	}

	r, err := repo.NewMCORepo(clusterOpts.repoRoot, repo.BuildModeCluster)
	if err != nil {
		return err
	}

	klog.Infof("Using %s as branch name", r.Branch())
	klog.Infof("Using %s as git remote", r.RemoteFork())

	builderOpts := builders.OpenshiftBuilderOpts{
		ImageStreamName: imagestreamName,
		Dockerfile:      string(r.DockerfileContents()),
		BranchName:      r.Branch(),
		RemoteURL:       r.RemoteFork(),
		FollowBuild:     clusterOpts.followBuild,
	}

	builder := builders.NewOpenshiftBuilder(cs, builderOpts)

	if err := builder.Build(); err != nil {
		return fmt.Errorf("could not build in cluster: %w", err)
	}

	if clusterOpts.skipRollout {
		klog.Infof("Skipping rollout because --skip-rollout flag was used")
		return nil
	}

	if err := builder.Push(); err != nil {
		return fmt.Errorf("could not push: %w", err)
	}

	klog.Infof("Build and rollout complete!")

	return nil
}
