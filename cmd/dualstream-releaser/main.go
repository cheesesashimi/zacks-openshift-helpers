package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"github.com/openshift/machine-config-operator/test/framework"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getRhcosDevelReleaseInfo(ctx context.Context, cs *framework.ClientSet, cfg config) (map[string]*releasecontroller.ReleaseInfo, error) {
	is, err := cs.ImageV1Interface.ImageStreams("rhcos-devel").Get(ctx, "ocp-4.21-10.1", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	arches := map[string]struct{}{
		"amd64": {},
		"arm64": {},
	}

	out := map[string]*releasecontroller.ReleaseInfo{}

	for _, tag := range is.Status.Tags {
		pullspec := fmt.Sprintf("%s:%s", is.Status.PublicDockerImageRepository, tag.Tag)
		ri, err := getReleaseInfo(pullspec, cfg)
		if err != nil {
			return nil, fmt.Errorf("could not get release info for pullspec %s: %w", pullspec, err)
		}

		if _, ok := arches[ri.Config.Architecture]; ok {
			out[ri.Config.Architecture] = ri
		}
	}

	return out, nil
}

func getOSImageTags() map[string]string {
	return map[string]string{
		"rhel-coreos":              "rhel-coreos-10",
		"rhel-coreos-extensions":   "rhel-coreos-10-extensions",
		"stream-coreos":            "stream-coreos-10",
		"stream-coreos-extensions": "stream-coreos-10-extensions",
	}
}

type config struct {
	ocpVersion           string
	destRepoPullspec     string
	destRepoAuthfilePath string
	srcRepoAuthfilePath  string
	overwriteExisting    bool
}

func doStuff(cfg config) error {
	cs := framework.NewClientSet("")

	ri, err := getRhcosDevelReleaseInfo(context.TODO(), cs, cfg)
	if err != nil {
		return err
	}

	for arch, stream10ReleaseInfo := range ri {
		rc, err := releasecontroller.GetReleaseController("ocp", arch)
		if err != nil {
			return err
		}

		streams, err := rc.GetAllReleaseStreams()
		if err != nil {
			return err
		}

		for _, stream := range streams {
			var releases []releasecontroller.Release

			if strings.Contains(stream, cfg.ocpVersion) {
				if strings.Contains(stream, cfg.ocpVersion) {
					latest, err := rc.GetLatestReleaseForStream(stream)
					if err != nil {
						return err
					}

					releases = []releasecontroller.Release{*latest}
				}
			}

			if strings.Contains(stream, "dev-preview") {
				streamReleases, err := rc.GetAllReleasesForStream(stream)
				if err != nil {
					return err
				}

				for _, release := range streamReleases.Tags {
					releases = append(releases, release)
				}
			}

			for _, release := range releases {
				dsr, err := newDualstreamRelease(stream, cfg, release.Pullspec, stream10ReleaseInfo)
				if err != nil {
					return err
				}

				if err := dsr.maybeCreateNewRelease(); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func main() {
	cfg := config{
		ocpVersion:           "4.21.0",
		destRepoPullspec:     "quay.io/zzlotnik/dualstream:latest",
		destRepoAuthfilePath: "/home/zzlotnik/.creds/zzlotnik-quay-push-creds.json",
		srcRepoAuthfilePath:  "/home/zzlotnik/.docker/config.json",
	}

	// RPi5 config
	if runtime.GOARCH == "arm64" && runtime.GOOS == "linux" {
		cfg.destRepoAuthfilePath = "/home/zack/.creds/quay-push-creds.json"
		cfg.srcRepoAuthfilePath = "/home/zack/.creds/openshift-dockerconfig.json"
	}

	if err := doStuff(cfg); err != nil {
		panic(err)
	}
}
