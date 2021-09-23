package types

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/target/impeller/utils/commandbuilder"
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
	Name             string     `yaml:"name"`
	DeploymentMethod string     `yaml:"deploymentMethod,omitempty"`
	Version          string     `yaml:"version"`
	ChartPath        string     `yaml:"chartPath"`
	ChartsSource     string     `yaml:"chartsSource"`
	History          uint       `yaml:history`
	Overrides        []Override `yaml:"overrides,omitempty"`
	Namespace        string     `yaml:"namespace,omitempty"`
	ValueFiles       []string   `yaml:"valueFiles,omitempty"`
}

type Override struct {
	Value  `yaml:",inline"`
	Target string `yaml:"target"`
}

/// BuildArg creates a commandbuilder.Arg for either a `--set` or
/// `--set-file` argument. If the value is provided directly or as an
/// environment variable, `--set` will be used. If a file is provided,
/// then `--set-file` will be used.
func (o Override) BuildArg() (*commandbuilder.Arg, error) {
	return o.Value.BuildArg(o.Target)
}

type HelmConfig struct {
	Upgrade           bool              `yaml:"upgrade"`
	SkipSetupHelmRepo bool              `yaml:"skipSetupHelmRepo"`
	DefaultHistory    uint              `yaml:"defaultHistory"`
	Debug             bool              `yaml:"debug"`
	LogLevel          uint              `yaml:"log"`
	ServiceAccount    string            `yaml:"serviceAccount"`
	Repos             []HelmRepo        `yaml:"repos"`
	Overrides         map[string]string `yaml:"overrides"`
}

type Value struct {
	Value     *string    `yaml:"value,omitempty"`
	ValueFrom *ValueFrom `yaml:"valueFrom,omitempty"`
	ShowValue bool       `yaml:"showValue"`
}

func (v Value) BuildArg(name string) (*commandbuilder.Arg, error) {
	if v.Value != nil {
		if *v.Value == "" {
			log.Println("WARNING: Override value is blank.")
		}
		return &commandbuilder.Arg{
			Type:        commandbuilder.ArgTypeLongParam,
			Name:        "set",
			Value:       fmt.Sprintf("%s=%s", name, *v.Value),
			ValueSecret: !v.ShowValue,
		}, nil
	} else if v.ValueFrom != nil {
		return v.ValueFrom.BuildArg(name, v.ShowValue)
	} else {
		return nil, fmt.Errorf("no value provided")
	}
}

/// GetValue returns the override value as a string. If a file or environment
/// variable is set, the contents are read import ()and returned as a string.
func (v Value) GetValue() (string, error) {
	if v.Value != nil {
		return *v.Value, nil
	}
	if v.ValueFrom != nil {
		return v.ValueFrom.GetValue()
	}
	return "", fmt.Errorf("no value provided")
}

type ValueFrom struct {
	Environment string `yaml:"environment"`
	File        string `yaml:"file"`
}

func (vf ValueFrom) BuildArg(name string, show bool) (*commandbuilder.Arg, error) {
	if vf.Environment != "" {
		value, err := vf.GetValue()
		if err != nil {
			return nil, err
		}
		if value == "" {
			log.Println("WARNING: Override value is blank.")
		}
		return &commandbuilder.Arg{
			Type:        commandbuilder.ArgTypeLongParam,
			Name:        "set",
			Value:       fmt.Sprintf("%s=%s", name, value),
			ValueSecret: !show,
		}, nil
	} else if vf.File != "" {
		return &commandbuilder.Arg{
			Type:        commandbuilder.ArgTypeLongParam,
			Name:        "set-file",
			Value:       fmt.Sprintf("%s=%s", name, vf.File),
			ValueSecret: false,
		}, nil
	} else {
		return nil, fmt.Errorf("no source specified for ValueFrom")
	}
}

func (vf ValueFrom) GetValue() (string, error) {
	if vf.Environment != "" {
		return os.Getenv(vf.Environment), nil
	} else if vf.File != "" {
		bytes, err := ioutil.ReadFile(vf.File)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
	return "", fmt.Errorf("no source specified for ValueFrom")
}
