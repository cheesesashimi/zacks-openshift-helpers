package installconfig

import "github.com/ghodss/yaml"

// This is the local representation of the InstallConfig struct. It would be a
// much better idea to use the upstream struct in openshift/installer, but I
// don't feel like fighting with go mod right now.
type metadata struct {
	Name string `json:"name"`
}

type compute struct {
	Architecture string `json:"architecture"`
}

type installConfig struct {
	Metadata     metadata  `json:"metadata"`
	Compute      []compute `json:"compute"`
	ControlPlane compute   `json:"controlPlane"`
	PullSecret   string    `json:"pullSecret"`
	SSHKey       string    `json:"sshKey"`
	FeatureSet   string    `json:"featureSet"`
}

type ParsedInstallConfig struct {
	Name         string
	Architecture string
	SSHKey       string
	FeatureSet   string
	PullSecret   string
	rawCfg       []byte
}

func (p *ParsedInstallConfig) injectIntoRawCfg() ([]byte, error) {
	parsedSimple := map[string]interface{}{}

	if err := yaml.Unmarshal(p.rawCfg, &parsedSimple); err != nil {
		return nil, err
	}

	if p.FeatureSet != "" {
		parsedSimple["featureSet"] = p.FeatureSet
	}

	parsedSimple["pullSecret"] = p.PullSecret
	parsedSimple["sshKey"] = p.SSHKey
	parsedSimple["metadata"] = map[string]interface{}{
		"name":              p.Name,
		"creationTimestamp": nil,
	}

	rawCfg, err := yaml.Marshal(parsedSimple)
	if err != nil {
		return nil, err
	}

	p.rawCfg = rawCfg

	return rawCfg, nil
}

func ParseInstallConfig(icb []byte) (*ParsedInstallConfig, error) {
	ic := &installConfig{}

	if err := yaml.Unmarshal(icb, ic); err != nil {
		return nil, err
	}

	return &ParsedInstallConfig{
		Name: ic.Metadata.Name,
		// TODO: Support multiarch clusters.
		Architecture: ic.ControlPlane.Architecture,
		SSHKey:       ic.SSHKey,
		FeatureSet:   ic.FeatureSet,
		PullSecret:   ic.PullSecret,
		rawCfg:       icb,
	}, nil
}
