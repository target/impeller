package types

import (
	"fmt"
	"os"
)

type ClusterConfig struct {
	Name     string     `yaml:"name"`
	Releases []Release  `yaml:"releases"`
	Helm     HelmConfig `yaml:"helm"`
}

type HelmRepo struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Username *Value `yaml:"username,omitempty"`
	Password *Value `yaml:"password,omitempty"`
}

type Release struct {
	Name       string     `yaml:"name"`
	Version    string     `yaml:"version"`
	ChartPath  string     `yaml:"chartPath"`
	Overrides  []Override `yaml:"overrides,omitempty"`
	Namespace  string     `yaml:"namespace,omitempty"`
	ValueFiles []string   `yaml:"valueFiles,omitempty"`
}

type Override struct {
	Value  `yaml:",inline"`
	Target string `yaml:"target"`
}

type HelmConfig struct {
	Upgrade    bool              `yaml:"upgrade"`
	MaxHistory int               `yaml:"maxHistory"`
	Namespace  string            `yaml:"namespace"`
	Repos      []HelmRepo        `yaml:"repos"`
	Overrides  map[string]string `yaml:"overrides"`
}

type Value struct {
	Value     *string    `yaml:"value,omitempty"`
	ValueFrom *ValueFrom `yaml:"valueFrom,omitempty"`
}

func (v Value) GetValue() (string, error) {
	if v.Value != nil {
		return *v.Value, nil
	}
	if v.ValueFrom != nil {
		return v.ValueFrom.GetValue()
	}
	return "", fmt.Errorf("No value provided")
}

type ValueFrom struct {
	Environment string `yaml:"environment"`
}

func (vf ValueFrom) GetValue() (string, error) {
	return os.Getenv(vf.Environment), nil
}
