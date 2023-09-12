package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/cmd/build-in-cluster/builders"
	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/releasecontroller"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/openshift/machine-config-operator/test/framework"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/klog"
)

const (
	hardcodedRepoRoot string = "/Users/zzlotnik/go/src/github.com/openshift/machine-config-operator"
)

func revert() error {
	cs := framework.NewClientSet("")

	clusterVersion, err := cs.ConfigV1Interface.ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		return err
	}

	currentRelease := clusterVersion.Status.Desired.Image
	originalMCOImage, err := releasecontroller.GetComponentPullspecForRelease("machine-config-operator", currentRelease)
	if err != nil {
		return err
	}

	if err := rollout(cs, originalMCOImage); err != nil {
		return err
	}

	return setDeploymentReplicas(cs, "cluster-version-operator", "openshift-cluster-version", 1)
}

func doOpenshiftBuild(cs *framework.ClientSet) (string, error) {
	imagestreamName := "machine-config-operator"

	if err := createImagestream(cs, imagestreamName); err != nil {
		return "", err
	}

	pullspec, err := getImagestreamPullspec(cs, imagestreamName)
	if err != nil {
		return "", err
	}

	klog.Infof("Got pullspec %q for imagestream %q", pullspec, imagestreamName)

	gitInfo, err := getGitInfo(hardcodedRepoRoot)
	if err != nil {
		return "", err
	}

	klog.Infof("Using %s as branch name", gitInfo.branchName)
	klog.Infof("Using %s as git remote", gitInfo.remoteURL)

	builderOpts := builders.OpenshiftBuilderOpts{
		ImageStreamName:     imagestreamName,
		ImageStreamPullspec: pullspec,
		// TODO: Write separate Dockerfile
		Dockerfile: strings.ReplaceAll(string(dockerfile), "make -f Makefile.fast-build install-binaries", "make install DESTDIR=./instroot"),
		BranchName: gitInfo.branchName,
		RemoteURL:  gitInfo.remoteURL,
	}

	builder := builders.NewOpenshiftBuilder(cs, builderOpts)
	if err := builder.Build(); err != nil {
		return "", err
	}

	klog.Infof("Build completed, using pullspec %s", pullspec)
	return pullspec, nil
}

func buildLocallyAndDeploy() error {
	gi, err := getGitInfo(hardcodedRepoRoot)
	if err != nil {
		klog.Fatalln(err)
	}

	if err := setupRepo(gi); err != nil {
		klog.Fatalln(err)
	}

	opts := builders.Opts{
		RepoRoot:       hardcodedRepoRoot,
		FinalPullspec:  "quay.io/zzlotnik/machine-config-operator:latest",
		PushSecretPath: "/Users/zzlotnik/.docker-zzlotnik-testing/config.json",
		DockerfileName: dockerfileName,
	}

	builder := builders.NewDockerBuilder(opts)

	if err := builder.Build(); err != nil {
		return err
	}

	klog.Infof("Final image pullspec: %s", opts.FinalPullspec)

	files := []string{
		gi.dockerfilePath(),
		gi.makefilePath(),
	}

	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			klog.Fatalln(err)
		}
	}

	if err := teardownRepo(gi); err != nil {
		klog.Fatalln(err)
	}

	return rollout(framework.NewClientSet(""), opts.FinalPullspec)
}

func buildInClusterAndDeploy() error {
	cs := framework.NewClientSet("")

	pullspec, err := doOpenshiftBuild(cs)
	if err != nil {
		return err
	}

	return rollout(cs, pullspec)
}

func rollout(cs *framework.ClientSet, pullspec string) error {
	if err := setDeploymentReplicas(cs, "cluster-version-operator", "openshift-cluster-version", 0); err != nil {
		return err
	}

	if err := setDeploymentReplicas(cs, "machine-config-operator", ctrlcommon.MCONamespace, 0); err != nil {
		return err
	}

	if err := replaceMCOConfigmap(cs, pullspec); err != nil {
		return err
	}

	components := map[string][]string{
		"daemonset": {
			"machine-config-server",
			"machine-config-daemon",
		},
		"deployment": {
			"machine-config-operator",
			"machine-config-controller",
			"machine-os-builder",
		},
	}

	updated := false

	daemonsetsUpdated, err := updateDaemonsets(cs, components["daemonset"], pullspec)
	if err != nil {
		return err
	}

	if daemonsetsUpdated {
		updated = true
	}

	deploymentsUpdated, err := updateDeployments(cs, components["deployment"], pullspec)
	if err != nil {
		return err
	}

	if deploymentsUpdated {
		updated = true
	}

	if updated {
		klog.Warningf("This update will trigger a MachineConfig update.")
	}

	return setDeploymentReplicas(cs, "machine-config-operator", ctrlcommon.MCONamespace, 1)
}

func updateDeployments(cs *framework.ClientSet, names []string, pullspec string) (bool, error) {
	updated := false

	for _, name := range names {
		klog.Infof("Updating deployment/%s", name)
		wasUpdated, err := updateDeployment(cs, name, pullspec)
		if err != nil {
			return false, err
		}

		if wasUpdated {
			updated = true
		}
	}

	return updated, nil
}

func updateDaemonsets(cs *framework.ClientSet, names []string, pullspec string) (bool, error) {
	updated := false

	for _, name := range names {
		klog.Infof("Updating daemonset/%s", name)
		wasUpdated, err := updateDaemonset(cs, name, pullspec)
		if err != nil {
			return false, err
		}

		if wasUpdated {
			updated = true
		}
	}

	return updated, nil
}

func loadMCOImagesConfigMap(cs *framework.ClientSet) (*corev1.ConfigMap, map[string]string, error) {
	cmName := "machine-config-operator-images"
	imagesJSONKey := "images.json"
	imagesKey := "machineConfigOperator"

	cm, err := cs.CoreV1Interface.ConfigMaps(ctrlcommon.MCONamespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	_, ok := cm.Data[imagesJSONKey]
	if !ok {
		return nil, nil, fmt.Errorf("expected Configmap %s to have key %s, but was missing", cmName, imagesJSONKey)
	}

	images := map[string]string{}

	if err := json.Unmarshal([]byte(cm.Data[imagesJSONKey]), &images); err != nil {
		return nil, nil, err
	}

	if _, ok := images[imagesKey]; !ok {
		return nil, nil, fmt.Errorf("expected %s in Configmap %s to have key %s, but was missing", imagesJSONKey, cmName, imagesKey)
	}

	return cm, images, nil
}

func replaceMCOConfigmap(cs *framework.ClientSet, pullspec string) error {
	cmName := "machine-config-operator-images"
	imagesJSONKey := "images.json"
	imagesKey := "machineConfigOperator"

	cm, images, err := loadMCOImagesConfigMap(cs)
	if err != nil {
		return err
	}

	if images[imagesKey] == pullspec {
		return nil
	}

	images[imagesKey] = pullspec

	imagesBytes, err := json.Marshal(images)
	if err != nil {
		return err
	}

	cm.Data[imagesJSONKey] = string(imagesBytes)

	replacementCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   cm.Namespace,
			Name:        cm.Name,
			Annotations: cm.Annotations,
			Labels:      cm.Labels,
		},
		Data: cm.Data,
	}

	if err := cs.CoreV1Interface.ConfigMaps(ctrlcommon.MCONamespace).Delete(context.TODO(), cmName, metav1.DeleteOptions{}); err != nil {
		return err
	}

	if _, err = cs.CoreV1Interface.ConfigMaps(ctrlcommon.MCONamespace).Create(context.TODO(), replacementCM, metav1.CreateOptions{}); err != nil {
		return err
	}

	klog.Infof("Set %s in %s in ConfigMap %s to %s", imagesKey, imagesJSONKey, cmName, pullspec)

	return nil
}

func updateDeployment(cs *framework.ClientSet, name, pullspec string) (bool, error) {
	updated := false
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		deploy, err := cs.AppsV1Interface.Deployments(ctrlcommon.MCONamespace).Get(context.TODO(), name, metav1.GetOptions{})
		if name == "machine-os-builder" && apierrs.IsNotFound(err) {
			return nil
		}

		if err != nil {
			return err
		}

		if !containersNeedUpdated(name, pullspec, deploy.Spec.Template.Spec.Containers) {
			klog.Infof("Container pullspec did not change from %s, restarting deployment/%s to pull the latest image", pullspec, name)
			return restartWithOC("deployment", name)
		}

		_, err = cs.AppsV1Interface.Deployments(ctrlcommon.MCONamespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
		updated = true
		return err
	})

	return updated, err
}

func restartWithOC(kind, name string) error {
	resource := fmt.Sprintf("%s/%s", kind, name)
	cmd := exec.Command("oc", "rollout", "restart", "--namespace", ctrlcommon.MCONamespace, resource)
	out, err := cmd.CombinedOutput()
	if err == nil {
		klog.Infof("Restarted %s", resource)
		return err
	}

	_, err = os.Stderr.Write(out)
	return err
}

func updateDaemonset(cs *framework.ClientSet, name, pullspec string) (bool, error) {
	updated := false

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ds, err := cs.AppsV1Interface.DaemonSets(ctrlcommon.MCONamespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if !containersNeedUpdated(name, pullspec, ds.Spec.Template.Spec.Containers) {
			klog.Infof("Container pullspec did not change from %s, restarting daemonset/%s to pull the latest image", pullspec, name)
			return restartWithOC("daemonset", name)
		}

		ds.Spec.Template.Spec.Containers = updateContainers(name, pullspec, ds.Spec.Template.Spec.Containers)

		_, err = cs.AppsV1Interface.DaemonSets(ctrlcommon.MCONamespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		updated = true

		return err
	})

	return updated, err
}

func containersNeedUpdated(name, pullspec string, containers []corev1.Container) bool {
	for _, container := range containers {
		if container.Name == name {
			return container.Image != pullspec
		}
	}

	return false
}

func updateContainers(name, pullspec string, containers []corev1.Container) []corev1.Container {
	out := []corev1.Container{}

	for _, container := range containers {
		if container.Name == name {
			container.Image = pullspec
			container.ImagePullPolicy = corev1.PullAlways
		}

		out = append(out, container)
	}

	return out
}

func setDeploymentReplicas(cs *framework.ClientSet, deploymentName, namespace string, replicas int32) error {
	klog.Infof("Setting replicas for %s/%s to %d", namespace, deploymentName, replicas)
	scale, err := cs.AppsV1Interface.Deployments(namespace).GetScale(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	scale.Spec.Replicas = replicas

	_, err = cs.AppsV1Interface.Deployments(namespace).UpdateScale(context.TODO(), deploymentName, scale, metav1.UpdateOptions{})
	return err
}

func main() {
	if err := rollout(framework.NewClientSet(""), "quay.io/zzlotnik/machine-config-operator:latest"); err != nil {
		klog.Fatalln(err)
	}
}
