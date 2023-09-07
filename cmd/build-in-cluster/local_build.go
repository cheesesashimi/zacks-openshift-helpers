package main

import (
	"github.com/cheesesashimi/zacks-openshift-helpers/cmd/build-in-cluster/builders"
	"k8s.io/klog/v2"
)

type localBuildOpts struct {
	*gitInfo
	builder        string
	pushSecretPath string
	taggedPullspec string
}

func doLocalBuild(opts localBuildOpts) error {
	builder := builders.NewDockerBuilder(builders.Opts{
		RepoRoot:       opts.repoRoot,
		FinalPullspec:  opts.taggedPullspec,
		PushSecretPath: "/Users/zzlotnik/docker-config.json",
		//PushSecretPath: opts.pushSecretPath,
		DockerfileName: dockerfileName,
	})

	pullspec, err := builder.Build()
	if err != nil {
		return err
	}

	klog.Infof("Final image pullspec: %s", pullspec)
	return nil
}
