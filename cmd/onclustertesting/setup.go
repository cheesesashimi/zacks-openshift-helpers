package main

import (
	"context"
	"fmt"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/utils"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

var (
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Sets up pool for on-cluster build testing",
		Long:  "",
		RunE:  runSetupCmd,
	}

	inClusterRegistryCmd = &cobra.Command{
		Use:   "in-cluster-registry",
		Short: "Sets up pool for on-cluster build testing using an ImageStream",
		Long:  "",
		RunE:  runInClusterRegistrySetupCmd,
	}

	setupOpts struct {
		dockerfilePath     string
		pullSecretPath     string
		pushSecretPath     string
		pullSecretName     string
		pushSecretName     string
		finalImagePullspec string
		poolName           string
		waitForBuildInfo   bool
		enableFeatureGate  bool
	}
)

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.AddCommand(inClusterRegistryCmd)
	setupCmd.PersistentFlags().StringVar(&setupOpts.poolName, "pool", defaultLayeredPoolName, "Pool name to setup")
	setupCmd.PersistentFlags().BoolVar(&setupOpts.waitForBuildInfo, "wait-for-build", false, "Wait for build info")
	setupCmd.PersistentFlags().StringVar(&setupOpts.pullSecretName, "pull-secret-name", "", "The name of a preexisting secret to use as the pull secret. If absent, will clone global pull secret.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.pushSecretName, "push-secret-name", "", "The name of a preexisting secret to use as the push secret.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.pullSecretPath, "pull-secret-path", "", "Path to a pull secret K8s YAML to use. If absent, will clone global pull secret.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.pushSecretPath, "push-secret-path", "", "Path to a push secret K8s YAML to use.")
	setupCmd.PersistentFlags().StringVar(&setupOpts.finalImagePullspec, "final-pullspec", "", "The final image pushspec to use for testing")
	setupCmd.PersistentFlags().StringVar(&setupOpts.dockerfilePath, "dockerfile-path", "", "Optional Dockerfile to inject for the build.")
	setupCmd.PersistentFlags().BoolVar(&setupOpts.enableFeatureGate, "enable-feature-gate", false, "Enables the required featuregates if not already enabled.")
}

func runSetupCmd(_ *cobra.Command, _ []string) error {
	utils.ParseFlags()

	// TODO: Figure out how to use cobra flags for validation directly.
	if err := errIfNotSet(setupOpts.poolName, "pool"); err != nil {
		return err
	}

	if err := errIfNotSet(setupOpts.finalImagePullspec, "final-pullspec"); err != nil {
		return err
	}

	if isNoneSet(setupOpts.pushSecretPath, setupOpts.pushSecretName) {
		return fmt.Errorf("either --push-secret-name or --push-secret-path must be provided")
	}

	if !isOnlyOneSet(setupOpts.pushSecretPath, setupOpts.pushSecretName) {
		return fmt.Errorf("flags --pull-secret-name and --pull-secret-path cannot be combined")
	}

	if !isOnlyOneSet(setupOpts.pullSecretPath, setupOpts.pullSecretName) {
		return fmt.Errorf("flags --push-secret-name and --push-secret-path cannot be combined")
	}

	cs := framework.NewClientSet("")

	if err := checkForRequiredFeatureGates(cs); err != nil {
		return err
	}

	return mobSetup(cs, setupOpts.waitForBuildInfo, onClusterBuildConfigMapOpts{
		pushSecretName:     setupOpts.pushSecretName,
		pullSecretName:     setupOpts.pullSecretName,
		pushSecretPath:     setupOpts.pushSecretPath,
		pullSecretPath:     setupOpts.pullSecretPath,
		finalImagePullspec: setupOpts.finalImagePullspec,
		dockerfilePath:     setupOpts.dockerfilePath,
		pool:               setupOpts.poolName,
	})
}

func runInClusterRegistrySetupCmd(_ *cobra.Command, _ []string) error {
	utils.ParseFlags()

	if err := errIfNotSet(setupOpts.poolName, "pool"); err != nil {
		return err
	}

	cs := framework.NewClientSet("")

	if err := checkForRequiredFeatureGates(cs); err != nil {
		return err
	}

	pushSecretName, err := getBuilderPushSecretName(cs)
	if err != nil {
		return err
	}

	imagestreamName := "os-image"
	if err := createImagestream(cs, imagestreamName); err != nil {
		return err
	}

	pullspec, err := getImagestreamPullspec(cs, imagestreamName)
	if err != nil {
		return err
	}

	return mobSetup(cs, setupOpts.waitForBuildInfo, onClusterBuildConfigMapOpts{
		pushSecretName:     pushSecretName,
		finalImagePullspec: pullspec,
		dockerfilePath:     setupOpts.dockerfilePath,
		pool:               setupOpts.poolName,
	})
}

func mobSetup(cs *framework.ClientSet, getBuildInfo bool, cmOpts onClusterBuildConfigMapOpts) error {
	eg := errgroup.Group{}

	eg.Go(func() error {
		_, err := createPool(cs, cmOpts.pool)
		return err
	})

	eg.Go(func() error {
		return createSecrets(cs, cmOpts)
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	mosc, err := cmOpts.toMachineOSConfig()
	if err != nil {
		return err
	}

	if err := createMachineOSConfig(cs, mosc); err != nil {
		return err
	}

	if !getBuildInfo {
		return nil
	}

	return waitForBuildInfo(cs, cmOpts.pool)
}

func checkForRequiredFeatureGates(cs *framework.ClientSet) error {
	if err := validateFeatureGatesEnabled(cs, "OnClusterBuild"); err != nil {
		if setupOpts.enableFeatureGate {
			return enableFeatureGate(cs)
		}

		prompt := `You may need to enable TechPreview feature gates on your cluster. Try the following: $ oc patch featuregate/cluster --type=merge --patch='{"spec":{"featureSet":"TechPreviewNoUpgrade"}}'`
		klog.Infof(prompt)
		klog.Infof("Alternatively, rerun this command with the --enable-feature-gate flag")
		return err
	}

	return nil
}

func enableFeatureGate(cs *framework.ClientSet) error {
	fg, err := cs.ConfigV1Interface.FeatureGates().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("could not enable feature gate(s): %w", err)
	}

	fg.Spec.FeatureSet = "TechPreviewNoUpgrade"

	_, err = cs.ConfigV1Interface.FeatureGates().Update(context.TODO(), fg, metav1.UpdateOptions{})
	if err == nil {
		klog.Infof("Enabled FeatureGate %s", fg.Spec.FeatureSet)
	}

	return err
}

// Cribbed from: https://github.com/openshift/machine-config-operator/blob/master/test/helpers/utils.go
func validateFeatureGatesEnabled(cs *framework.ClientSet, requiredFeatureGates ...configv1.FeatureGateName) error {
	currentFeatureGates, err := cs.ConfigV1Interface.FeatureGates().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to fetch feature gates: %w", err)
	}

	// This uses the new Go generics to construct a typed set of
	// FeatureGateNames. Under the hood, sets are map[T]struct{}{} where
	// only the keys matter and one cannot have duplicate keys. Perfect for our use-case!
	enabledFeatures := sets.New[configv1.FeatureGateName]()
	disabledFeatures := sets.New[configv1.FeatureGateName]()

	// Load all of the feature gate names into our set. Duplicates will be
	// automatically be ignored.
	for _, currentFeatureGateDetails := range currentFeatureGates.Status.FeatureGates {
		for _, enabled := range currentFeatureGateDetails.Enabled {
			enabledFeatures.Insert(enabled.Name)
		}

		for _, disabled := range currentFeatureGateDetails.Disabled {
			disabledFeatures.Insert(disabled.Name)
		}
	}

	// If we have all of the required feature gates, we're done!
	if enabledFeatures.HasAll(requiredFeatureGates...) && !disabledFeatures.HasAny(requiredFeatureGates...) {
		klog.Infof("All required feature gates %v are enabled", requiredFeatureGates)
		return nil
	}

	// Now, lets validate that our FeatureGates are just disabled and not unknown.
	requiredFeatures := sets.New[configv1.FeatureGateName](requiredFeatureGates...)
	allFeatures := enabledFeatures.Union(disabledFeatures)
	if !allFeatures.HasAll(requiredFeatureGates...) {
		return fmt.Errorf("unknown FeatureGate(s): %v, available FeatureGate(s): %v", sets.List(requiredFeatures.Difference(allFeatures)), sets.List(allFeatures))
	}

	// If we don't, lets diff against what we have vs. what we want and return that information.
	disabledRequiredFeatures := requiredFeatures.Difference(enabledFeatures)
	return fmt.Errorf("required FeatureGate(s) %v not enabled; have: %v", sets.List(disabledRequiredFeatures), sets.List(enabledFeatures))
}

func createSecrets(cs *framework.ClientSet, opts onClusterBuildConfigMapOpts) error {
	eg := errgroup.Group{}

	eg.Go(func() error {
		if opts.shouldCloneGlobalPullSecret() {
			if err := copyGlobalPullSecret(cs); err != nil {
				// Not sure why this snarfs any errors from this process.
				return nil
			}
		}

		return nil
	})

	eg.Go(func() error {
		if opts.pushSecretPath != "" {
			if err := createSecretFromFile(cs, opts.pushSecretPath); err != nil {
				return err
			}
		}

		return nil
	})

	eg.Go(func() error {
		if opts.pullSecretPath != "" {
			if err := createSecretFromFile(cs, opts.pullSecretPath); err != nil {
				return err
			}
		}

		return nil
	})

	eg.Go(func() error {
		return copyEtcPkiEntitlementSecret(cs)
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	secretNames := opts.getSecretNameParams()
	if err := validateSecretsExist(cs, secretNames); err != nil {
		return err
	}

	return nil
}

func waitForBuildInfo(_ *framework.ClientSet, _ string) error {
	klog.Infof("no-op for now")
	return nil
}
