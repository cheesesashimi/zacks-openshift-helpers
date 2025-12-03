package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"

	imagev1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

func runCommandWithOutput(cmd []string) error {
	fmt.Println("Running:", strings.Join(cmd, " "))
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func runCommandAndCollectOutput(cmd []string) ([]byte, []byte, error) {
	fmt.Println("Running:", strings.Join(cmd, " "))
	c := exec.Command(cmd[0], cmd[1:]...)

	outBuf := bytes.NewBuffer([]byte{})
	errBuf := bytes.NewBuffer([]byte{})

	c.Stdout = outBuf
	c.Stderr = errBuf

	err := c.Run()
	return outBuf.Bytes(), errBuf.Bytes(), err
}

func addTagRefsToImagestreamMetadataFile(path string, tagRefs []imagev1.TagReference) error {
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not open imagestream file: %w", err)
	}

	is := &imagev1.ImageStream{}
	if err := json.Unmarshal(rawBytes, is); err != nil {
		return fmt.Errorf("could not decode imagestream file: %w", err)
	}

	osIndex := 0
	osExtIndex := 0
	for i, tag := range is.Spec.Tags {
		if strings.Contains(tag.Name, "coreos") {
			if strings.Contains(tag.Name, "extensions") {
				osExtIndex = i
			} else {
				osIndex = i
			}
		}
	}

	if val, ok := is.Spec.Tags[osIndex].Annotations["io.openshift.os.streamclass"]; !ok || val == "" {
		is.Spec.Tags[osIndex].Annotations["io.openshift.os.streamclass"] = "rhel-9"
	}

	is.Spec.Tags = slices.Insert(is.Spec.Tags, osExtIndex, tagRefs...)

	outBytes, err := json.Marshal(is)
	if err != nil {
		return fmt.Errorf("could not encode imagestream: %w", err)
	}

	if err := os.WriteFile(path+".tmp", outBytes, 0o755); err != nil {
		return fmt.Errorf("could not write new imagestream file: %w", err)
	}

	if err := os.Rename(path+".tmp", path); err != nil {
		return fmt.Errorf("could not move file into place: %w", err)
	}

	return nil
}

func getTagRefForImageWithLabels(name, pullspec string, labelKeys []string) (*imagev1.TagReference, error) {
	sc := newSkopeoClient()

	ii, err := sc.Inspect(pullspec)
	if err != nil {
		return nil, fmt.Errorf("could not inspect image %s: %w", pullspec, err)
	}

	annotations := map[string]string{}
	for _, key := range labelKeys {
		val, ok := ii.Labels[key]
		annotations[key] = val
		if !ok {
			klog.Warningf("Label %q not found on image %q", key, pullspec)
		}
	}

	return &imagev1.TagReference{
		Name: name,
		From: &corev1.ObjectReference{
			Kind: "DockerImage",
			Name: pullspec,
		},
		Annotations: annotations,
	}, nil
}

func getTagRefForRHEL10Image(pullspec string) (*imagev1.TagReference, error) {
	return getTagRefForImageWithLabels("rhel-coreos-10", pullspec, []string{
		"io.openshift.build.commit.id",
		"io.openshift.build.commit.ref",
		"io.openshift.build.source-location",
		"io.openshift.build.version-display-names",
		"io.openshift.build.versions",
		"io.openshift.os.streamclass",
	})
}

func getTagRefForRHEL10ExtImage(pullspec string) (*imagev1.TagReference, error) {
	return getTagRefForImageWithLabels("rhel-coreos-10-extensions", pullspec, []string{
		"io.openshift.build.commit.id",
		"io.openshift.build.commit.ref",
		"io.openshift.build.source-location",
	})
}

func createDualstreamRelease() error {
	c := podmanClient{}

	releasePayloadPullspec := "registry.build07.ci.openshift.org/ci-ln-6p5smzb/release:latest"
	rhel10OSPullspec := "registry.ci.openshift.org/rhcos-devel/node-staging@sha256:857318f7a12783c71bda0d4e7c8eab3dcadfe88fb7a8ab937abc5592eaac640d"
	rhel10OSExtPullspec := "registry.ci.openshift.org/rhcos-devel/node-staging@sha256:8959dae45e038084c115e23815fab775177ba97071ccee4f080bd4859060bee5"

	ri, err := releasecontroller.GetReleaseInfo(releasePayloadPullspec)
	if err != nil {
		return fmt.Errorf("could not get release info: %w", err)
	}

	cvoTagRef := ri.GetTagRefForComponentName("cluster-version-operator")
	if cvoTagRef == nil {
		return fmt.Errorf("cluster-version-operator tag should not be nil")
	}

	fmt.Println("Release Pullspec:", releasePayloadPullspec)
	fmt.Println("CVO Pullspec:", cvoTagRef.From.Name)

	workdir, err := os.MkdirTemp("", "dualstream-release-builder")
	if err != nil {
		return fmt.Errorf("could not create tempdir: %w", err)
	}

	defer func() {
		if err := os.RemoveAll(workdir); err != nil {
			klog.Warningf("Could not delete tempdir %s: %s", workdir, err)
		}
	}()

	if err := c.CopyFilesFromImage(releasePayloadPullspec, "/release-manifests", workdir); err != nil {
		return fmt.Errorf("could not copy files from image: %w", err)
	}

	containerfile := []string{
		fmt.Sprintf("FROM %s", cvoTagRef.From.Name),
		"COPY /release-manifests /release-manifests",
	}

	rhel10TagRef, err := getTagRefForRHEL10Image(rhel10OSPullspec)
	if err != nil {
		return fmt.Errorf("could not get RHEL10 tag ref: %w", err)
	}

	rhel10ExtTagRef, err := getTagRefForRHEL10ExtImage(rhel10OSExtPullspec)
	if err != nil {
		return fmt.Errorf("could not get RHEL10 extensions tag ref: %w", err)
	}

	rhel10Tags := []imagev1.TagReference{*rhel10TagRef, *rhel10ExtTagRef}

	if err := addTagRefsToImagestreamMetadataFile(filepath.Join(workdir, "release-manifests", "image-references"), rhel10Tags); err != nil {
		return fmt.Errorf("could not add tag refs to image-references file: %w", err)
	}

	if err := setReleaseVersion(workdir); err != nil {
		return fmt.Errorf("could not set release version: %w", err)
	}

	if err := os.WriteFile(filepath.Join(workdir, "Containerfile"), []byte(strings.Join(containerfile, "\n")), 0o755); err != nil {
		return fmt.Errorf("could not write Containerfile: %w", err)
	}

	if err := c.Build("localhost/dualstream-release-payload:latest", workdir, filepath.Join(workdir, "Containerfile")); err != nil {
		return fmt.Errorf("could not build dualstream release payload: %w", err)
	}

	return nil
}

type releaseMetadata struct {
	Version string `json:"version"`
}

func setReleaseVersion(workdir string) error {
	rm, err := getReleaseMetadata(workdir)
	if err != nil {
		return fmt.Errorf("could not get release metadata: %w", err)
	}

	releaseVersion := rm.Version + "-dualstream"

	return filepath.Walk(workdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			replaced, err := readAndReplaceTextInFile(path, rm.Version, releaseVersion)
			if err != nil {
				return fmt.Errorf("could not replace release version in %s: %w", path, err)
			}

			if replaced {
				klog.Infof("Replaced release version in %s", path)
			}
		}

		return nil
	})
}

func readAndReplaceTextInFile(filename, search, replace string) (bool, error) {
	inBytes, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}

	str := string(inBytes)
	if !strings.Contains(str, search) {
		return false, nil
	}

	str = strings.ReplaceAll(str, search, replace)

	tmp := filename + ".tmp"
	if err := os.WriteFile(tmp, []byte(str), 0o755); err != nil {
		return false, err
	}

	if err := os.Rename(tmp, filename); err != nil {
		return false, err
	}

	return true, nil
}

func getReleaseMetadata(workdir string) (*releaseMetadata, error) {
	inBytes, err := os.ReadFile(filepath.Join(workdir, "release-manifests", "release-metadata"))
	if err != nil {
		return nil, err
	}

	rm := &releaseMetadata{}
	if err := json.Unmarshal(inBytes, rm); err != nil {
		return nil, err
	}

	return rm, nil
}

func main() {
	if err := createDualstreamRelease(); err != nil {
		log.Fatalln(err)
	}
}
