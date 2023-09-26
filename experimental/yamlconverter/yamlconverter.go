package yamlconverter

import (
	"fmt"
	"reflect"

	"github.com/ghodss/yaml"
	"github.com/hexops/valast"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	mcfgv1resourceread "github.com/openshift/machine-config-operator/lib/resourceread"
	"k8s.io/apimachinery/pkg/runtime"
)

func KubeYAMLToStruct(in []byte) (string, error) {
	kubeObj, err := decodeIntoKubeObject(in)
	if err != nil {
		return "", fmt.Errorf("unable to decode into Kubernetes object: %w", err)
	}

	outOpts := &valast.Options{}

	outOpts.PackagePathToName = func(importPath string) (string, error) {
		pathToPackageNames := map[string]string{
			"github.com/openshift/api/config/v1": "configv1",
			"github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1": "mcfgv1",
			"k8s.io/api/apps/v1": "appsv1",
			"k8s.io/api/core/v1": "corev1",
			"k8s.io/api/rbac/v1": "rbacv1",
			"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1": "apiextensionsv1",
			"k8s.io/apimachinery/pkg/api/resource":                     "resource",
			"k8s.io/apimachinery/pkg/apis/meta/v1":                     "metav1",
			"k8s.io/apimachinery/pkg/util/intstr":                      "intstr",
			"k8s.io/apimachinery/pkg/runtime":                          "runtime",
		}

		pkgName, ok := pathToPackageNames[importPath]
		if ok {
			return pkgName, nil
		}

		return "", fmt.Errorf("unknown path %s", importPath)
	}

	_, err = valast.AST(reflect.ValueOf(kubeObj), outOpts)
	if err != nil {
		return "", fmt.Errorf("unable to parse struct: %w", err)
	}

	return valast.StringWithOptions(kubeObj, outOpts), nil
}

func decodeIntoKubeObject(in []byte) (runtime.Object, error) {
	// Do a trial decoding into an untyped struct so we can look up the object
	// kind. There is probably a more efficient and straightforward way to do
	// this that I am unaware of.
	initialDecoded := map[string]interface{}{}

	if err := yaml.Unmarshal(in, &initialDecoded); err != nil {
		return nil, fmt.Errorf("could not decode YAML: %w", err)
	}

	kind := initialDecoded["kind"].(string)

	if kind == "" {
		return nil, fmt.Errorf("missing kind field, is this Kubernetes YAML?")
	}

	var out runtime.Object

	switch kind {
	case "ClusterRole":
		out = resourceread.ReadClusterRoleV1OrDie(in)
	case "ClusterRoleBinding":
		out = resourceread.ReadClusterRoleBindingV1OrDie(in)
	case "ConfigMap":
		out = resourceread.ReadConfigMapV1OrDie(in)
	case "ControllerConfig":
		out = mcfgv1resourceread.ReadControllerConfigV1OrDie(in)
	case "CustomResourceDefinition":
		out = resourceread.ReadCustomResourceDefinitionV1OrDie(in)
	case "DaemonSet":
		out = resourceread.ReadDaemonSetV1OrDie(in)
	case "Deployment":
		out = resourceread.ReadDeploymentV1OrDie(in)
	case "MachineConfig":
		out = mcfgv1resourceread.ReadMachineConfigV1OrDie(in)
	case "MachineConfigPool":
		out = mcfgv1resourceread.ReadMachineConfigPoolV1OrDie(in)
	case "Pod":
		out = resourceread.ReadPodV1OrDie(in)
	case "Role":
		out = resourceread.ReadRoleV1OrDie(in)
	case "RoleBinding":
		out = resourceread.ReadRoleBindingV1OrDie(in)
	case "Secret":
		out = resourceread.ReadSecretV1OrDie(in)
	case "ServiceAccount":
		out = resourceread.ReadServiceAccountV1OrDie(in)
	default:
		return nil, fmt.Errorf("unknown kind %q", kind)
	}

	return out, nil
}
