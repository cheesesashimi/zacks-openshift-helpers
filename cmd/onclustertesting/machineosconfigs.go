package main

import (
	"context"
	"fmt"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	clientmachineconfigv1alpha1 "github.com/openshift/client-go/machineconfiguration/clientset/versioned/typed/machineconfiguration/v1alpha1"
	"github.com/openshift/machine-config-operator/test/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

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
