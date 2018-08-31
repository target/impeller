package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/target/helm-propeller/constants"
	"github.com/target/helm-propeller/types"
	"github.com/target/helm-propeller/utils"
	"github.com/target/helm-propeller/utils/commandbuilder"
)

const (
	kubectlBin = "/usr/bin/kubectl"
	kubeConfig = "/root/.kube/config"
)

type Plugin struct {
	ClusterConfig types.ClusterConfig
	ValueFiles    []string
	KubeConfig    string
	KubeContext   string
	Dryrun        bool
}

func (p *Plugin) Exec() error {
	// Init Kubernetes config
	if err := p.setupKubeconfig(); err != nil {
		return fmt.Errorf("Error initializing Kubernetes config: %v", err)
	}

	// Helm init
	if err := p.helmInit(); err != nil {
		return fmt.Errorf("Error initializing Helm: %v", err)
	}

	// Add configured repos
	for _, repo := range p.ClusterConfig.Helm.Repos {
		if err := p.addHelmRepo(repo); err != nil {
			return fmt.Errorf("Error adding Helm repo: %v", err)
		}
	}
	if err := p.updateHelmRepos(); err != nil {
		return fmt.Errorf("Error updating Helm repos: %v", err)
	}

	// Install addons
	for _, addon := range p.ClusterConfig.Addons {
		if err := p.installAddon(&addon); err != nil {
			return fmt.Errorf("Error installing addon \"%s\": %v", addon.Name, err)
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
			return fmt.Errorf("Could not get username for repo: %v", err)
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
			return fmt.Errorf("Could not get password for repo: %v", err)
		}
		cb.Add(commandbuilder.Arg{
			Type:        commandbuilder.ArgTypeLongParam,
			Name:        "password",
			Value:       password,
			ValueSecret: true,
		})
	}

	if err := cb.Run(); err != nil {
		return fmt.Errorf("Could not add repo \"%s\": %v", repo.Name, err)
	}
	return nil
}

func (p *Plugin) updateHelmRepos() error {
	log.Println("Updating Helm repos")
	cmd := exec.Command(constants.HelmBin, "repo", "update")
	if err := utils.Run(cmd, true); err != nil {
		return fmt.Errorf("Error updating helm repos: %v", err)
	}
	return nil
}

func (p *Plugin) installAddon(addon *types.ClusterAddon) error {
	log.Println("Installing addon:", addon.Name, "@", addon.Version)
	args := []string{constants.HelmBin, "upgrade", "--install", addon.Name, addon.ChartPath, "--version", addon.Version}

	// Add namespaces to command
	if addon.Namespace != "" {
		args = append(args, "--namespace", strings.TrimSpace(addon.Namespace))
	}
	if p.ClusterConfig.Helm.Namespace != "" {
		args = append(args, "--tiller-namespace", p.ClusterConfig.Helm.Namespace)
	}

	// Add Overrides
	args = append(args, p.overrides(addon)...)

	// Dry Run
	if p.Dryrun {
		log.Println("Running Dry run:", addon.Name)
		args = append(args, "--dry-run")
	}

	// Execute helm upgrade
	cmd := exec.Command("/bin/sh", "-c", strings.Join(args, " "))
	if err := utils.Run(cmd, false); err != nil {
		return fmt.Errorf("Error running helm: %v", err)
	}
	return nil
}

func (p *Plugin) setupKubeconfig() error {
	log.Println("Creating Kubernetes config")
	if err := ioutil.WriteFile(kubeConfig, []byte(p.KubeConfig), 0644); err != nil {
		return fmt.Errorf("Error creating kube config file: %v", err)
	}
	cmd := exec.Command(kubectlBin, "config", "use-context", p.KubeContext)
	if err := utils.Run(cmd, true); err != nil {
		return fmt.Errorf("Error setting Kubernetes context: %v", err)
	}
	return nil
}

func (p *Plugin) helmInit() error {
	log.Println("Initializing Helm")
	cmd := []string{"init", "--debug"}

	if !(p.ClusterConfig.Helm.DisableHistory) {
		cmd = append(cmd, "--history-max", "5")
	}
	if p.ClusterConfig.Helm.Upgrade {
		cmd = append(cmd, "--upgrade")
	}
	if p.ClusterConfig.Helm.Namespace != "" {
		cmd = append(cmd, "--tiller-namespace", p.ClusterConfig.Helm.Namespace)
	}
	if len(p.ClusterConfig.Helm.Overrides) > 0 {
		overrides := []string{}
		for overrideKey, overrideValue := range p.ClusterConfig.Helm.Overrides {
			overrides = append(overrides, fmt.Sprintf("'%v'='%v'", overrideKey, overrideValue))
		}
		cmd = append(cmd, "--override", strings.Join(overrides, ","))
	}
	if p.Dryrun {
		cmd = append(cmd, "--client-only")
	}

	if err := utils.Run(exec.Command(constants.HelmBin, cmd...), true); err != nil {
		return err
	}

	return utils.PollTiller(10, p.ClusterConfig.Helm.Namespace)
}

func (p *Plugin) overrides(addon *types.ClusterAddon) []string {
	args := []string{}

	// Add override files
	for _, fileName := range p.ValueFiles {
		log.Println("Adding override file:", fileName)
		args = append(args, "-f", strings.TrimSpace(fileName))
	}
	path := fmt.Sprintf("values/%s/default.yaml", addon.Name)
	if _, err := os.Stat(path); err == nil {
		log.Println("Adding override file:", path)
		args = append(args, "-f", path)
	}
	for _, path := range addon.ValueFiles {
		if _, err := os.Stat(path); err != nil {
			log.Println("WARN: Value file does not exist:", path)
			continue
		}
		log.Println("Adding override file:", path)
		args = append(args, "-f", path)
	}
	path = fmt.Sprintf("values/%s/%s.yaml", addon.Name, p.ClusterConfig.Name)
	if _, err := os.Stat(path); p.ClusterConfig.Name != "" && err == nil {
		log.Println("Adding override file:", path)
		args = append(args, "-f", path)
	}

	// Handle individual value overrides
	setValues := []string{}
	for _, override := range addon.Overrides {
		log.Println("Overriding value for:", override.Target)
		overrideValue, err := override.GetValue()
		if err != nil {
			log.Println("WARNING: Could not get override value. Skipping override:", err)
			continue
		}
		if overrideValue == "" {
			log.Println("WARNING: Override value is blank.")
		}
		setValues = append(setValues, fmt.Sprintf("%s=%s", override.Target, overrideValue))
	}
	if len(setValues) > 0 {
		args = append(args, "--set", strings.Join(setValues, ","))
	}
	return args
}
