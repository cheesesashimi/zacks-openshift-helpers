package builders

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/errors"
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/repo"
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/utils"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

type BuilderType string

const (
	BuilderTypePodman    BuilderType = "podman"
	BuilderTypeDocker    BuilderType = "docker"
	BuilderTypeOpenshift BuilderType = "openshift"
	BuilderTypeUnknown   BuilderType = "unknown-builder-type"
)

const (
	localPullspec string = "localhost/machine-config-operator:latest"
)

type Builder interface {
	Build() error
	Push() error
}

type Opts struct {
	RepoRoot       string
	PullSecretPath string
	PushSecretPath string
	FinalPullspec  string
	DockerfileName string
	BuildMode      repo.BuildMode
}

func (o *Opts) isDirectClusterPush() bool {
	return strings.Contains(o.FinalPullspec, "image-registry-openshift-image-registry")
}

func NewLocalBuilder(opts Opts, bt BuilderType) Builder {
	if bt == BuilderTypePodman {
		return newPodmanBuilder(opts)
	}

	return newDockerBuilder(opts)
}

func GetBuilderTypes() sets.Set[BuilderType] {
	return GetLocalBuilderTypes().Insert(BuilderTypeOpenshift)
}

func GetLocalBuilderTypes() sets.Set[BuilderType] {
	return sets.New[BuilderType](BuilderTypePodman, BuilderTypeDocker)
}

func GetDefaultBuilderTypeForPlatform() BuilderType {
	if _, err := exec.LookPath("podman"); err == nil {
		return BuilderTypePodman
	}

	if _, err := exec.LookPath("docker"); err == nil {
		return BuilderTypeDocker
	}

	return BuilderTypeUnknown
}

func maybeMakeBinaries(opts Opts) error {
	if opts.BuildMode == repo.BuildModeFast {
		klog.Infof("Build mode %q selected, pre-building binaries using native tooling", repo.BuildModeFast)
		return makeBinaries(opts.RepoRoot)
	}

	klog.Infof("Build mode %q selected, using container builder to build binaries", repo.BuildModeNormal)
	return nil
}

func makeBinaries(repoRoot string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	if err := utils.CheckForBinaries([]string{"go", "make"}); err != nil {
		return err
	}

	cmd := exec.Command("make", "binaries")
	cmd.Env = utils.ToEnvVars(map[string]string{
		"HOME":   u.HomeDir,
		"GOARCH": "amd64",
		"GOOS":   "linux",
	})
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

func pushWithSkopeo(opts Opts, builder BuilderType) error {
	imgStorageMap := map[BuilderType]string{
		BuilderTypePodman: "containers-storage",
		BuilderTypeDocker: "docker-daemon",
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

	if opts.isDirectClusterPush() {
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

func validateLocalBuilderMode(opts Opts) error {
	validLocalModes := []repo.BuildMode{repo.BuildModeNormal, repo.BuildModeFast}

	for _, mode := range validLocalModes {
		if opts.BuildMode == mode {
			return nil
		}
	}

	return fmt.Errorf("invalid build mode %s, valid modes: %v", opts.BuildMode, validLocalModes)
}
