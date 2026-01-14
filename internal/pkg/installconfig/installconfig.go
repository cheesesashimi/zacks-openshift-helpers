package installconfig

import (
	_ "embed"
	"fmt"
	"os"

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

type Paths struct {
	SSHKeyPath        string
	PullSecretPath    string
	InstallConfigPath string
}

type Opts struct {
	Prefix            string
	Arch              string
	Kind              string
	Variant           string
	EnableTechPreview bool
	Paths
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

func (o *Opts) validateForConfigGeneration() error {
	if o.Prefix == "" {
		return fmt.Errorf("prefix must be provided for config generation")
	}

	if o.Arch == "" {
		return fmt.Errorf("architecture must be provided for config generation")
	}

	if o.Kind == "" {
		return fmt.Errorf("kind must be provided for config generation")
	}

	if _, err := IsValidKindAndArch(o.Kind, o.Arch); err != nil {
		return err
	}

	if o.Variant != "" {
		if err := o.validateVariant(); err != nil {
			return err
		}
	}

	if o.SSHKeyPath == "" {
		return fmt.Errorf("ssh key path must be provided for config generation")
	}

	if _, err := os.Stat(o.SSHKeyPath); err != nil {
		return err
	}

	if o.PullSecretPath == "" {
		return fmt.Errorf("pull secret path must be provided for config generation")
	}

	if _, err := os.Stat(o.PullSecretPath); err != nil {
		return err
	}

	return nil
}

func (o *Opts) validateForConfigReuse() error {
	if _, err := os.Stat(o.InstallConfigPath); err != nil {
		return err
	}

	return nil
}

func (o *Opts) validate() error {
	if o.InstallConfigPath != "" {
		return o.validateForConfigReuse()
	}

	return o.validateForConfigGeneration()
}

func (o *Opts) getBaseConfigForGeneration() []byte {
	snoConfigs := map[string][]byte{
		aarch64: singleNodeInstallConfigARM64,
		amd64:   singleNodeInstallConfigAMD64,
		arm64:   singleNodeInstallConfigARM64,
	}

	if o.Variant == singleNode {
		return snoConfigs[o.Arch]
	}

	baseConfigs := map[string][]byte{
		aarch64: baseInstallConfigARM64,
		amd64:   baseInstallConfigAMD64,
		arm64:   baseInstallConfigARM64,
		// Multiarch starts with an AMD64 or ARM64, but we default to AMD64 here.
		multi: baseInstallConfigAMD64,
	}

	return baseConfigs[o.Arch]
}

func (o *Opts) loadInstallConfig() (*ParsedInstallConfig, error) {
	cfg, err := os.ReadFile(o.InstallConfigPath)
	if err != nil {
		return nil, err
	}

	return ParseInstallConfig(cfg)
}

func (o *Opts) getParsedBaseConfig() (*ParsedInstallConfig, error) {
	return ParseInstallConfig(o.getBaseConfigForGeneration())
}

func GetInstallConfig(opts Opts) ([]byte, error) {
	// TODO: Consolidate both of these paths.
	if opts.InstallConfigPath == "" {
		return generateInstallConfig(opts)
	}

	return prepareInstallConfig(opts)
}

func generateInstallConfig(opts Opts) ([]byte, error) {
	if err := opts.validate(); err != nil {
		return nil, fmt.Errorf("could not get install config: %w", err)
	}

	config, err := opts.getParsedBaseConfig()
	if err != nil {
		return nil, err
	}

	pullSecret, err := os.ReadFile(opts.PullSecretPath)
	if err != nil {
		return nil, err
	}

	sshKey, err := os.ReadFile(opts.SSHKeyPath)
	if err != nil {
		return nil, err
	}

	config.Name = opts.ClusterName()
	config.Architecture = opts.Arch
	config.SSHKey = string(sshKey)
	config.PullSecret = string(pullSecret)

	if opts.EnableTechPreview {
		config.FeatureSet = "TechPreviewNoUpgrade"
	}

	return config.injectIntoRawCfg()
}

// Reads in the given installconfig and injects the various values into it.
func prepareInstallConfig(opts Opts) ([]byte, error) {
	if err := opts.validateForConfigReuse(); err != nil {
		return nil, err
	}

	paths := opts.Paths

	config, err := opts.loadInstallConfig()
	if err != nil {
		return nil, err
	}

	if config.Name == "" {
		if opts.Prefix == "" {
			return nil, fmt.Errorf("missing prefix for empty name field")
		}

		if opts.Kind == "" {
			return nil, fmt.Errorf("missing kind for empty name field")
		}

		// Set this on the Opts struct to generate the name.
		opts.Arch = config.Architecture

		config.Name = opts.ClusterName()
	}

	if paths.PullSecretPath != "" {
		ps, err := os.ReadFile(paths.PullSecretPath)
		if err != nil {
			return nil, fmt.Errorf("could not read pull secret path from %s: %w", paths.PullSecretPath, err)
		}

		config.PullSecret = string(ps)
	}

	if config.PullSecret == "" {
		return nil, fmt.Errorf("pull secret config field empty")
	}

	if paths.SSHKeyPath != "" {
		sb, err := os.ReadFile(paths.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("could not read SSH key path from %s: %w", paths.SSHKeyPath, err)
		}

		config.SSHKey = string(sb)
	}

	if config.SSHKey == "" {
		return nil, fmt.Errorf("SSH key config field empty")
	}

	if opts.EnableTechPreview {
		config.FeatureSet = "TechPreviewNoUpgrade"
	}

	return config.injectIntoRawCfg()
}
