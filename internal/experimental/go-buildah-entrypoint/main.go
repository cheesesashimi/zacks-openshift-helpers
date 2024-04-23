package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	pullAuthfile        = "/path/to/pull/authfile"
	pushAuthfile        = "/path/to/push/authfile"
	tag                 = "registry.host.com/org/repo:tag"
	mountOpts    string = "z,rw"
)

type buildahOpts struct {
	logLevel      string
	storageDriver string
	authfile      string
}

func (b buildahOpts) toArgs() []string {
	if b.logLevel == "" {
		b.logLevel = "INFO"
	}

	if b.storageDriver == "" {
		b.storageDriver = "vfs"
	}

	out := []string{
		"--log-level", b.logLevel,
		"--storage-driver", b.storageDriver,
	}

	if b.authfile != "" {
		out = append(out, []string{"--authfile", b.authfile}...)
	}

	return out
}

type buildAsset struct {
	sourcePath      string
	localPath       string
	buildMountpoint string
}

func (b *buildAsset) copyToLocal() error {
	fmt.Printf("Copying %s to %s\n", b.sourcePath, b.localPath)
	cmd := exec.Command("cp", "-r", "-v", fmt.Sprintf("%s/.", b.sourcePath), b.localPath)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (b *buildAsset) isSourcePathExist() (bool, error) {
	_, err := os.Stat(b.sourcePath)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func (b *buildAsset) toBuildahVolumeArgument() string {
	return fmt.Sprintf("--volume=%s:%s:%s", b.localPath, b.buildMountpoint, mountOpts)
}

func performBuild(opts buildahOpts, buildAssets []buildAsset) error {
	buildahBuildArgs := []string{
		"build",
		"--tag", tag,
		"--file=/path/to/Containerfile",
	}

	buildahBuildArgs = append(buildahBuildArgs, opts.toArgs()...)

	for _, buildAsset := range buildAssets {
		isSourcePathExist, err := buildAsset.isSourcePathExist()
		if err != nil {
			return err
		}

		if !isSourcePathExist {
			fmt.Println(buildAsset.sourcePath, "does not exist, skipping!")
			continue
		}

		tempDir, err := os.MkdirTemp("", filepath.Base(buildAsset.sourcePath))
		if err != nil {
			return err
		}

		defer os.RemoveAll(tempDir)
		buildAsset.localPath = tempDir
		buildahBuildArgs = append(buildahBuildArgs, buildAsset.toBuildahVolumeArgument())
		if err := buildAsset.copyToLocal(); err != nil {
			return err
		}
	}

	buildahBuildArgs = append(buildahBuildArgs, "/path/to/build-context")

	cmd := exec.Command("buildah", buildahBuildArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println(cmd)
	return nil
}

func performPush(opts buildahOpts) error { ////nolint:unparam // This should return nil for now since it is only an example.
	digestfile := "/path/to/digestfile"
	kubeCertDir := "/var/run/secrets/kubernetes.io/serviceaccount"

	buildahPushCmd := []string{"push", "--digestfile", digestfile, "--cert-dir", kubeCertDir}
	buildahPushCmd = append(buildahPushCmd, opts.toArgs()...)
	cmd := exec.Command("buildah", buildahPushCmd...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println(cmd)
	return nil
}

func main() {
	buildAssetPaths := []string{
		"/etc/pki/entitlement",
		"/etc/pki/rpm-gpg",
		"/etc/yum.repos.d",
	}

	buildAssets := []buildAsset{
		{
			sourcePath:      "/var/run/secrets/rhsm",
			buildMountpoint: "/run/secrets/rhsm",
		},
	}

	for _, buildAssetPath := range buildAssetPaths {
		buildAssets = append(buildAssets, buildAsset{
			sourcePath:      buildAssetPath,
			buildMountpoint: buildAssetPath,
		})
	}

	buildOpts := buildahOpts{
		authfile: pullAuthfile,
	}

	if err := performBuild(buildOpts, buildAssets); err != nil {
		log.Fatal(err)
	}

	pushOpts := buildahOpts{
		authfile: pushAuthfile,
	}

	if err := performPush(pushOpts); err != nil {
		log.Fatal(err)
	}
}
