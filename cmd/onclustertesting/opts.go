package main

import (
	"fmt"
	"os"

	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	mosc := &mcfgv1alpha1.MachineOSConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.poolName,
			Labels: map[string]string{
				createdByOnClusterBuildsHelper: "",
			},
		},
		Spec: mcfgv1alpha1.MachineOSConfigSpec{
			MachineConfigPool: mcfgv1alpha1.MachineConfigPoolReference{
				Name: o.poolName,
			},
			BuildInputs: mcfgv1alpha1.BuildInputs{
				BaseImagePullSecret: mcfgv1alpha1.ImageSecretObjectReference{
					Name: globalPullSecretCloneName,
				},
				RenderedImagePushSecret: mcfgv1alpha1.ImageSecretObjectReference{
					Name: pushSecretName,
				},
				RenderedImagePushspec: o.finalImagePullspec,
				ImageBuilder: &mcfgv1alpha1.MachineOSImageBuilder{
					ImageBuilderType: mcfgv1alpha1.PodBuilder,
				},
				Containerfile: []mcfgv1alpha1.MachineOSContainerfile{
					{
						ContainerfileArch: mcfgv1alpha1.NoArch,
						Content:           containerfileContents,
					},
				},
			},
			BuildOutputs: mcfgv1alpha1.BuildOutputs{
				CurrentImagePullSecret: mcfgv1alpha1.ImageSecretObjectReference{
					Name: pullSecretName,
				},
			},
		},
	}

	return mosc, nil
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
