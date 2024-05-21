package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/rollout"
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/utils"
	"github.com/openshift/machine-config-operator/test/framework"
	"golang.org/x/sync/errgroup"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
)

const (
	controlPlanePoolName string = "master"
	workerPoolName       string = "worker"
)

func runCiSetupCmd(setupOpts opts) error {
	utils.ParseFlags()

	if setupOpts.injectYumRepos && setupOpts.copyEtcPkiEntitlementSecret {
		return fmt.Errorf("flags --inject-yum-repos and --copy-etc-pki-entitlement cannot be combined")
	}

	cs := framework.NewClientSet("")

	if err := checkForRequiredFeatureGates(cs, setupOpts); err != nil {
		return err
	}

	pushSecretName, err := getBuilderPushSecretName(cs)
	if err != nil {
		return err
	}

	setupOpts.pushSecretName = pushSecretName

	if err := setupForCI(cs, setupOpts); err != nil {
		return err
	}

	if err := waitForBuildsToComplete(cs); err != nil {
		return err
	}

	klog.Infof("Builds complete!")

	delay := time.Minute
	klog.Infof("Sleeping for %s to allow MachineConfigPools to begin rollout process", delay)
	time.Sleep(delay)

	waitTime := time.Minute * 30

	klog.Infof("Waiting up to %s for all MachineConfigPools to complete", waitTime)

	if err := rollout.WaitForAllMachineConfigPoolsToComplete(cs, waitTime); err != nil {
		return err
	}

	klog.Infof("Setup for CI complete!")

	return nil
}

func setupForCI(cs *framework.ClientSet, setupOpts opts) error {
	eg := errgroup.Group{}

	eg.Go(func() error {
		return createSecrets(cs, setupOpts)
	})

	eg.Go(func() error {
		return setupMoscForCI(cs, setupOpts.deepCopy(), workerPoolName)
	})

	eg.Go(func() error {
		return setupMoscForCI(cs, setupOpts.deepCopy(), controlPlanePoolName)
	})

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("could not setup MachineOSConfig for CI test: %w", err)
	}

	return nil
}

func waitForBuildsToComplete(cs *framework.ClientSet) error {
	eg := errgroup.Group{}

	waitTime := time.Minute * 15

	klog.Infof("Waiting up to %s for builds to complete", waitTime)

	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()

	eg.Go(func() error {
		return waitForBuildToComplete(ctx, cs, workerPoolName)
	})

	eg.Go(func() error {
		return waitForBuildToComplete(ctx, cs, controlPlanePoolName)
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func setupMoscForCI(cs *framework.ClientSet, opts opts, poolName string) error {
	opts.poolName = poolName

	if poolName != controlPlanePoolName && poolName != workerPoolName {
		if _, err := createPool(cs, poolName); err != nil {
			return err
		}
	}

	pullspec, err := createImagestreamAndGetPullspec(cs, poolName)
	if err != nil && !apierrs.IsAlreadyExists(err) {
		return err
	}

	opts.finalImagePullspec = pullspec

	mosc, err := opts.toMachineOSConfig()
	if err != nil {
		return err
	}

	if err := createMachineOSConfig(cs, mosc); err != nil {
		return err
	}

	return nil
}
