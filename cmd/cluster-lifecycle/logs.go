package main

import (
	"errors"
	"os"

	"github.com/ghodss/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

type op string

const (
	setupOp    op = "setup"
	teardownOp op = "teardown"
)

type clusterLifecycleLogEntry struct {
	Arch        string          `json:"arch"`
	ClusterName string          `json:"clusterName"`
	Kind        string          `json:"kind"`
	Pullspec    string          `json:"pullspec"`
	Stream      string          `json:"stream"`
	Started     metav1.Time     `json:"started"`
	Finished    metav1.Time     `json:"finished"`
	Duration    metav1.Duration `json:"duration"`
	Op          op              `json:"op"`
}

func (c *clusterLifecycleLogEntry) appendTeardownToLogFile(opts inputOpts) error {
	ci := *c
	ci.Started = metav1.Now()
	ci.Op = teardownOp
	return writeLogEntry(opts, ci)
}

func (c *clusterLifecycleLogEntry) appendToLogFile(opts inputOpts) error {
	return writeLogEntry(opts, *c)
}

func (c *clusterLifecycleLogEntry) writeToCurrentInstallPath(opts inputOpts) error {
	outBytes, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(opts.currentInstallPath(), outBytes, 0o755)
}

type clusterLifecycleLogEntries struct {
	Entries []clusterLifecycleLogEntry `json:"entries"`
}

func newSetupLogEntry(pullspec string, opts inputOpts) clusterLifecycleLogEntry {
	return clusterLifecycleLogEntry{
		Arch:        opts.releaseArch,
		ClusterName: opts.clusterName(),
		Kind:        opts.releaseKind,
		Pullspec:    pullspec,
		Stream:      opts.releaseStream,
		Started:     metav1.Now(),
		Op:          setupOp,
	}
}

func readCurrentInstallFile(opts inputOpts) (clusterLifecycleLogEntry, error) {
	out := clusterLifecycleLogEntry{}
	inBytes, err := os.ReadFile(opts.currentInstallPath())
	if errors.Is(err, os.ErrNotExist) {
		klog.Warningf("%s does not exist, unable to write corresponding output entry", opts.currentInstallPath())
		return out, err
	}

	if err != nil {
		return out, err
	}

	if err := yaml.Unmarshal(inBytes, &out); err != nil {
		return out, err
	}

	return out, nil
}

func writeLogEntry(opts inputOpts, logEntry clusterLifecycleLogEntry) error {
	if !opts.writeLogFile {
		return nil
	}

	logFile := opts.logPath()

	logBytes, err := os.ReadFile(logFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	entries := clusterLifecycleLogEntries{}

	if err == nil {
		err = yaml.Unmarshal(logBytes, &entries)
		if err != nil {
			return err
		}
	}

	logEntry.Finished = metav1.Now()
	logEntry.Duration = metav1.Duration{
		Duration: logEntry.Finished.Sub(logEntry.Started.Time),
	}

	entries.Entries = append(entries.Entries, logEntry)

	outBytes, err := yaml.Marshal(entries)
	if err != nil {
		return err
	}

	return os.WriteFile(logFile, outBytes, 0o755)
}
