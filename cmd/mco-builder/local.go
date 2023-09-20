package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cheesesashimi/zacks-openshift-helpers/cmd/mco-builder/builders"
	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/containers"
	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/repo"
	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/rollout"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
	aggerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

type localBuildOpts struct {
	builderKind              string // builders.BuilderType
	finalImagePushSecretPath string
	finalImagePullspec       string
	directPush               bool
	buildMode                string // repo.BuildMode
	repoRoot                 string
	skipRollout              bool
}

func (l *localBuildOpts) getBuilderType() builders.BuilderType {
	return builders.BuilderType(l.builderKind)
}

func (l *localBuildOpts) getBuildMode() repo.BuildMode {
	return repo.BuildMode(l.buildMode)
}

func (l *localBuildOpts) validate() error {
	if l.repoRoot == "" {
		return fmt.Errorf("--repo-root must be provided")
	}

	if _, err := exec.LookPath("oc"); err != nil {
		return err
	}

	if _, err := os.Stat(l.repoRoot); err != nil {
		return err
	}

	localBuilderTypes := builders.GetLocalBuilderTypes()
	if !localBuilderTypes.Has(l.getBuilderType()) {
		return fmt.Errorf("invalid builder type %s, valid builder types: %v", l.getBuilderType(), sets.List(localBuilderTypes))
	}

	if _, err := exec.LookPath(l.builderKind); err != nil {
		return err
	}

	if !buildModes.Has(l.getBuildMode()) {
		return fmt.Errorf("invalid build mode %s, valid build modes: %v", l.buildMode, sets.List(buildModes))
	}

	if l.directPush {
		if l.finalImagePushSecretPath != "" {
			return fmt.Errorf("--push-secret may not be used in direct mode")
		}

		if l.finalImagePullspec != "" {
			return fmt.Errorf("--final-image-pullspec may not be used in direct mode")
		}

		if _, err := exec.LookPath("skopeo"); err != nil {
			return err
		}

		return nil
	}

	if l.finalImagePushSecretPath == "" {
		return fmt.Errorf("--push-secret must be provided when not using direct mode")
	}

	if _, err := os.Stat(l.finalImagePushSecretPath); err != nil {
		return err
	}

	if l.finalImagePullspec == "" {
		return fmt.Errorf("--final-image-pullspec must be provided when not using direct mode")
	}

	return nil
}

var (
	localCmd = &cobra.Command{
		Use:   "local",
		Short: "Builds an MCO image locally and deploys it to your sandbox cluster.",
		Long:  "Builds the MCO image locally using the specified builder and options. Can either push to a remote image registry (such as Quay.io) or can expose a route to enable pushing directly into ones sandbox cluster.",
		RunE:  runLocalCmd,
	}

	buildModes sets.Set[repo.BuildMode]

	opts localBuildOpts
)

func init() {
	buildModes = repo.GetBuildModes().Delete(repo.BuildModeCluster)

	rootCmd.AddCommand(localCmd)
	localCmd.PersistentFlags().BoolVar(&opts.directPush, "direct", false, "Exposes a route and pushes the image directly into ones cluster")
	localCmd.PersistentFlags().StringVar(&opts.repoRoot, "repo-root", "", "Path to the local MCO Git repo")
	localCmd.PersistentFlags().StringVar(&opts.finalImagePushSecretPath, "push-secret", "", "Path to the push secret path needed to push to the provided pullspec (not needed in direct mode)")
	localCmd.PersistentFlags().StringVar(&opts.finalImagePullspec, "final-image-pullspec", "", "Where to push the final image (not needed in direct mode)")
	localCmd.PersistentFlags().StringVar(&opts.buildMode, "build-mode", string(repo.BuildModeFast), fmt.Sprintf("What build mode to use: %v", sets.List(buildModes)))
	localCmd.PersistentFlags().StringVar(&opts.builderKind, "builder", string(builders.GetDefaultBuilderTypeForPlatform()), fmt.Sprintf("What image builder to use: %v", sets.List(builders.GetLocalBuilderTypes())))
	localCmd.PersistentFlags().BoolVar(&opts.skipRollout, "skip-rollout", false, "Builds and pushes the image, but does not update the MCO deployment / daemonset objects")
}

func runLocalCmd(_ *cobra.Command, _ []string) error {
	if err := opts.validate(); err != nil {
		return err
	}

	cs := framework.NewClientSet("")

	if opts.directPush {
		return buildLocallyAndPushIntoCluster(cs, opts)
	}

	return buildLocallyAndDeploy(cs, opts)
}

func buildLocallyAndDeploy(cs *framework.ClientSet, buildOpts localBuildOpts) error {
	// TODO: Return these out of this function.
	deferredErrs := []error{}
	defer func() {
		if err := aggerrs.NewAggregate(deferredErrs); err != nil {
			klog.Fatalf("teardown encountered error(s): %s", err)
		}
	}()

	r, err := repo.NewMCORepo(buildOpts.repoRoot, buildOpts.getBuildMode())
	if err != nil {
		return err
	}

	if err := r.SetupForBuild(); err != nil {
		return err
	}
	defer func() {
		if err := r.TeardownFromBuild(); err != nil {
			deferredErrs = append(deferredErrs, err)
		}
	}()

	opts := builders.Opts{
		RepoRoot:       buildOpts.repoRoot,
		FinalPullspec:  buildOpts.finalImagePullspec,
		PushSecretPath: buildOpts.finalImagePushSecretPath,
		DockerfileName: r.DockerfilePath(),
	}

	builder := builders.NewLocalBuilder(opts, buildOpts.getBuilderType())

	if err := builder.Build(); err != nil {
		return err
	}

	if err := builder.Push(); err != nil {
		return err
	}

	digestedPullspec, err := containers.ResolveToDigestedPullspec(buildOpts.finalImagePullspec, buildOpts.finalImagePushSecretPath)
	if err != nil {
		return fmt.Errorf("could not resolve %s to digested image pullspec: %w", buildOpts.finalImagePullspec, err)
	}

	klog.Infof("Pushed image has digested pullspec %s", digestedPullspec)

	if buildOpts.skipRollout {
		klog.Infof("Skipping rollout since --skip-rollout was used")
		return nil
	}

	if err := rollout.ReplaceMCOImage(cs, buildOpts.finalImagePullspec); err != nil {
		return err
	}

	klog.Infof("New MCO rollout complete!")
	return nil
}

func buildLocallyAndPushIntoCluster(cs *framework.ClientSet, buildOpts localBuildOpts) error {
	// TODO: Return these out of this function.
	deferredErrs := []error{}
	defer func() {
		if err := aggerrs.NewAggregate(deferredErrs); err != nil {
			klog.Fatalf("encountered error(s) during teardown: %s", err)
		}
	}()

	r, err := repo.NewMCORepo(buildOpts.repoRoot, buildOpts.getBuildMode())
	if err != nil {
		return err
	}

	if err := r.SetupForBuild(); err != nil {
		return err
	}
	defer func() {
		if err := r.TeardownFromBuild(); err != nil {
			deferredErrs = append(deferredErrs, err)
		}
	}()

	extHostname, err := rollout.ExposeClusterImageRegistry(cs)
	if err != nil {
		return err
	}

	klog.Infof("Cluster is set up for direct pushes")

	secretPath, err := writeBuilderSecretToTempDir(cs, extHostname)
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(filepath.Dir(secretPath)); err != nil {
			deferredErrs = append(deferredErrs, err)
		}
	}()

	extPullspec := fmt.Sprintf("%s/%s/machine-config-operator:latest", extHostname, ctrlcommon.MCONamespace)

	opts := builders.Opts{
		RepoRoot:       buildOpts.repoRoot,
		FinalPullspec:  extPullspec,
		PushSecretPath: secretPath,
		DockerfileName: r.DockerfilePath(),
	}

	builder := builders.NewLocalBuilder(opts, buildOpts.getBuilderType())

	if err := builder.Build(); err != nil {
		return err
	}

	if err := builder.Push(); err != nil {
		return err
	}

	digestedPullspec, err := containers.ResolveToDigestedPullspec(extPullspec, secretPath)
	if err != nil {
		return fmt.Errorf("could not resolve %s to digested image pullspec: %w", extPullspec, err)
	}

	klog.Infof("Pushed image has digested pullspec %s", digestedPullspec)

	if buildOpts.skipRollout {
		klog.Infof("Skipping rollout since --skip-rollout was used")
		return nil
	}

	if err := rollout.ReplaceMCOImage(cs, imagestreamPullspec); err != nil {
		return err
	}

	klog.Infof("New MCO rollout complete!")
	return nil
}
