package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/utils"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	clientmachineconfigv1alpha1 "github.com/openshift/client-go/machineconfiguration/clientset/versioned/typed/machineconfiguration/v1alpha1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/openshift/machine-config-operator/test/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

type moscOpts struct {
	poolName              string
	containerfileContents string
	pullSecretName        string
	pushSecretName        string
	finalImagePullspec    string
}

func newMachineOSConfig(opts moscOpts) *mcfgv1alpha1.MachineOSConfig {
	return &mcfgv1alpha1.MachineOSConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: opts.poolName,
			Labels: map[string]string{
				createdByOnClusterBuildsHelper: "",
			},
		},
		Spec: mcfgv1alpha1.MachineOSConfigSpec{
			MachineConfigPool: mcfgv1alpha1.MachineConfigPoolReference{
				Name: opts.poolName,
			},
			BuildInputs: mcfgv1alpha1.BuildInputs{
				BaseImagePullSecret: mcfgv1alpha1.ImageSecretObjectReference{
					Name: globalPullSecretCloneName,
				},
				RenderedImagePushSecret: mcfgv1alpha1.ImageSecretObjectReference{
					Name: opts.pushSecretName,
				},
				RenderedImagePushspec: opts.finalImagePullspec,
				ImageBuilder: &mcfgv1alpha1.MachineOSImageBuilder{
					ImageBuilderType: mcfgv1alpha1.PodBuilder,
				},
				Containerfile: []mcfgv1alpha1.MachineOSContainerfile{
					{
						ContainerfileArch: mcfgv1alpha1.NoArch,
						Content:           opts.containerfileContents,
					},
				},
			},
			BuildOutputs: mcfgv1alpha1.BuildOutputs{
				CurrentImagePullSecret: mcfgv1alpha1.ImageSecretObjectReference{
					Name: opts.pullSecretName,
				},
			},
		},
	}
}

func getMachineOSConfigForPool(cs *framework.ClientSet, pool *mcfgv1.MachineConfigPool) (*mcfgv1alpha1.MachineOSConfig, error) {
	client := clientmachineconfigv1alpha1.NewForConfigOrDie(cs.GetRestConfig())

	moscList, err := client.MachineOSConfigs().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	found := filterMachineOSConfigsForPool(moscList, pool)
	if len(found) == 1 {
		return found[0], nil
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("no MachineOSConfigs exist for MachineConfigPool %s", pool.Name)
	}

	names := []string{}
	for _, mosc := range found {
		names = append(names, mosc.Name)
	}

	return nil, fmt.Errorf("expected one MachineOSConfig for MachineConfigPool %s, found multiple: %v", pool.Name, names)
}

func filterMachineOSConfigsForPool(moscList *mcfgv1alpha1.MachineOSConfigList, pool *mcfgv1.MachineConfigPool) []*mcfgv1alpha1.MachineOSConfig {
	found := []*mcfgv1alpha1.MachineOSConfig{}

	for _, mosc := range moscList.Items {
		if mosc.Spec.MachineConfigPool.Name == pool.Name {
			mosc := mosc
			found = append(found, &mosc)
		}
	}

	return found
}

func createMachineOSConfig(cs *framework.ClientSet, mosc *mcfgv1alpha1.MachineOSConfig) error {
	client := clientmachineconfigv1alpha1.NewForConfigOrDie(cs.GetRestConfig())

	_, err := client.MachineOSConfigs().Create(context.TODO(), mosc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("could not create MachineOSConfig %s: %w", mosc.Name, err)
	}

	klog.Infof("Created MachineOSConfig %s", mosc.Name)
	return nil
}

func deleteMachineOSConfigs(cs *framework.ClientSet) error {
	client := clientmachineconfigv1alpha1.NewForConfigOrDie(cs.GetRestConfig())

	moscList, err := client.MachineOSConfigs().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mosc := range moscList.Items {
		err := client.MachineOSConfigs().Delete(context.TODO(), mosc.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("could not delete MachineOSConfig %s: %w", mosc.Name, err)
		}

		klog.Infof("Deleted MachineOSConfig %s", mosc.Name)
	}

	return err
}

func deleteMachineOSBuilds(cs *framework.ClientSet) error {
	client := clientmachineconfigv1alpha1.NewForConfigOrDie(cs.GetRestConfig())

	mosbList, err := client.MachineOSBuilds().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mosb := range mosbList.Items {
		err := client.MachineOSBuilds().Delete(context.TODO(), mosb.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("could not delete MachineOSBuild %s: %w", mosb.Name, err)
		}

		klog.Infof("Deleted MachineOSBuild %s", mosb.Name)
	}

	return err
}

func isBuildDone(mosb *mcfgv1alpha1.MachineOSBuild, err error) (bool, error) {
	if err != nil {
		return false, err
	}

	state := ctrlcommon.NewMachineOSBuildState(mosb)

	if state.IsBuildFailure() {
		return false, fmt.Errorf("build %q failed", mosb.Name)
	}

	return state.IsBuildSuccess(), nil
}

func waitForBuildToComplete(cs *framework.ClientSet, poolName string) error {
	return waitForMachineOSBuildToReachState(cs, poolName, isBuildDone)
}

func waitForMachineOSBuildToReachState(cs *framework.ClientSet, poolName string, condFunc func(*mcfgv1alpha1.MachineOSBuild, error) (bool, error)) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*15)
	defer cancel()

	// TODO: This may eventually need to be moved into the loop to check if the
	// MachineConfigPool gets a new MachineConfig while the build is starting.
	mcp, err := cs.MachineconfigurationV1Interface.MachineConfigPools().Get(context.TODO(), poolName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	return wait.PollUntilContextCancel(ctx, time.Second, true, func(_ context.Context) (bool, error) {
		mosb, err := utils.GetMachineOSBuildForPool(ctx, cs, mcp)
		if err != nil {
			return false, err
		}

		return condFunc(mosb, err)
	})
}
