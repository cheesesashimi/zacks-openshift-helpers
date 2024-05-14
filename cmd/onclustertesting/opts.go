package main

import (
	"fmt"
	"os"

	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	"k8s.io/klog/v2"
)

type opts struct {
	pushSecretName              string
	pullSecretName              string
	pushSecretPath              string
	pullSecretPath              string
	finalImagePullspec          string
	containerfilePath           string
	poolName                    string
	injectYumRepos              bool
	waitForBuildInfo            bool
	copyEtcPkiEntitlementSecret bool
	enableFeatureGate           bool
}

func (o *opts) deepCopy() opts {
	return opts{
		pushSecretName:              o.pushSecretName,
		pullSecretName:              o.pullSecretName,
		pushSecretPath:              o.pushSecretPath,
		pullSecretPath:              o.pullSecretPath,
		finalImagePullspec:          o.finalImagePullspec,
		containerfilePath:           o.containerfilePath,
		poolName:                    o.poolName,
		injectYumRepos:              o.injectYumRepos,
		waitForBuildInfo:            o.waitForBuildInfo,
		copyEtcPkiEntitlementSecret: o.copyEtcPkiEntitlementSecret,
		enableFeatureGate:           o.enableFeatureGate,
	}
}

func (o *opts) getContainerfileContent() (string, error) {
	if o.containerfilePath == "" {
		return "", fmt.Errorf("no custom Containerfile path provided")
	}

	containerfileBytes, err := os.ReadFile(o.containerfilePath)
	if err != nil {
		return "", fmt.Errorf("cannot read Containerfile from %s: %w", o.containerfilePath, err)
	}

	klog.Infof("Using contents in Containerfile %q for %s custom Containerfile", o.containerfilePath, o.poolName)
	return string(containerfileBytes), nil
}

func (o *opts) maybeGetContainerfileContent() (string, error) {
	if o.containerfilePath == "" {
		return "", nil
	}

	return o.getContainerfileContent()
}

func (o *opts) shouldCloneGlobalPullSecret() bool {
	return isNoneSet(o.pullSecretName, o.pullSecretPath)
}

func (o *opts) toMachineOSConfig() (*mcfgv1alpha1.MachineOSConfig, error) {
	pushSecretName, err := o.getPushSecretName()
	if err != nil {
		return nil, err
	}

	pullSecretName, err := o.getPullSecretName()
	if err != nil {
		return nil, err
	}

	containerfileContents, err := o.maybeGetContainerfileContent()
	if err != nil {
		return nil, err
	}

	moscOpts := moscOpts{
		poolName:              o.poolName,
		containerfileContents: containerfileContents,
		pullSecretName:        pullSecretName,
		pushSecretName:        pushSecretName,
		finalImagePullspec:    o.finalImagePullspec,
	}

	return newMachineOSConfig(moscOpts), nil
}

func (o *opts) getPullSecretName() (string, error) {
	if o.shouldCloneGlobalPullSecret() {
		return globalPullSecretCloneName, nil
	}

	if o.pullSecretName != "" {
		return o.pullSecretName, nil
	}

	return getSecretNameFromFile(o.pullSecretPath)
}

func (o *opts) getPushSecretName() (string, error) {
	if o.pushSecretName != "" {
		return o.pushSecretName, nil
	}

	return getSecretNameFromFile(o.pushSecretPath)
}

func (o *opts) getSecretNameParams() []string {
	secretNames := []string{}

	if o.pullSecretName != "" {
		secretNames = append(secretNames, o.pullSecretName)
	}

	if o.pushSecretName != "" {
		secretNames = append(secretNames, o.pushSecretName)
	}

	return secretNames
}
