package main

import (
	"context"
	"fmt"
	"os"

	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/openshift/machine-config-operator/test/framework"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type onClusterBuildConfigMapOpts struct {
	pushSecretName              string
	pullSecretName              string
	pushSecretPath              string
	pullSecretPath              string
	finalImagePullspec          string
	dockerfilePath              string
	pool                        string
	injectYumRepos              bool
	copyEtcPkiEntitlementSecret bool
}

func (o *onClusterBuildConfigMapOpts) getDockerfileContent() (string, error) {
	if o.dockerfilePath == "" {
		return "", fmt.Errorf("no custom Dockerfile path provided")
	}

	dockerfileBytes, err := os.ReadFile(o.dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("cannot read Dockerfile from %s: %w", o.dockerfilePath, err)
	}

	klog.Infof("Using contents in Dockerfile %q for %s custom Dockerfile", o.dockerfilePath, o.pool)
	return string(dockerfileBytes), nil
}

func (o *onClusterBuildConfigMapOpts) maybeGetDockerfileContent() (string, error) {
	if o.dockerfilePath == "" {
		return "", nil
	}

	return o.getDockerfileContent()
}

func (o *onClusterBuildConfigMapOpts) shouldCloneGlobalPullSecret() bool {
	return isNoneSet(o.pullSecretName, o.pullSecretPath)
}

func (o *onClusterBuildConfigMapOpts) toMachineOSConfig() (*mcfgv1alpha1.MachineOSConfig, error) {
	pushSecretName, err := o.getPushSecretName()
	if err != nil {
		return nil, err
	}

	pullSecretName, err := o.getPullSecretName()
	if err != nil {
		return nil, err
	}

	dockerfileContents, err := o.maybeGetDockerfileContent()
	if err != nil {
		return nil, err
	}

	mosc := &mcfgv1alpha1.MachineOSConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.pool,
			Labels: map[string]string{
				createdByOnClusterBuildsHelper: "",
			},
		},
		Spec: mcfgv1alpha1.MachineOSConfigSpec{
			MachineConfigPool: mcfgv1alpha1.MachineConfigPoolReference{
				Name: o.pool,
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
						Content:           dockerfileContents,
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

func (o *onClusterBuildConfigMapOpts) getPullSecretName() (string, error) {
	if o.shouldCloneGlobalPullSecret() {
		return globalPullSecretCloneName, nil
	}

	if o.pullSecretName != "" {
		return o.pullSecretName, nil
	}

	return getSecretNameFromFile(o.pullSecretPath)
}

func (o *onClusterBuildConfigMapOpts) getPushSecretName() (string, error) {
	if o.pushSecretName != "" {
		return o.pushSecretName, nil
	}

	return getSecretNameFromFile(o.pushSecretPath)
}

func (o *onClusterBuildConfigMapOpts) getSecretNameParams() []string {
	secretNames := []string{}

	if o.pullSecretName != "" {
		secretNames = append(secretNames, o.pullSecretName)
	}

	if o.pushSecretName != "" {
		secretNames = append(secretNames, o.pushSecretName)
	}

	return secretNames
}

func createConfigMap(cs *framework.ClientSet, cm *corev1.ConfigMap) error { //nolint:dupl // These are ConfigMaps.
	if !hasOurLabel(cm.Labels) {
		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}

		cm.Labels[createdByOnClusterBuildsHelper] = ""
	}

	_, err := cs.CoreV1Interface.ConfigMaps(ctrlcommon.MCONamespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	if err == nil {
		klog.Infof("Created ConfigMap %q in namespace %q", cm.Name, ctrlcommon.MCONamespace)
		return nil
	}

	if err != nil && !apierrs.IsAlreadyExists(err) {
		return err
	}

	configMap, err := cs.CoreV1Interface.ConfigMaps(ctrlcommon.MCONamespace).Get(context.TODO(), cm.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !hasOurLabel(configMap.Labels) {
		klog.Infof("Found preexisting user-supplied ConfigMap %q, using as-is.", cm.Name)
		return nil
	}

	// Delete and recreate.
	klog.Infof("ConfigMap %q was created by us, but could be out of date. Recreating...", cm.Name)
	err = cs.CoreV1Interface.ConfigMaps(ctrlcommon.MCONamespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return createConfigMap(cs, cm)
}
