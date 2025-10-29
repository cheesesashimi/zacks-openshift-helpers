package main

import (
	"fmt"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"github.com/containers/image/v5/docker/reference"
	imagev1 "github.com/openshift/api/image/v1"
	"k8s.io/klog"
)

type dualstreamRelease struct {
	stream9    *releasecontroller.ReleaseInfo
	stream10   *releasecontroller.ReleaseInfo
	streamName string
	config
}

func newDualstreamRelease(streamName string, cfg config, stream9Pullspec string, stream10 *releasecontroller.ReleaseInfo) (*dualstreamRelease, error) {
	stream9ReleaseInfo, err := getReleaseInfo(stream9Pullspec, cfg)
	if err != nil {
		return nil, err
	}

	return &dualstreamRelease{
		streamName: streamName,
		config:     cfg,
		stream9:    stream9ReleaseInfo,
		stream10:   stream10,
	}, nil
}

func (d *dualstreamRelease) imageExists() (bool, error) {
	_, remotePullspec, err := d.getLocalAndRemotePullspecs()
	if err != nil {
		return false, err
	}

	return imageExists(remotePullspec)
}

func (d *dualstreamRelease) getStream10OSImageTagRefs() []*imagev1.TagReference {
	osImageTags := getOSImageTags()

	out := []*imagev1.TagReference{}

	for tag, newName := range osImageTags {
		if tagRef := d.stream10.GetTagRefForComponentName(tag); tagRef != nil {
			tagRef.Name = newName
			out = append(out, tagRef)
		}
	}

	return out
}

func (d *dualstreamRelease) getLocalAndRemotePullspecs() (string, string, error) {
	releaseName := d.getNewReleaseName()

	localPullspec, err := replaceHostnameAndTagOnPullspec(d.destRepoPullspec, "localhost:5000", releaseName)
	if err != nil {
		return "", "", fmt.Errorf("could not get local pullspec: %w", err)
	}

	remotePullspec, err := replaceTagOnPullspec(d.destRepoPullspec, releaseName)
	if err != nil {
		return "", "", fmt.Errorf("could not get remote pullspec: %w", err)
	}

	return localPullspec, remotePullspec, nil
}

func (d *dualstreamRelease) maybeCreateNewRelease() error {
	exists, err := d.imageExists()
	if err != nil {
		return err
	}

	if exists && d.overwriteExisting {
		return d.createNewRelease()
	}

	if !exists {
		return d.createNewRelease()
	}

	_, remotePullspec, err := d.getLocalAndRemotePullspecs()
	if err != nil {
		return err
	}

	klog.Infof("Release %s was found, skipping...", remotePullspec)

	return nil
}

func (d *dualstreamRelease) createNewRelease() error {
	localRegistry := newLocalImageRegistry("registry", 5000)

	if err := localRegistry.start(); err != nil {
		return fmt.Errorf("could not start local image registry: %w", err)
	}

	defer func() {
		if err := localRegistry.stopAndRemove(); err != nil {
			klog.Errorf("could not stop local image registry: %s", err)
		}
	}()

	cvoTag := d.stream9.GetTagRefForComponentName("cluster-version-operator")
	if cvoTag == nil {
		return fmt.Errorf("could not find cluster-version-operator image")
	}

	includeFlags := []string{}
	newImages := []string{}

	for _, tag := range d.getStream10OSImageTagRefs() {
		includeFlags = append(includeFlags, []string{"--include", tag.Name}...)
		newImages = append(newImages, fmt.Sprintf("%s=%s", tag.Name, tag.From.Name))
	}

	releaseName := d.getNewReleaseName()

	localPullspec, remotePullspec, err := d.getLocalAndRemotePullspecs()
	if err != nil {
		return err
	}

	newReleaseCmd := []string{
		"oc",
		"adm",
		"release",
		"new",
		"--name",
		releaseName,
		"--from-release",
		d.stream9.ReleasePullspec,
		"--to-image-base",
		cvoTag.From.Name,
		"--reference-mode",
		"source",
		"--to-image",
		localPullspec,
		"--insecure=true",
	}

	if d.srcRepoAuthfilePath != "" {
		newReleaseCmd = append(newReleaseCmd, []string{"--registry-config", d.srcRepoAuthfilePath}...)
	}

	newReleaseCmd = append(newReleaseCmd, includeFlags...)
	newReleaseCmd = append(newReleaseCmd, newImages...)

	return runCommandsWithoutOutput([][]string{
		newReleaseCmd,
		{"skopeo", "copy", "--authfile", d.destRepoAuthfilePath, "--src-tls-verify=false", "docker://" + localPullspec, "docker://" + remotePullspec},
	})
}

func (d *dualstreamRelease) getNewReleaseName() string {
	out := strings.ReplaceAll(d.stream9.References.Name, d.ocpVersion, d.ocpVersion+"-10.1")
	if strings.Contains(out, d.stream9.Config.Architecture) {
		return out
	}

	return out + "-" + d.stream9.Config.Architecture
}

func replaceHostnameAndTagOnPullspec(pullspec, hostname, tag string) (string, error) {
	parsed, err := reference.ParseNamed(pullspec)
	if err != nil {
		return "", err
	}

	_, orgAndRepo := reference.SplitHostname(parsed)

	return replaceTagOnPullspec(hostname+"/"+orgAndRepo, tag)
}

func replaceTagOnPullspec(pullspec, tag string) (string, error) {
	parsed, err := reference.ParseNamed(pullspec)
	if err != nil {
		return "", err
	}

	tagged, err := reference.WithTag(parsed, tag)
	if err != nil {
		return "", err
	}

	return tagged.String(), nil
}
