package utils

import (
	"context"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	"github.com/openshift/machine-config-operator/test/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetMachineOSBuildForPool(ctx context.Context, cs *framework.ClientSet, mcp *mcfgv1.MachineConfigPool) (*mcfgv1alpha1.MachineOSBuild, error) {
	mosbList, err := cs.MachineOSBuilds().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, mosb := range mosbList.Items {
		mosb := mosb
		if mosb.Spec.DesiredConfig.Name == mcp.Spec.Configuration.Name {
			return &mosb, nil
		}
	}

	return nil, nil
}

func GetMachineOSConfigForPool(ctx context.Context, cs *framework.ClientSet, mcp *mcfgv1.MachineConfigPool) (*mcfgv1alpha1.MachineOSConfig, error) {
	moscList, err := cs.MachineOSConfigs().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, mosc := range moscList.Items {
		mosc := mosc
		if mosc.Spec.MachineConfigPool.Name == mcp.Name {
			return &mosc, nil
		}
	}

	return nil, nil
}
