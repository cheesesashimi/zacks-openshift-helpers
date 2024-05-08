package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	aggerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

type teardownOpts struct {
	force bool
	inputOpts
}

func init() {
	teardownOpts := teardownOpts{}

	teardownCmd := &cobra.Command{
		Use:   "teardown",
		Short: "Brings up an OpenShift cluster for testing purposes",
		Long:  "",
		RunE: func(_ *cobra.Command, _ []string) error {
			return teardown(teardownOpts)
		},
	}

	teardownCmd.PersistentFlags().StringVar(&teardownOpts.workDir, "work-dir", defaultWorkDir, "The directory to use for running openshift-install.")
	teardownCmd.PersistentFlags().BoolVar(&teardownOpts.force, "force", false, "Runs openshift-install destroy cluster, if openshift-install is present.")
	teardownCmd.PersistentFlags().BoolVar(&teardownOpts.writeLogFile, "write-log-file", false, "Keeps track of cluster setups and teardown by writing to "+clusterLifecycleLogFile)

	rootCmd.AddCommand(teardownCmd)
}

func teardown(teardownOpts teardownOpts) error {
	if err := teardownOpts.validateForTeardown(); err != nil {
		return err
	}

	if teardownOpts.writeLogFile {
		ci, ciErr := readCurrentInstallFile(teardownOpts.inputOpts)
		if ciErr != nil && !errors.Is(ciErr, os.ErrNotExist) {
			if !teardownOpts.force {
				return ciErr
			}

			klog.Errorf("Ignoring encountered error while trying to read %s because --force was used: %s", teardownOpts.currentInstallPath(), ciErr)
		}
		defer func() {
			if ciErr != nil {
				return
			}

			if err := ci.appendTeardownToLogFile(teardownOpts.inputOpts); err != nil {
				klog.Fatalln(err)
			}
		}()
	}

	if teardownOpts.force {
		return forcedTeardown(teardownOpts)
	}

	return gracefulTeardown(teardownOpts)
}

func gracefulTeardown(opts teardownOpts) error {
	filesToCheckFor := []string{
		opts.installerPath(),
		opts.appendWorkDir("metadata.json"),
	}

	for _, file := range filesToCheckFor {
		exists, err := isFileExists(file)
		if err != nil {
			return err
		}

		if !exists {
			klog.Infof("%s not found. Nothing to do!", file)
			return nil
		}
	}

	if err := destroyCluster(opts); err != nil {
		return fmt.Errorf("unable to destroy cluster: %w", err)
	}

	if err := teardownWorkDir(opts); err != nil {
		return fmt.Errorf("unable to teardown workdir: %w", err)
	}

	klog.Infof("Cluster destroyed!")
	return nil
}

func forcedTeardown(opts teardownOpts) error {
	errs := []error{}

	if err := destroyCluster(opts); err != nil {
		err = fmt.Errorf("unable to destroy cluster: %w", err)
		klog.Errorf("Ignoring error while destroying cluster: %s", err)
		errs = append(errs, err)
	}

	if err := teardownWorkDir(opts); err != nil {
		err = fmt.Errorf("unable to teardown workdir: %w", err)
		klog.Errorf("Ignoring error while tearing down workdir %s: %s", opts.workDir, err)
		errs = append(errs, err)
	}

	return aggerrs.NewAggregate(errs)
}

func teardownWorkDir(opts teardownOpts) error {
	pathsToIgnore := sets.NewString([]string{
		opts.workDir,
		opts.vacationFilePath(),
		opts.releaseFilePath(),
		opts.logPath(),
	}...)

	// If the persistent release file exists, leave the installer behind so it
	// does not have to be re-fetched.
	releaseFileExists, err := isFileExists(opts.releaseFilePath())
	if err != nil {
		return err
	}

	if releaseFileExists {
		pathsToIgnore.Insert(opts.installerPath())
	}

	pathsToDelete := sets.NewString()

	// Identify the files to delete, first.
	err = filepath.Walk(opts.workDir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if pathsToIgnore.Has(path) {
			return nil
		}

		if f.IsDir() && path == opts.workDir {
			return nil
		}

		pathsToDelete.Insert(path)
		return nil
	})

	if err != nil {
		return fmt.Errorf("could not walk workdir: %w", err)
	}

	// Delete the files, taking care to preserve the ones we want to keep around.
	for _, path := range pathsToDelete.List() {
		if pathsToIgnore.Has(path) {
			continue
		}

		klog.Infof("Removing %s", path)

		err := ignoreFileNotExistsErr(os.RemoveAll(path))
		if err != nil {
			return err
		}
	}

	klog.Infof("Workdir %s is clean", opts.workDir)
	return nil
}
