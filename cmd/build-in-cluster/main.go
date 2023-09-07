package main

import (
	"os"

	"k8s.io/klog"
)

func main() {
	repoRoot := "/Users/zzlotnik/go/src/github.com/openshift/machine-config-operator"

	gi, err := getGitInfo(repoRoot)
	if err != nil {
		klog.Fatalln(err)
	}

	if err := setupRepo(gi); err != nil {
		klog.Fatalln(err)
	}

	buildOpts := localBuildOpts{
		gitInfo:        gi,
		taggedPullspec: "quay.io/zzlotnik/machine-config-operator:latest",
		pushSecretPath: "/Users/zzlotnik/.docker-zzlotnik-testing/config.json",
		builder:        "docker",
	}

	if err := doLocalBuild(buildOpts); err != nil {
		klog.Fatalln(err)
	}

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
}
