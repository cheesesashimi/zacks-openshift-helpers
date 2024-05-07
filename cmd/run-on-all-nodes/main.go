package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/cli"

	"golang.org/x/sync/errgroup"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/utils"
	versioncmd "github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/version"
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
)

var (
	version = "not-built-properly"
	commit  = "not-built-properly"
	date    = "not-built-properly"
)

type runOpts struct {
	command       string
	kubeconfig    string
	labelSelector string
	keepGoing     bool
	writeLogs     bool
}

func main() {
	opts := runOpts{}

	rootCmd := &cobra.Command{
		Use:   "run-on-all-nodes [flags] [command]",
		Short: "Automates running a command on all nodes in a given OpenShift cluster",
		Long:  "",
		RunE: func(_ *cobra.Command, args []string) error {
			if args[0] == "" {
				return fmt.Errorf("no command provided")
			}

			opts.command = args[0]

			return runOnAllNodes(opts)
		},
		Args: cobra.ExactArgs(1),
	}

	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	rootCmd.AddCommand(versioncmd.Command(version, commit, date))
	rootCmd.PersistentFlags().StringVar(&opts.labelSelector, "label-selector", "", "Label selector for nodes.")
	rootCmd.PersistentFlags().BoolVar(&opts.keepGoing, "keep-going", false, "Do not stop on first command error")
	rootCmd.PersistentFlags().BoolVar(&opts.writeLogs, "write-logs", false, "Write command logs to disk under $PWD/<nodename>.log")

	os.Exit(cli.Run(rootCmd))
}

func getNodeRoles(node *corev1.Node) []string {
	roles := []string{}

	for label := range node.Labels {
		if strings.Contains(label, "node-role.kubernetes.io") {
			roles = append(roles, label)
		}
	}

	return roles
}

func getNodeNames(nodes *corev1.NodeList) []string {
	names := []string{}

	for _, node := range nodes.Items {
		names = append(names, node.Name)
	}

	return names
}

func runCommand(outChan chan string, node *corev1.Node, opts runOpts) error {
	cmd := exec.Command("oc", "debug", fmt.Sprintf("node/%s", node.Name), "--", "chroot", "/host", "/bin/bash", "-c", opts.command)

	stdout := bytes.NewBuffer([]byte{})
	cmd.Stdout = stdout
	cmd.Env = utils.ToEnvVars(map[string]string{
		"KUBECONFIG": opts.kubeconfig,
	})

	err := cmd.Run()

	if !opts.keepGoing && err != nil {
		return fmt.Errorf("could not run command %s: %w", cmd, err)
	}

	out := &strings.Builder{}
	fmt.Fprintf(out, "[%s - %v]:\n", node.Name, getNodeRoles(node))
	fmt.Fprintf(out, "$ %s\n", opts.command)
	fmt.Fprintln(out, stdout.String())

	logFileName := fmt.Sprintf("%s.log", node.Name)
	if opts.writeLogs {
		fmt.Fprintf(out, "Writing log to %s\n", logFileName)
	}

	outChan <- out.String()

	if opts.writeLogs {
		return os.WriteFile(logFileName, stdout.Bytes(), 0o644)
	}

	return nil
}

func runCommandOnAllNodes(nodes *corev1.NodeList, opts runOpts) error {
	eg := new(errgroup.Group)

	outChan := make(chan string)

	// Spawn a separate logging Goroutine so that outputs are not interweaved.
	go func() {
		for msg := range outChan {
			fmt.Println(msg)
		}
	}()

	for _, node := range nodes.Items {
		node := node
		// For each node, spawn an oc command and run the provided command on the node.
		eg.Go(func() error {
			return runCommand(outChan, &node, opts)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	close(outChan)

	return nil
}

func runOnAllNodes(opts runOpts) error {
	if err := utils.CheckForBinaries([]string{"oc"}); err != nil {
		return err
	}

	cs := framework.NewClientSet("")

	kubeconfig, err := cs.GetKubeconfig()
	if err != nil {
		return err
	}

	opts.kubeconfig = kubeconfig

	listOpts := metav1.ListOptions{}

	if opts.labelSelector != "" {
		listOpts.LabelSelector = opts.labelSelector
		fmt.Println("Using label selector:", opts.labelSelector)
	}

	nodes, err := cs.CoreV1Interface.Nodes().List(context.TODO(), listOpts)
	if err != nil {
		return err
	}

	fmt.Println("Running on nodes:", getNodeNames(nodes))
	fmt.Println("")

	return runCommandOnAllNodes(nodes, opts)
}
