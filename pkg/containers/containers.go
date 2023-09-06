package containers

import (
	"context"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
)

func ResolveToDigestedPullspec(pullspec, pullSecretPath string) (string, error) {
	sysCtx := &types.SystemContext{
		AuthFilePath: pullSecretPath,
	}

	tagged, err := docker.ParseReference("//" + pullspec)
	if err != nil {
		return "", err
	}

	digest, err := docker.GetDigest(context.TODO(), sysCtx, tagged)
	if err != nil {
		return "", err
	}

	canonical, err := reference.WithDigest(tagged.DockerReference(), digest)
	if err != nil {
		return "", err
	}

	return canonical.String(), nil
}
