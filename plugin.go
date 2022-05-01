package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/target/impeller/constants"
	"github.com/target/impeller/types"
	"github.com/target/impeller/utils"
	"github.com/target/impeller/utils/commandbuilder"
	"github.com/target/impeller/utils/report"
)

const (
	kubectlBin = "kubectl"
)

var (
	kubeConfig = os.Getenv("HOME") + "/.kube/config"
)

type Plugin struct {
	ClusterConfig     types.ClusterConfig
	ClusterConfigPath string
	ClustersList      report.Clusters
	ValueFiles        []string
	KubeConfig        string
	KubeContext       string
	Dryrun            bool
	Diffrun           bool
	Audit             bool
	AuditFile         string
}

func (p *Plugin) Exec() error {
	if !p.Audit {
		if !p.ClusterConfig.Helm.SkipSetupKubeConfig {
			// Init Kubernetes config
			if err := p.setupKubeconfig(); err != nil {
				return fmt.Errorf("error initializing Kubernetes config: %v", err)
			}
		} else {
			log.Println("Skipping setting up kubeconfig...")
		}
		// Add configured repos
		if !p.ClusterConfig.Helm.SkipSetupHelmRepo {
			for _, repo := range p.ClusterConfig.Helm.Repos {
				if err := p.addHelmRepo(repo); err != nil {
					return fmt.Errorf("error adding Helm repo: %v", err)
				}
			}
			if err := p.updateHelmRepos(); err != nil {
				return fmt.Errorf("error updating Helm repos: %v", err)
			}
		} else {
			log.Println("Skipping setting up Helm repos...")
		}
		if !p.ClusterConfig.Helm.SkipSetupKubeConfig {
			// Install addons
			for _, addon := range p.ClusterConfig.Releases {
				if err := p.installAddon(&addon); err != nil {
					return fmt.Errorf("error installing addon \"%s\": %v", addon.Name, err)
				}
			}
		}
	} else {
		log.Println("Generating Audit report:")
		rpt := report.NewReport()
		for cluster := range p.ClustersList.ClusterList {
			clusterConfig, err := utils.ReadClusterConfig(p.ClusterConfigPath + "/" + cluster)
			if err != nil {
				return fmt.Errorf("error reading cluster config: %v", err)
			}

			for _, addon := range clusterConfig.Releases {
				rpt.Add(report.ReportKey{
					Name:      addon.Name,
					Cluster:   cluster,
					Namespace: addon.Namespace,
				}, report.ReportDetail{
					Version:      addon.Version,
					Overrides:    "",
					ChartPath:    addon.ChartPath,
					ChartsSource: addon.ChartsSource,
					ValueFiles:   utils.GetValueFiles(&addon.ValueFiles),
				})
			}
		}
		// write report to output file
		err := rpt.Write(p.AuditFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Plugin) addHelmRepo(repo types.HelmRepo) error {
	log.Println("Adding Helm repo:", repo.Name)
	cb := commandbuilder.CommandBuilder{Name: constants.HelmBin}
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "repo"})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "add"})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: repo.Name})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: repo.URL})

	if repo.Username != nil {
		username, err := repo.Username.GetValue()
		if err != nil {
			return fmt.Errorf("could not get username for repo: %v", err)
		}
		cb.Add(commandbuilder.Arg{
			Type:        commandbuilder.ArgTypeLongParam,
			Name:        "username",
			Value:       username,
			ValueSecret: true,
		})
	}

	if repo.Password != nil {
		password, err := repo.Password.GetValue()
		if err != nil {
			return fmt.Errorf("could not get password for repo: %v", err)
		}
		cb.Add(commandbuilder.Arg{
			Type:        commandbuilder.ArgTypeLongParam,
			Name:        "password",
			Value:       password,
			ValueSecret: true,
		})
	}

	if err := cb.Run(); err != nil {
		return fmt.Errorf("could not add repo \"%s\": %v", repo.Name, err)
	}
	return nil
}

func (p *Plugin) updateHelmRepos() error {
	log.Println("Updating Helm repos")
	cmd := exec.Command(constants.HelmBin, "repo", "update")
	if err := utils.Run(cmd, true); err != nil {
		return fmt.Errorf("error updating helm repos: %v", err)
	}
	return nil
}

func (p *Plugin) installAddon(release *types.Release) error {
	log.Println("Installing addon:", release.Name, "@", release.Version)
	switch release.DeploymentMethod {
	case "kubectl":
		return p.installAddonViaKubectl(release)
	case "helm":
		fallthrough
	default:
		return p.installAddonViaHelm(release)
	}
}

// installAddonViaHelm installs addons via helm upgrade --install RELEASE CHART
func (p *Plugin) installAddonViaHelm(release *types.Release) error {
	cb := commandbuilder.CommandBuilder{Name: constants.HelmBin}
	if p.Diffrun {
		log.Println("Running Diff plugin:", release.Name)
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "diff"})
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "upgrade"})
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "--allow-unreleased"})
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "--suppress-secrets"})
	} else {
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "upgrade"})
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "--install"})
		if release.History > 0 {
			cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "history-max", Value: fmt.Sprint(release.History)})
		} else if p.ClusterConfig.Helm.DefaultHistory > 0 {
			cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "history-max", Value: fmt.Sprint(p.ClusterConfig.Helm.DefaultHistory)})
		}
	}
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: release.Name})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: release.ChartPath})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "version", Value: release.Version})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "kube-context", Value: p.KubeContext})

	if p.ClusterConfig.Helm.Debug {
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "--debug"})
	}

	// Add namespaces to command
	if release.Namespace != "" {
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "namespace", Value: release.Namespace})
	}

	if p.ClusterConfig.Helm.LogLevel != 0 {
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "v", Value: fmt.Sprint(p.ClusterConfig.Helm.LogLevel)})
	}
	if release.ChartsSource != "" {
		log.Println("Charts Source defined for:", release.Name)

		_, err := p.downloadCharts(release)
		if err != nil {
			return fmt.Errorf("error downloading charts: %s", err)
		}
	}
	// Add Overrides
	for _, override := range p.overrides(release) {
		cb.Add(override)
	}

	// Dry Run
	if p.Dryrun {
		log.Println("Running Dry run:", release.Name)
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "--dry-run"})
	}

	// Execute helm upgrade
	if err := cb.Run(); err != nil {
		return fmt.Errorf("error running helm: %v", err)
	}
	return nil
}

// installAddonViaKubectl installs addons via:
// helm fetch --version release.Version --untar release.ChartPath
// helm template $CHART | kubectl create -f -
func (p *Plugin) installAddonViaKubectl(release *types.Release) error {
	renderedManifests, err := p.templateChart(release)
	if err != nil {
		return fmt.Errorf("error rendering chart for kubectl apply: %s", err)
	}

	cb := commandbuilder.CommandBuilder{Name: constants.KubectlBin}
	// kubectl apply -f -
	if release.Namespace != "" {
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "namespace", Value: release.Namespace})
	}
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "context", Value: p.KubeContext})

	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "apply"})
	// Dry Run
	if p.Dryrun {
		log.Println("Running Dry run:", release.Name)
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "--dry-run=server"})
	}

	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "filename", Value: "-"})

	// Grab raw commandbuilder command so we can set stdin
	kubectlApplyCmd := cb.Command()
	kubectlApplyCmd.Stdin = strings.NewReader(renderedManifests)
	// We may need to run kubectl apply -f - twice if the helm chart
	// has dependant kubernetes resources. The first run will install
	// independent components and the second run will install the ones
	// that failed previously. If this command fails twice then the chart
	// is just broken
	if err := utils.Run(kubectlApplyCmd, false); err != nil {
		kubectlApplyCmd := cb.Command()
		kubectlApplyCmd.Stdin = strings.NewReader(renderedManifests)
		return utils.Run(kubectlApplyCmd, false)
	}
	return nil
}

func (p *Plugin) fetchChart(release *types.Release) error {
	cb := commandbuilder.CommandBuilder{Name: constants.HelmBin}
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "fetch"})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "version", Value: release.Version})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "--untar"})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: release.ChartPath})
	return cb.Run()
}

func (p *Plugin) downloadCharts(release *types.Release) (string, error) {

	if _, err := os.Stat("./downloads"); os.IsNotExist(err) {
		err := os.Mkdir("./downloads", 0755)
		if err != nil {
			return "", fmt.Errorf("error creting ./downloads folder: %s", err)
		}
	}
	myUrl, err := url.Parse(release.ChartsSource)
	if err != nil {
		return "", fmt.Errorf("error parsing charts myUrl: %s", err)
	}
	splits := strings.Split(myUrl.Path, "/")
	tarFilePath := "./downloads/" + splits[len(splits)-1]

	if _, err := os.Stat(tarFilePath); err != nil || os.IsExist(err) {
		log.Println("Downloading:", tarFilePath)
		cmd := exec.Command(constants.WgetBin, "-P", "./downloads", release.ChartsSource)
		if err := utils.Run(cmd, true); err != nil {
			return "", fmt.Errorf("error extracting Charts archive: %v", err)
		}
		err = p.extractCharts(tarFilePath)
		if err != nil {
			return tarFilePath, err
		}
	} else {
		log.Println("File exist, skipping download:", tarFilePath)
	}
	return tarFilePath, nil
}

func (p *Plugin) extractCharts(archiveName string) error {
	cmd := exec.Command(constants.TarBin, "-xzf", archiveName, "-C", "./downloads")
	if err := utils.Run(cmd, true); err != nil {
		return fmt.Errorf("error extracting Charts archive: %v", err)
	}
	return nil
}

func (p *Plugin) templateChart(release *types.Release) (string, error) {

	cb := commandbuilder.CommandBuilder{Name: constants.HelmBin}
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "template"})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: release.Name})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: release.ChartPath})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "version", Value: release.Version})
	// Add Overrides
	for _, override := range p.overrides(release) {
		cb.Add(override)
	}
	cmd := cb.Command()
	templateBytes, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(templateBytes), nil
}

func (p *Plugin) setupKubeconfig() error {
	// Providing a Kubernetes config is mostly used for Drone support.
	// If not provided, the default `kubectl` search path is used.
	// WARNING: this may overwrite your config if it already exists.
	if p.KubeConfig != "" {
		log.Println("Creating Kubernetes config")
		if err := ioutil.WriteFile(kubeConfig, []byte(p.KubeConfig), 0600); err != nil {
			return fmt.Errorf("error creating kube config file: %v", err)
		}
	}

	// Providing a Kubernetes config context is mostly used for Drone support.
	// If not provided, the current context from Kubernetes config is used.
	if p.KubeContext != "" {
		log.Println("Setting Kubernetes context")
		cmd := exec.Command(kubectlBin, "config", "use-context", p.KubeContext)
		if err := utils.Run(cmd, true); err != nil {
			return fmt.Errorf("error setting Kubernetes context: %v", err)
		}
	}
	return nil
}

func (p *Plugin) overrides(release *types.Release) (args []commandbuilder.Arg) {
	// Add override files
	for _, fileName := range p.ValueFiles {
		log.Println("Adding override file:", fileName)
		args = append(args, commandbuilder.Arg{
			Type:  commandbuilder.ArgTypeShortParam,
			Name:  "f",
			Value: strings.TrimSpace(fileName)})
	}
	path := fmt.Sprintf("values/%s/default.yaml", release.Name)
	if _, err := os.Stat(path); err == nil {
		log.Println("Adding override file:", path)
		args = append(args, commandbuilder.Arg{
			Type:  commandbuilder.ArgTypeShortParam,
			Name:  "f",
			Value: path})
	}
	for _, path := range release.ValueFiles {
		if _, err := os.Stat(path); err != nil {
			log.Println("WARN: Value file does not exist:", path)
			continue
		}
		log.Println("Adding override file:", path)
		args = append(args, commandbuilder.Arg{
			Type:  commandbuilder.ArgTypeShortParam,
			Name:  "f",
			Value: path})
	}
	path = fmt.Sprintf("values/%s/%s.yaml", release.Name, p.ClusterConfig.Name)
	if _, err := os.Stat(path); p.ClusterConfig.Name != "" && err == nil {
		log.Println("Adding override file:", path)
		args = append(args, commandbuilder.Arg{
			Type:  commandbuilder.ArgTypeShortParam,
			Name:  "f",
			Value: path})
	}
	// Handle individual value overrides
	for _, override := range release.Overrides {
		log.Println("Overriding value for:", override.Target)
		arg, err := override.BuildArg()
		if err != nil {
			log.Println("WARNING: Could not get override value. Skipping override:", err)
			continue
		}
		args = append(args, *arg)
	}

	return args
}
