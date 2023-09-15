package main

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/errors"
	routeClient "github.com/openshift/client-go/route/clientset/versioned"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/openshift/machine-config-operator/test/framework"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	k8stypes "k8s.io/apimachinery/pkg/types"
)

func setupForPushIntoClusterAPI(cs *framework.ClientSet) error {
	kubeconfig, err := cs.GetKubeconfig()
	if err != nil {
		return err
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}

	rc := routeClient.NewForConfigOrDie(config)

	imageRegistryNamespace := "openshift-image-registry"

	_, err = rc.RouteV1().Routes(imageRegistryNamespace).Get(context.TODO(), "image-registry", metav1.GetOptions{})
	if err != nil && !apierrs.IsNotFound(err) {
		return err
	}

	if apierrs.IsNotFound(err) {
		cmd := exec.Command("oc", "expose", "-n", imageRegistryNamespace, "svc/image-registry")
		if out, err := cmd.CombinedOutput(); err != nil {
			return errors.NewExecError(cmd, out, err)
		}
	}

	_, err = rc.RouteV1().Routes(imageRegistryNamespace).Get(context.TODO(), "image-registry", metav1.GetOptions{})

	registryPatchSpec := []byte(`{"spec": {"tls": {"insecureEdgeTerminationPolicy": "Redirect", "termination": "reencrypt"}}}`)

	_, err = rc.RouteV1().Routes(imageRegistryNamespace).Patch(context.TODO(), "image-registry", k8stypes.MergePatchType, registryPatchSpec, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("could not patch image-registry: %w", err)
	}

	cmd := exec.Command("oc", "-n", ctrlcommon.MCONamespace, "policy", "add-role-to-group", "registry-viewer", "system:anonymous")
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.NewExecError(cmd, out, err)
	}

	imgRegistryRoute, err := rc.RouteV1().Routes(imageRegistryNamespace).Get(context.TODO(), "image-registry", metav1.GetOptions{})
	if err != nil {
		return err
	}

	registryHostName := imgRegistryRoute.Spec.Host
	klog.Infof("Got %s for registry hostname", registryHostName)
	return err
}

func teardownPushIntoCluster(cs *framework.ClientSet) error {
	kubeconfig, err := cs.GetKubeconfig()
	if err != nil {
		return err
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}

	rc := routeClient.NewForConfigOrDie(config)

	imageRegistryNamespace := "openshift-image-registry"

	if err := rc.RouteV1().Routes(imageRegistryNamespace).Delete(context.TODO(), "image-registry", metav1.DeleteOptions{}); err != nil {
		return err
	}

	if err := cs.Services(imageRegistryNamespace).Delete(context.TODO(), "image-registry", metav1.DeleteOptions{}); err != nil {
		return err
	}

	cmd := exec.Command("oc", "-n", ctrlcommon.MCONamespace, "policy", "remove-role-from-group", "registry-viewer", "system:anonymous")
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.NewExecError(cmd, out, err)
	}

	return nil
}
