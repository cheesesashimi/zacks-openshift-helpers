package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"github.com/coreos/stream-metadata-go/stream"
	"github.com/openshift/machine-config-operator/test/framework"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type imageInfo struct {
	Created      *metav1.Time
	Architecture string
	Os           string
	Labels       map[string]string
	Name         string
	Digest       string
}

func (i *imageInfo) digestedPullspec() string {
	return fmt.Sprintf("%s@%s", i.Name, i.Digest)
}

func (i *imageInfo) getCommitID() string {
	return i.Labels["io.openshift.build.commit.id"]
}

type imagePair struct {
	os  *imageInfo
	ext *imageInfo
}

func (i imagePair) pullspecs() map[string]string {
	return map[string]string{
		"rhel-coreos":            i.os.digestedPullspec(),
		"rhel-coreos-extensions": i.ext.digestedPullspec(),
	}
}

// func (i imagePair) getReleaseArtifacts() (*stream.Stream, error) {
// 	expectedMatch := map[string]string{
// 		"containers.bootc":  "1",
// 		"com.coreos.osname": "rhcos",
// 		"ostree.bootable":   "true",
// 	}
//
// 	versionKey := "org.opencontainers.image.version"
//
// 	expectedPresent := []string{
// 		versionKey,
// 		"ostree.linux",
// 	}
//
// 	for k, v := range expectedMatch {
// 		val, ok := i.os.Labels[k]
// 		if !ok || val != v {
// 			return nil, fmt.Errorf("expected image to have %q=%q", k, v)
// 		}
// 	}
//
// 	for _, item := range expectedPresent {
// 		if val, ok := i.os.Labels[item]; !ok || val == "" {
// 			return nil, fmt.Errorf("expected label key %q to be present", item)
// 		}
// 	}
//
// 	return getCoreOSReleaseStreamForBuild(i.os.Labels[versionKey])
// }

func getCoreOSReleaseStreamForBuild(buildID string) (*stream.Stream, error) {
	opts := []string{
		"podman",
		"run",
		"--interactive",
		"--tty",
		"--rm",
		"quay.io/coreos-assembler/coreos-assembler:latest",
		"shell",
		"plume",
		"cosa2stream",
		"--distro",
		"rhcos",
		"--no-signatures",
		"--name",
		"rhel-10.1",
		"--url",
		"https://rhcos.mirror.openshift.com/art/storage/prod/streams",
		"x86_64=" + buildID,
		"aarch64=" + buildID,
		"s390x=" + buildID,
		"ppc64le=" + buildID,
	}

	stdoutBuf, _, err := runCommandWithOutput(opts)
	if err != nil {
		return nil, err
	}

	s := &stream.Stream{}

	if err := json.Unmarshal(stdoutBuf.Bytes(), s); err != nil {
		return nil, err
	}

	return s, nil
}

func getLatestRHEL10ImagesFromRhcosDevel(ctx context.Context, cs *framework.ClientSet, cfg config) (map[string]imagePair, error) {
	is, err := cs.ImageV1Interface.ImageStreams("rhcos-devel").Get(ctx, "node-staging", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	extImages := []*imageInfo{}
	osImages := []*imageInfo{}

	for _, tag := range is.Status.Tags {
		for _, item := range tag.Items {
			pullspec := fmt.Sprintf("%s@%s", is.Status.PublicDockerImageRepository, item.Image)

			info, err := getImageInfo(cfg, pullspec)
			if err != nil {
				return nil, err
			}

			if strings.Contains(tag.Tag, "extensions") {
				extImages = append(extImages, info)
			} else {
				osImages = append(osImages, info)
			}
		}
	}

	pairsByArch := map[string]imagePair{}

	for _, osImage := range osImages {
		for _, extImage := range extImages {
			if osImage.getCommitID() == extImage.getCommitID() && osImage.Architecture == extImage.Architecture {
				curPair := imagePair{
					os:  osImage,
					ext: extImage,
				}

				pair, ok := pairsByArch[osImage.Architecture]
				if !ok {
					pairsByArch[osImage.Architecture] = curPair
					continue
				}

				if pair.os.Created.Before(curPair.os.Created) {
					fmt.Println(pair.os.Created, "is before", curPair.os.Created)
					pairsByArch[osImage.Architecture] = curPair
				} else {
					fmt.Println(pair.os.Created, "is after", curPair.os.Created)
				}
			}
		}
	}

	return pairsByArch, nil
}

func getImageInfo(cfg config, pullspec string) (*imageInfo, error) {
	stdoutBuf, _, err := runCommandWithOutput([]string{"skopeo", "inspect", "--no-tags", "docker://" + pullspec})
	if err != nil {
		return nil, err
	}

	i := &imageInfo{}
	if err := json.Unmarshal(stdoutBuf.Bytes(), i); err != nil {
		return nil, err
	}

	return i, nil
}

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

func getConfig() config {
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

	return cfg
}

func main() {
	cs := framework.NewClientSet("")

	cfg := getConfig()

	if err := getRhcosDevelNodeStagingInfo(context.TODO(), cs, cfg); err != nil {
		panic(err)
	}
}

func oldmain() {
	if err := doStuff(getConfig()); err != nil {
		panic(err)
	}
}
