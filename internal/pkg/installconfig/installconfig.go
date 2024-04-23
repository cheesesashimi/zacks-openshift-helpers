package installconfig

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Architectures
const (
	aarch64 string = "aarch64"
	arm64   string = "arm64"
	amd64   string = "amd64"
	multi   string = "multi"
)

// Kinds
const (
	ocp     string = "ocp"
	okd     string = "okd"
	okdSCOS string = "okd-scos"
)

// Variants
const (
	singleNode string = "single-node"
)

func GetSupportedKinds() sets.Set[string] {
	return sets.KeySet(GetSupportedArchesAndKinds())
}

func GetSupportedArches() sets.Set[string] {
	return sets.New[string]([]string{amd64, arm64, multi}...)
}

func GetSupportedArchesAndKinds() map[string]map[string]struct{} {
	return map[string]map[string]struct{}{
		ocp: {
			amd64:   {},
			aarch64: {},
			arm64:   {},
			multi:   {},
		},
		okd: {
			amd64: struct{}{},
		},
		okdSCOS: {
			amd64: struct{}{},
		},
	}
}

func IsValidKindAndArch(kind, arch string) (bool, error) {
	ic := GetSupportedArchesAndKinds()

	if _, ok := ic[kind]; !ok {
		return false, fmt.Errorf("invalid kind %q, valid kinds: %v", kind, sets.StringKeySet(ic).List())
	}

	if _, ok := ic[kind][arch]; !ok {
		return false, fmt.Errorf("invalid arch %q for kind %q, valid arch(s): %v", arch, kind, sets.StringKeySet(ic[kind]).List())
	}

	return true, nil
}

func GetSupportedVariants() sets.Set[string] {
	return sets.New[string](singleNode)
}

func IsSupportedVariant(variant string) (bool, error) {
	vars := GetSupportedVariants()
	if vars.Has(variant) {
		return true, nil
	}

	return false, fmt.Errorf("invalid variant %q, valid variant(s): %v", variant, sets.List(vars))
}

//go:embed single-node-install-config-amd64.yaml
var singleNodeInstallConfigAMD64 []byte

//go:embed single-node-install-config-arm64.yaml
var singleNodeInstallConfigARM64 []byte

//go:embed base-install-config-amd64.yaml
var baseInstallConfigAMD64 []byte

//go:embed base-install-config-arm64.yaml
var baseInstallConfigARM64 []byte

type Opts struct {
	Prefix            string
	Arch              string
	Kind              string
	SSHKeyPath        string
	PullSecretPath    string
	Variant           string
	EnableTechPreview bool
}

func (o *Opts) ClusterName() string {
	baseName := fmt.Sprintf("%s-%s-%s", o.Prefix, o.Kind, o.Arch)
	if o.Variant == "" {
		return baseName
	}

	if o.Variant == singleNode {
		return fmt.Sprintf("%s-sno", baseName)
	}

	return fmt.Sprintf("%s-%s", baseName, o.Variant)
}

func (o *Opts) validateVariant() error {
	if _, err := IsSupportedVariant(o.Variant); err != nil {
		return err
	}

	supportedSNOArches := sets.NewString(amd64, arm64, aarch64)

	if o.Variant == "single-node" && !supportedSNOArches.Has(o.Arch) {
		return fmt.Errorf("arch %q is unsupported by single-node variant", o.Arch)
	}

	return nil
}

func (o *Opts) validate() error {
	if o.Prefix == "" {
		return fmt.Errorf("prefix must be provided")
	}

	if o.SSHKeyPath == "" {
		return fmt.Errorf("ssh key path must be provided")
	}

	if _, err := os.Stat(o.SSHKeyPath); err != nil {
		return err
	}

	if o.PullSecretPath == "" {
		return fmt.Errorf("pull secret path must be provided")
	}

	if _, err := os.Stat(o.PullSecretPath); err != nil {
		return err
	}

	if o.Arch == "" {
		return fmt.Errorf("architecture must be provided")
	}

	if o.Kind == "" {
		return fmt.Errorf("kind must be provided")
	}

	if _, err := IsValidKindAndArch(o.Kind, o.Arch); err != nil {
		return err
	}

	if o.Variant != "" {
		if err := o.validateVariant(); err != nil {
			return err
		}
	}

	return nil
}

func (o *Opts) getSingleNodeConfig() []byte {
	snoConfigs := map[string][]byte{
		aarch64: singleNodeInstallConfigARM64,
		amd64:   singleNodeInstallConfigAMD64,
		arm64:   singleNodeInstallConfigARM64,
	}

	return snoConfigs[o.Arch]
}

func (o *Opts) getBaseConfig() []byte {
	baseConfigs := map[string][]byte{
		aarch64: baseInstallConfigARM64,
		amd64:   baseInstallConfigAMD64,
		arm64:   baseInstallConfigARM64,
		// Multiarch starts with an AMD64 or ARM64, but we default to AMD64 here.
		multi: baseInstallConfigAMD64,
	}

	return baseConfigs[o.Arch]
}

func GetInstallConfig(opts Opts) ([]byte, error) {
	if err := opts.validate(); err != nil {
		return nil, fmt.Errorf("could not get install config: %w", err)
	}

	return renderConfig(opts)
}

func renderConfig(opts Opts) ([]byte, error) {
	pullSecret, err := loadFile(opts.PullSecretPath)
	if err != nil {
		return nil, err
	}

	sshKey, err := loadFile(opts.SSHKeyPath)
	if err != nil {
		return nil, err
	}

	// TODO: Use an actual struct for this.
	parsed := map[string]interface{}{}

	config := opts.getBaseConfig()
	if opts.Variant == singleNode {
		config = opts.getSingleNodeConfig()
	}

	if err := yaml.Unmarshal(config, &parsed); err != nil {
		return nil, err
	}

	if opts.EnableTechPreview {
		parsed["featureSet"] = "TechPreviewNoUpgrade"
	}

	parsed["pullSecret"] = pullSecret
	parsed["sshKey"] = sshKey
	parsed["metadata"] = map[string]interface{}{
		"name":              opts.ClusterName(),
		"creationTimestamp": nil,
	}

	return yaml.Marshal(parsed)
}

func loadFile(sshKeyPath string) (string, error) {
	out, err := os.ReadFile(sshKeyPath)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
