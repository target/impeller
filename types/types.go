package types

type ClusterConfig struct {
	Name   string         `yaml:"name"`
	Addons []ClusterAddon `yaml:"addons"`
	Helm   HelmConfig     `yaml:"helm"`
}

type HelmRepo struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type ClusterAddon struct {
	Name       string     `yaml:"name"`
	Version    string     `yaml:"version"`
	ChartPath  string     `yaml:"chartPath"`
	Overrides  []Override `yaml:"overrides,omitempty"`
	Namespace  string     `yaml:"namespace,omitempty"`
	ValueFiles []string   `yaml:"valueFiles,omitempty"`
}

type Override struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

type HelmConfig struct {
	Upgrade        bool              `yaml:"upgrade"`
	DisableHistory bool              `yaml:"disableHistory"`
	Namespace      string            `yaml:"namespace"`
	Repos          []HelmRepo        `yaml:"repos"`
	Overrides      map[string]string `yaml:"overrides"`
}
