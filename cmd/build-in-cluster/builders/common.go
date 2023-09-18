package builders

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/pkg/errors"
	"k8s.io/klog"
)

type builderType string

const (
	builderTypePodman    builderType = "podman"
	builderTypeDocker    builderType = "docker"
	builderTypeOpenshift builderType = "openshift"
)

const (
	localPullspec string = "localhost/machine-config-operator:latest"
)

type Opts struct {
	RepoRoot       string
	PullSecretPath string
	PushSecretPath string
	FinalPullspec  string
	DockerfileName string
}

func makeBinaries(repoRoot string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	cmd := exec.Command("make", "binaries")
	cmd.Env = append(cmd.Env, toEnvVars(map[string]string{
		"HOME":   u.HomeDir,
		"GOARCH": "amd64",
		"GOOS":   "linux",
	})...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	klog.Infof("Running %s", cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to build binaries: %w", err)
	}

	return nil
}

func toEnvVars(in map[string]string) []string {
	out := []string{}

	for k, v := range in {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}

	return out
}

func pushWithSkopeo(opts Opts, builder builderType) error {
	imgStorageMap := map[builderType]string{
		builderTypePodman: "containers-storage",
		builderTypeDocker: "docker-daemon",
	}

	imgStorage, ok := imgStorageMap[builder]
	if !ok {
		return fmt.Errorf("unknown builder type %s", imgStorage)
	}

	skopeoOpts := []string{
		"--dest-authfile",
		opts.PushSecretPath,
		fmt.Sprintf("%s:%s", imgStorage, localPullspec),
		fmt.Sprintf("docker://%s", opts.FinalPullspec),
	}

	if strings.Contains(opts.FinalPullspec, "image-registry-openshift-image-registry") {
		skopeoOpts = append([]string{"copy", "--dest-tls-verify=false"}, skopeoOpts...)
	} else {
		skopeoOpts = append([]string{"copy"}, skopeoOpts...)
	}

	cmd := exec.Command("skopeo", skopeoOpts...)
	klog.Infof("Running $ %s", cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.NewExecErrorNoOutput(cmd, err)
	}

	return nil
}
