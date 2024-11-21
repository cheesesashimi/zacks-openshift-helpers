package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/rollout"
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/utils"
	"github.com/distribution/reference"
	"github.com/openshift/machine-config-operator/test/framework"
	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	internalRegistryHostname string = "image-registry.openshift-image-registry.svc:5000"
)

func makeIdempotent(f func() error) func() error {
	hasRun := false
	var result error

	return func() error {
		if hasRun {
			return result
		}

		result = f()
		hasRun = true
		return result
	}
}

func knownPingError(b []byte) bool {
	byteStr := string(b)
	return strings.Contains(byteStr, "pinging container registry") && strings.Contains(byteStr, "Client sent an HTTP request to an HTTPS server")
}

func waitForRegistryToBeAvailable(secretPath, pullspec string) error {
	maxAttempts := 60

	finalOutput := []byte{}

	for i := 0; i <= maxAttempts; i++ {
		klog.Infof("Attempt #%d to inspect image with skopeo", i)

		cmd := exec.Command("skopeo", "inspect", "--tls-verify=false", "--authfile", secretPath, fmt.Sprintf("docker://%s", pullspec))
		output, err := cmd.CombinedOutput()
		if err == nil {
			klog.Infof("Registry became available after %d attempts", i)
			return nil
		}

		byteStr := string(output)
		if !strings.Contains(byteStr, "pinging container registry") && !strings.Contains(byteStr, "Client sent an HTTP request to an HTTPS server") {
			return err
		}

		klog.Infof("Attempt #%d failed: %s", i, byteStr)

		time.Sleep(time.Second)

		finalOutput = output
	}

	return fmt.Errorf("skopeo could not inspect image %q after %d attempts, output: %s", pullspec, maxAttempts, finalOutput)
}

func doTheThing(pullspec string) error {
	if err := utils.CheckForBinaries([]string{"oc", "podman", "skopeo"}); err != nil {
		return err
	}

	named, err := reference.ParseNamed(pullspec)
	if err != nil {
		return err
	}

	cs := framework.NewClientSet("")

	extHostname, err := rollout.ExposeClusterImageRegistry(cs)
	if err != nil {
		return err
	}

	namespace := strings.Split(reference.Path(named), "/")[0]

	secretPath, err := writeBuilderSecretToTempDir(cs, namespace, extHostname)
	if err != nil {
		return err
	}

	if reference.Domain(named) == internalRegistryHostname {
		pullspec = strings.ReplaceAll(pullspec, internalRegistryHostname, extHostname)
	}

	klog.Infof("Attempting to pull image %q", pullspec)

	if err := waitForRegistryToBeAvailable(secretPath, pullspec); err != nil {
		return err
	}

	cmd := exec.Command("podman", "pull", "--tls-verify=false", "--authfile", secretPath, pullspec)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	if err := rollout.UnexposeClusterImageRegistry(cs); err != nil {
		return err
	}

	if err := os.RemoveAll(filepath.Dir(secretPath)); err != nil {
		return err
	}

	return nil
}

func writeBuilderSecretToTempDir(cs *framework.ClientSet, namespace, hostname string) (string, error) {
	secrets, err := cs.Secrets(namespace).List(context.TODO(), metav1.ListOptions{})
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

	converted, _, err := canonicalizePullSecretBytes(foundDockerCfg.Data[corev1.DockerConfigKey], hostname)
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

// Converts a legacy Docker pull secret into a more modern representation.
// Essentially, it converts {"registry.hostname.com": {"username": "user"...}}
// into {"auths": {"registry.hostname.com": {"username": "user"...}}}. If it
// encounters a pull secret already in this configuration, it will return the
// input secret as-is. Returns either the supplied data or the newly-configured
// representation of said data, a boolean to indicate whether it was converted,
// and any errors resulting from the conversion process. Additionally, this
// function will add an additional entry for the external cluster image
// registry hostname.
func canonicalizePullSecretBytes(secretBytes []byte, extHostname string) ([]byte, bool, error) {
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

	oldStyleDecoded[extHostname] = oldStyleDecoded[internalRegistryHostname]

	out, err := json.Marshal(&newStyleAuth{
		Auths: oldStyleDecoded,
	})

	return out, err == nil, err
}

func main() {
	// TODO: Adopt flags

	if len(os.Args) != 2 {
		panic("wrong number of cli args")
	}

	pullspec := os.Args[1]
	if err := doTheThing(pullspec); err != nil {
		panic(err)
	}
}
