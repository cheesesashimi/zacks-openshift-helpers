package rollout

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcfgv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/openshift/machine-config-operator/test/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	aggerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

func WaitForMachineConfigPoolsToComplete(cs *framework.ClientSet, poolNames []string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, poolName := range poolNames {
		_, err := cs.MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}

	return waitForMachineConfigPoolsToCompleteWithContext(ctx, cs, poolNames)
}

func WaitForAllMachineConfigPoolsToComplete(cs *framework.ClientSet, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pools, err := cs.MachineConfigPools().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	poolNames := []string{}
	for _, pool := range pools.Items {
		poolNames = append(poolNames, pool.Name)
	}

	klog.Infof("Watching MachineConfigPool(s): %v", poolNames)

	start := time.Now()

	err = waitForMachineConfigPoolsToCompleteWithContext(ctx, cs, poolNames)
	if err == nil {
		klog.Infof("All pools updated in %s", time.Since(start))
		return nil
	}

	return err
}

func WaitForMachineConfigPoolToComplete(cs *framework.ClientSet, poolName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return waitForMachineConfigPoolToCompleteWithContext(ctx, cs, poolName)
}

func waitForMachineConfigPoolsToCompleteWithContext(ctx context.Context, cs *framework.ClientSet, poolNames []string) error {
	funcs := []func() error{}

	for _, poolName := range poolNames {
		poolName := poolName
		funcs = append(funcs, func() error {
			return waitForMachineConfigPoolToCompleteWithContext(ctx, cs, poolName)
		})
	}

	return aggerrs.AggregateGoroutines(funcs...)
}

func waitForMachineConfigPoolToCompleteWithContext(ctx context.Context, cs *framework.ClientSet, poolName string) error {
	doneNodes := sets.New[string]()
	nodesForPool := sets.New[string]()

	nodes, err := cs.CoreV1Interface.Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("node-role.kubernetes.io/%s", poolName),
	})

	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		nodesForPool.Insert(node.Name)
	}

	mcp, err := cs.MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	start := time.Now()

	klog.Infof("Current nodes for pool %q are: %v", poolName, sets.List(nodesForPool))
	klog.Infof("Waiting for nodes in pool to reach %s", getMachineConfigPoolState(mcp))

	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		mcp, err := cs.MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		nodes, err := cs.CoreV1Interface.Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("node-role.kubernetes.io/%s", poolName),
		})

		if err != nil {
			return false, err
		}

		for _, node := range nodes.Items {
			node := node

			if !nodesForPool.Has(node.Name) {
				klog.Infof("Pool %s has gained a new node %s", poolName, node.Name)
				nodesForPool.Insert(node.Name)
			}

			if doneNodes.Has(node.Name) {
				continue
			}

			ns := ctrlcommon.NewLayeredNodeState(&node)
			if ns.IsDoneAt(mcp) {
				doneNodes.Insert(node.Name)
				diff := sets.List(nodesForPool.Difference(doneNodes))
				klog.Infof("Node %s in pool %s has completed its update after %s. %d node(s) remaining: %v", node.Name, poolName, time.Since(start), len(diff), diff)
			}
		}

		isDone := doneNodes.Equal(nodesForPool)
		if isDone {
			klog.Infof("%d nodes in pool %s have completed their update after %s", nodesForPool.Len(), poolName, time.Since(start))
		}

		return isDone, nil
	})
}

func getMachineConfigPoolState(mcp *mcfgv1.MachineConfigPool) string {
	out := &strings.Builder{}

	fmt.Fprintf(out, "MachineConfig %s", mcp.Spec.Configuration.Name)

	lps := ctrlcommon.NewLayeredPoolState(mcp)
	if !lps.HasOSImage() {
		return out.String()
	}

	fmt.Fprintf(out, " / Image: %s", lps.GetOSImage())
	return out.String()
}
