package containers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
)

func ResolveToDigestedPullspec(pullspec, pullSecretPath string) (string, error) {
	sysCtx := &types.SystemContext{
		AuthFilePath: pullSecretPath,
	}

	if strings.Contains(pullspec, "image-registry-openshift-image-registry") {
		sysCtx.OCIInsecureSkipTLSVerify = true
		sysCtx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(true)
	}

	tagged, err := docker.ParseReference("//" + pullspec)
	if err != nil {
		return "", err
	}

	digest, err := docker.GetDigest(context.TODO(), sysCtx, tagged)
	if err != nil {
		return "", err
	}

	canonical, err := reference.WithDigest(reference.TrimNamed(tagged.DockerReference()), digest)
	if err != nil {
		return "", err
	}

	return canonical.String(), nil
}

func AddLatestTagIfMissing(pullspec string) (string, error) {
	parsed, err := docker.ParseReference("//" + pullspec)
	if err != nil {
		return "", err
	}

	return parsed.DockerReference().String(), nil
}

func GetImageLabelsWithSkopeo(pullspec string) (map[string]string, error) {
	type info struct {
		Labels map[string]string
	}

	buf := bytes.NewBuffer([]byte{})

	cmd := exec.Command("skopeo", "inspect", "--no-tags", "docker://"+pullspec)
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	i := &info{}

	if err := json.Unmarshal(buf.Bytes(), i); err != nil {
		return nil, err
	}

	return i.Labels, nil
}
