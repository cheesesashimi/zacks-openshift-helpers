package installconfig

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/ghodss/yaml"
)

//go:embed base-install-config-amd64.yaml
var baseInstallConfigAMD64 []byte

//go:embed base-install-config-arm64.yaml
var baseInstallConfigARM64 []byte

type Opts struct {
	Username       string
	Arch           string
	Kind           string
	SSHKeyPath     string
	PullSecretPath string
}

func (o *Opts) ClusterName() string {
	return fmt.Sprintf("%s-%s-%s", o.Username, o.Kind, o.Arch)
}

func (o *Opts) validate() error {
	if o.Username == "" {
		return fmt.Errorf("username must be provided")
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

	ic := map[string]map[string]struct{}{
		"ocp": {
			"amd64": {},
			"arm64": {},
			"multi": {},
		},
		"okd": {
			"amd64": struct{}{},
		},
		"okd-scos": {
			"amd64": struct{}{},
		},
	}

	if _, ok := ic[o.Kind]; !ok {
		return fmt.Errorf("invalid kind %q", o.Kind)
	}

	if _, ok := ic[o.Kind][o.Arch]; !ok {
		return fmt.Errorf("invalid arch %q for kind %q", o.Arch, o.Kind)
	}

	return nil
}

func (o *Opts) getBaseConfig() []byte {
	baseConfigs := map[string][]byte{
		"amd64": baseInstallConfigAMD64,
		"arm64": baseInstallConfigARM64,
		"multi": baseInstallConfigAMD64,
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

	if err := yaml.Unmarshal(opts.getBaseConfig(), &parsed); err != nil {
		return nil, err
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
