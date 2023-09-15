package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/cmd/build-in-cluster/builders"
	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/errors"
	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/rollout"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/openshift/machine-config-operator/test/framework"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/klog"
)

const (
	hardcodedRepoRoot string = "/Users/zzlotnik/go/src/github.com/openshift/machine-config-operator"
)

func setupForPushIntoCluster() error {
	cs := framework.NewClientSet("")

	if err := setDeploymentReplicas(cs, "cluster-version-operator", "openshift-cluster-version", 0); err != nil {
		return err
	}

	cmd := exec.Command("oc", "get", "-n", "openshift-image-registry", "route/image-registry")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(out), `Error from server (NotFound): routes.route.openshift.io "image-registry" not found`) {
			return errors.NewExecError(cmd, out, err)
		}

		cmd = exec.Command("oc", "expose", "-n", "openshift-image-registry", "svc/image-registry")
		out, err = cmd.CombinedOutput()
		if err != nil {
			return errors.NewExecError(cmd, out, err)
		}
	}

	registryPatchSpec := `{"spec": {"tls": {"insecureEdgeTerminationPolicy": "Redirect", "termination": "reencrypt"}}}`
	cmd = exec.Command("oc", "patch", "-n", "openshift-image-registry", "route/image-registry", "-p", registryPatchSpec)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return errors.NewExecError(cmd, out, err)
	}

	cmd = exec.Command("oc", "get", "openshift-image-registry", "-o=jsonpath={.spec.host}", "route/image-registry")
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	stdoutBuf := bytes.NewBuffer([]byte{})
	if _, err := io.Copy(stdoutBuf, stdoutPipe); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	registryHostName := stdoutBuf.String()

	klog.Infof("Exposed %s for container registry", registryHostName)
	return nil
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
		return err
	}

	if err := setupRepo(gi); err != nil {
		return err
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
			return err
		}
	}

	if err := teardownRepo(gi); err != nil {
		return err
	}

	return rollout.ReplaceMCOImage(framework.NewClientSet(""), opts.FinalPullspec)
}

func buildInClusterAndDeploy() error {
	cs := framework.NewClientSet("")

	pullspec, err := doOpenshiftBuild(cs)
	if err != nil {
		return err
	}

	return rollout.ReplaceMCOImage(cs, pullspec)
}

func writeBuilderSecretToTempDir(cs *framework.ClientSet) (string, error) {
	secrets, err := cs.Secrets(ctrlcommon.MCONamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	var foundDockerCfg *corev1.Secret
	names := []string{}
	for _, secret := range secrets.Items {
		secret := secret
		names = append(names, secret.Name)
		if strings.HasPrefix(secret.Name, "builder-dockercfg") {
			foundDockerCfg = &secret
			break
		}
	}

	if foundDockerCfg == nil {
		return "", fmt.Errorf("did not find a matching secret, foundDockerCfg: %v", names)
	}

	converted, _, err := canonicalizePullSecretBytes(foundDockerCfg.Data[corev1.DockerConfigKey])
	if err != nil {
		return "", err
	}

	secretPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(secretPath, converted, 0o755); err != nil {
		return "", err
	}

	klog.Infof("Secret %q has been written to %s", foundDockerCfg.Name, secretPath)

	return secretPath, nil
}

func buildLocallyAndPushIntoCluster(cs *framework.ClientSet) error {
	klog.Infof("Setting up")
	gi, err := getGitInfo(hardcodedRepoRoot)
	if err != nil {
		return err
	}

	if err := setupRepo(gi); err != nil {
		return err
	}

	if err := setupForPushIntoClusterAPI(cs); err != nil {
		klog.Fatalln(err)
	}
	klog.Infof("Cluster is set up for direct pushes")

	secretPath, err := writeBuilderSecretToTempDir(cs)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Dir(secretPath))

	clusterImgRegistryHostname := "image-registry-openshift-image-registry.apps.zzlotnik-ocp-amd64.devcluster.openshift.com"

	opts := builders.Opts{
		RepoRoot:       hardcodedRepoRoot,
		FinalPullspec:  fmt.Sprintf("%s/%s/machine-config-operator:latest", clusterImgRegistryHostname, ctrlcommon.MCONamespace),
		PushSecretPath: secretPath,
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
			return err
		}
	}

	if err := teardownRepo(gi); err != nil {
		return err
	}

	return nil
}

// Converts a legacy Docker pull secret into a more modern representation.
// Essentially, it converts {"registry.hostname.com": {"username": "user"...}}
// into {"auths": {"registry.hostname.com": {"username": "user"...}}}. If it
// encounters a pull secret already in this configuration, it will return the
// input secret as-is. Returns either the supplied data or the newly-configured
// representation of said data, a boolean to indicate whether it was converted,
// and any errors resulting from the conversion process.
func canonicalizePullSecretBytes(secretBytes []byte) ([]byte, bool, error) {
	type newStyleAuth struct {
		Auths map[string]interface{} `json:"auths,omitempty"`
	}

	// Try marshaling the new-style secret first:
	newStyleDecoded := &newStyleAuth{}
	if err := json.Unmarshal(secretBytes, newStyleDecoded); err != nil {
		return nil, false, fmt.Errorf("could not decode new-style pull secret: %w", err)
	}

	// We have an new-style secret, so we can just return here.
	if len(newStyleDecoded.Auths) != 0 {
		return secretBytes, false, nil
	}

	// We need to convert the legacy-style secret to the new-style.
	oldStyleDecoded := map[string]interface{}{}
	if err := json.Unmarshal(secretBytes, &oldStyleDecoded); err != nil {
		return nil, false, fmt.Errorf("could not decode legacy-style pull secret: %w", err)
	}

	extHostname := "image-registry-openshift-image-registry.apps.zzlotnik-ocp-amd64.devcluster.openshift.com"
	oldStyleDecoded[extHostname] = oldStyleDecoded["image-registry.openshift-image-registry.svc:5000"]

	out, err := json.Marshal(&newStyleAuth{
		Auths: oldStyleDecoded,
	})

	return out, err == nil, err
}

func main() {
	if err := buildLocallyAndPushIntoCluster(framework.NewClientSet("")); err != nil {
		klog.Fatalln(err)
	}
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
