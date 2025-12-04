package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"github.com/spf13/cobra"

	imagev1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/component-base/cli"
	"k8s.io/klog"
)

type cfg struct {
	baseRelease          string
	rhel10Pullspec       string
	rhel10ExtPullspec    string
	finalReleasePullspec string
	srcAuthfile          string
	dstAuthfile          string
}

func (c *cfg) validate() error {
	if c.baseRelease == "" {
		return fmt.Errorf("must supply --base-release")
	}

	if c.rhel10Pullspec == "" {
		return fmt.Errorf("must supply --rhel10-pullspec")
	}

	if c.rhel10ExtPullspec == "" {
		return fmt.Errorf("must supply --rhel10-ext-pullspec")
	}

	if c.finalReleasePullspec == "" {
		return fmt.Errorf("must supply --final-release-pullspec")
	}

	if c.srcAuthfile == "" {
		return fmt.Errorf("must supply --src-authfile")
	}

	if c.dstAuthfile == "" {
		return fmt.Errorf("must supply --dst-authfile")
	}

	return nil
}

func initializeCobraCommand() *cobra.Command {
	cfg := cfg{}

	rootCmd := &cobra.Command{
		Use:   "dualstream-release-builder",
		Short: "Creates a dualstream (RHEL9 / RHEL10) OCP release image",
		Long:  "",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if err := cfg.validate(); err != nil {
				return err
			}

			bins := []string{"oc", "podman", "skopeo"}

			for _, bin := range bins {
				if _, err := exec.LookPath(bin); err != nil {
					return fmt.Errorf("could not find required binary '%s': %w", bin, err)
				}
			}

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return createDualstreamRelease(cfg)
		},
	}

	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	rootCmd.PersistentFlags().StringVar(&cfg.baseRelease, "base-release", "", "Base release image pullspec to use.")
	rootCmd.PersistentFlags().StringVar(&cfg.rhel10Pullspec, "rhel10-pullspec", "", "RHEL10 OS image pullspec to use.")
	rootCmd.PersistentFlags().StringVar(&cfg.rhel10ExtPullspec, "rhel10-ext-pullspec", "", "RHEL10 extensions image pullspec to use.")
	rootCmd.PersistentFlags().StringVar(&cfg.finalReleasePullspec, "final-release-pullspec", "", "Final release pullspec to use.")
	rootCmd.PersistentFlags().StringVar(&cfg.srcAuthfile, "src-authfile", "", "The registry authfile to use for pulling the release image.")
	rootCmd.PersistentFlags().StringVar(&cfg.dstAuthfile, "dst-authfile", "", "The registry authfile to use for pushing the final release image.")

	return rootCmd
}

func main() {
	os.Exit(cli.Run(initializeCobraCommand()))
}

func runCommandWithOutput(cmd []string) error {
	klog.Info("Running: ", strings.Join(cmd, " "))
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func runCommandAndCollectOutput(cmd []string) ([]byte, []byte, error) {
	klog.Info("Running: ", strings.Join(cmd, " "))
	c := exec.Command(cmd[0], cmd[1:]...)

	outBuf := bytes.NewBuffer([]byte{})
	errBuf := bytes.NewBuffer([]byte{})

	c.Stdout = outBuf
	c.Stderr = errBuf

	err := c.Run()
	return outBuf.Bytes(), errBuf.Bytes(), err
}

func addTagRefsToImagestreamMetadataFile(path string, tagRefs []imagev1.TagReference) error {
	// Read the imagestream file in.
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not open imagestream file: %w", err)
	}

	// Decode the imagestream JSON.
	is := &imagev1.ImageStream{}
	if err := json.Unmarshal(rawBytes, is); err != nil {
		return fmt.Errorf("could not decode imagestream file: %w", err)
	}

	osImageIndex := 0
	osExtImageIndex := 0

	// Find the OS image and extensions image indexes.
	for i, tag := range is.Spec.Tags {
		if strings.Contains(tag.Name, "coreos") {
			if strings.Contains(tag.Name, "extensions") {
				osExtImageIndex = i
			} else {
				osImageIndex = i

			}
		}
	}

	// Set the streamclass annotation.
	if _, ok := is.Spec.Tags[osImageIndex].Annotations["io.openshift.os.streamclass"]; !ok {
		is.Spec.Tags[osImageIndex].Annotations["io.openshift.os.streamclass"] = "rhel-9"
	}

	// Determine which index is greater so that we can group the new tag refs
	// adjacent to the existing ones. This allows for nicer output though does
	// not seem to be required.
	insertPoint := 1
	if osImageIndex > osExtImageIndex {
		insertPoint += osImageIndex
	} else {
		insertPoint += osExtImageIndex
	}

	// Insert the new tagrefs right after the preexisting ones.
	is.Spec.Tags = slices.Insert(is.Spec.Tags, insertPoint, tagRefs...)

	// Encode the imagestream.
	outBytes, err := json.Marshal(is)
	if err != nil {
		return fmt.Errorf("could not encode imagestream: %w", err)
	}

	return writeFileViaTmp(path, outBytes)
}

// Writes files first to a temp file and then overwrites the original file.
// This is because some of the permissions make just writing the file in-place
// more difficult.
func writeFileViaTmp(path string, contents []byte) error {
	tmp := path + ".tmp"

	// Write it to a temp file first.
	if err := os.WriteFile(tmp, contents, 0o755); err != nil {
		return fmt.Errorf("could not write new imagestream file: %w", err)
	}

	// Overwrite the preexisting file with the new one for a more transactional write.
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("could not move file into place: %w", err)
	}

	return nil
}

// Retrieves labels for the given image pullspec and creates an
// imagev1.TagReference using the given subset of the labels and their
// contents as its annotations.
func getTagRefForImageWithLabels(name, authfile, pullspec string, labelKeys []string) (*imagev1.TagReference, error) {
	sc := newAuthedSkopeoClient(authfile)

	ii, err := sc.Inspect(pullspec)
	if err != nil {
		return nil, fmt.Errorf("could not inspect image %s: %w", pullspec, err)
	}

	annotations := map[string]string{}
	for _, key := range labelKeys {
		val, ok := ii.Labels[key]
		annotations[key] = val
		// If the label is not found, insert the key with an empty value and warn about it.
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

func getTagRefForRHEL10Image(authfile, pullspec string) (*imagev1.TagReference, error) {
	return getTagRefForImageWithLabels("rhel-coreos-10", authfile, pullspec, []string{
		"io.openshift.build.commit.id",
		"io.openshift.build.commit.ref",
		"io.openshift.build.source-location",
		"io.openshift.build.version-display-names",
		"io.openshift.build.versions",
		"io.openshift.os.streamclass",
	})
}

func getTagRefForRHEL10ExtImage(authfile, pullspec string) (*imagev1.TagReference, error) {
	return getTagRefForImageWithLabels("rhel-coreos-10-extensions", authfile, pullspec, []string{
		"io.openshift.build.commit.id",
		"io.openshift.build.commit.ref",
		"io.openshift.build.source-location",
	})
}

// Creates a dualstream OCP release.
func createDualstreamRelease(cfg cfg) error {
	c := newAuthedPodmanClient(cfg.srcAuthfile)

	// Retrieve the release info using the oc command.
	ri, err := releasecontroller.GetReleaseInfo(cfg.baseRelease)
	if err != nil {
		return fmt.Errorf("could not get release info: %w", err)
	}

	// Look up the pullspec for the cluster-version-operator since the release
	// payload is the CVO image with some additional metadata.
	cvoTagRef := ri.GetTagRefForComponentName("cluster-version-operator")
	if cvoTagRef == nil {
		return fmt.Errorf("cluster-version-operator tag should not be nil")
	}

	klog.Info("Release Pullspec:", cfg.baseRelease)
	klog.Info("CVO Pullspec:", cvoTagRef.From.Name)

	// Create a tempdir to stage all of our changes.
	workdir, err := os.MkdirTemp("", "dualstream-release-builder")
	if err != nil {
		return fmt.Errorf("could not create tempdir: %w", err)
	}

	defer func() {
		// Ensure that our tempdir is removed.
		if err := os.RemoveAll(workdir); err != nil {
			klog.Warningf("Could not delete tempdir %s: %s", workdir, err)
		}
	}()

	// Copy all of the files under /release-manifests from the release payload image to our work directory.
	if err := c.CopyFilesFromImage(cfg.baseRelease, "/release-manifests", workdir); err != nil {
		return fmt.Errorf("could not copy files from image: %w", err)
	}

	// Write a basic Containerfile that creates a new release image from the CVO image and our modified files.
	containerfile := []string{
		fmt.Sprintf("FROM %s", cvoTagRef.From.Name),
		"COPY /release-manifests /release-manifests",
	}

	// Get the tag reference for the RHEL10 OS image
	rhel10TagRef, err := getTagRefForRHEL10Image(cfg.srcAuthfile, cfg.rhel10Pullspec)
	if err != nil {
		return fmt.Errorf("could not get RHEL10 tag ref: %w", err)
	}

	// Get the tag reference for the RHEL10 extensions image.
	rhel10ExtTagRef, err := getTagRefForRHEL10ExtImage(cfg.srcAuthfile, cfg.rhel10ExtPullspec)
	if err != nil {
		return fmt.Errorf("could not get RHEL10 extensions tag ref: %w", err)
	}

	// Add the tag references into our imagestream.
	if err := addTagRefsToImagestreamMetadataFile(filepath.Join(workdir, "release-manifests", "image-references"), []imagev1.TagReference{*rhel10TagRef, *rhel10ExtTagRef}); err != nil {
		return fmt.Errorf("could not add tag refs to image-references file: %w", err)
	}

	// Search and replace the release version with our own release version.
	if err := setReleaseVersion(workdir); err != nil {
		return fmt.Errorf("could not set release version: %w", err)
	}

	// Write the Containerfile to our workspace.
	if err := os.WriteFile(filepath.Join(workdir, "Containerfile"), []byte(strings.Join(containerfile, "\n")), 0o755); err != nil {
		return fmt.Errorf("could not write Containerfile: %w", err)
	}

	// Build the new release image.
	if err := c.Build(cfg.finalReleasePullspec, workdir, filepath.Join(workdir, "Containerfile")); err != nil {
		return fmt.Errorf("could not build dualstream release payload: %w", err)
	}

	c = newAuthedPodmanClient(cfg.dstAuthfile)
	if err := c.PushImage(cfg.finalReleasePullspec); err != nil {
		return fmt.Errorf("cannot push final release image: %w", err)
	}

	return nil
}

// Finds the release version from the release-metadata file, appends
// -dualstream onto it, and then replaces that value in all of the
// release-manifest files within the workdir.
func setReleaseVersion(workdir string) error {
	type releaseMetadata struct {
		Version string `json:"version"`
	}

	inBytes, err := os.ReadFile(filepath.Join(workdir, "release-manifests", "release-metadata"))
	if err != nil {
		return err
	}

	rm := &releaseMetadata{}
	if err := json.Unmarshal(inBytes, rm); err != nil {
		return err
	}

	if err != nil {
		return fmt.Errorf("could not get release metadata: %w", err)
	}

	newReleaseVersion := rm.Version + "-dualstream"

	return filepath.Walk(workdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			replaced, err := readAndReplaceTextInFile(path, rm.Version, newReleaseVersion)
			if err != nil {
				return fmt.Errorf("could not replace release version in %s: %w", path, err)
			}

			if replaced {
				klog.Infof("Replaced release version to %s in %s", newReleaseVersion, strings.ReplaceAll(path, workdir, ""))
			}
		}

		return nil
	})
}

// Opens a file, reads it into memory, performs a search and replace, and
// writes the modified contents back to the file on-disk.
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

	if err := writeFileViaTmp(filename, []byte(str)); err != nil {
		return false, err
	}

	return true, nil
}

// type cfg struct {
// 	baseRelease string
// 	rhel10Pullspec             string
// 	rhel10ExtPullspec          string
// 	finalReleasePullspec  string
// 	srcAuthfile                string
// 	dstAuthfile                string
// }
