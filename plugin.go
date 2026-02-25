package main

import (
	"encoding/base64"
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
	KubeConfigFile    string
	KubeConfigBase64  bool
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
	var err error
	switch release.DeploymentMethod {
	case "kubectl":
		err = p.installAddonViaKubectl(release)
	case "helm":
		fallthrough
	default:
		err = p.installAddonViaHelm(release)
	}
	
	if err != nil {
		return err
	}
	
	// Wait for resources to be ready
	if err := p.waitForResources(release); err != nil {
		return err
	}
	
	// Apply additional kubectl files after resources are ready
	if err := p.applyKubectlFiles(release); err != nil {
		return err
	}
	
	return nil
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
		// Force recreate resources if immutable fields change
		if release.Force {
			log.Println("Force flag enabled: will recreate resources with immutable field changes")
			cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "--force"})
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
		// Create namespace if it doesn't exist (won't fail if it already exists like kube-system)
		if !p.Diffrun {
			cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "--create-namespace"})
		}
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
	// Diff Run
	if p.Diffrun {
		log.Println("Running Diff run:", release.Name)
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "diff"})
	} else {
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "apply"})
	}
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
	if err := utils.Run(kubectlApplyCmd, false); err != nil && !(p.Diffrun && release.DeploymentMethod == "kubectl") {
		kubectlApplyCmd := cb.Command()
		kubectlApplyCmd.Stdin = strings.NewReader(renderedManifests)
		return utils.Run(kubectlApplyCmd, false)
	}
	return nil
}

// waitForResources waits for deployments, daemonsets, and statefulsets to be ready
func (p *Plugin) waitForResources(release *types.Release) error {
	// Skip waiting if dry-run or diff-run
	if p.Dryrun || p.Diffrun {
		return nil
	}

	// Wait for Deployments
	for _, deployment := range release.WaitforDeployment {
		log.Printf("Waiting for Deployment: %s", deployment)
		if err := p.waitForResource("deployment", deployment, release.Namespace); err != nil {
			return fmt.Errorf("error waiting for deployment \"%s\": %v", deployment, err)
		}
	}

	// Wait for DaemonSets
	for _, daemonset := range release.WaitforDaemonSet {
		log.Printf("Waiting for DaemonSet: %s", daemonset)
		if err := p.waitForResource("daemonset", daemonset, release.Namespace); err != nil {
			return fmt.Errorf("error waiting for daemonset \"%s\": %v", daemonset, err)
		}
	}

	// Wait for StatefulSets
	for _, statefulset := range release.WaitforStatefulSet {
		log.Printf("Waiting for StatefulSet: %s", statefulset)
		if err := p.waitForResource("statefulset", statefulset, release.Namespace); err != nil {
			return fmt.Errorf("error waiting for statefulset \"%s\": %v", statefulset, err)
		}
	}

	return nil
}

// waitForResource uses kubectl wait to check if a resource is ready (read-only operation)
// Note: DaemonSets use rollout status instead of wait, as they don't support condition=ready
func (p *Plugin) waitForResource(resourceType, resourceName, namespace string) error {
	// DaemonSets need special handling - use rollout status
	if resourceType == "daemonset" {
		return p.waitForDaemonSet(resourceName, namespace)
	}
	
	cb := commandbuilder.CommandBuilder{Name: constants.KubectlBin}
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "wait"})
	
	// Set the appropriate condition based on resource type
	var condition string
	switch resourceType {
	case "deployment":
		condition = "condition=available"
	case "statefulset":
		condition = "condition=ready"
	default:
		condition = "condition=ready"
	}
	
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "for", Value: condition})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: fmt.Sprintf("%s/%s", resourceType, resourceName)})
	
	if namespace != "" {
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "namespace", Value: namespace})
	}
	
	if p.KubeContext != "" {
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "context", Value: p.KubeContext})
	}

	// No timeout - wait until resources are ready
	if err := cb.Run(); err != nil {
		return fmt.Errorf("wait for ready state failed: %v", err)
	}
	
	log.Printf("Successfully verified %s/%s is ready", resourceType, resourceName)
	return nil
}

// waitForDaemonSet uses kubectl rollout status for DaemonSets (read-only, doesn't trigger rollouts)
func (p *Plugin) waitForDaemonSet(resourceName, namespace string) error {
	cb := commandbuilder.CommandBuilder{Name: constants.KubectlBin}
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "rollout"})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "status"})
	cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: fmt.Sprintf("daemonset/%s", resourceName)})
	
	if namespace != "" {
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "namespace", Value: namespace})
	}
	
	if p.KubeContext != "" {
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "context", Value: p.KubeContext})
	}

	if err := cb.Run(); err != nil {
		return fmt.Errorf("rollout status check failed: %v", err)
	}
	
	log.Printf("Successfully verified daemonset/%s is ready", resourceName)
	return nil
}

// applyKubectlFiles applies additional kubectl manifest files after deployment
// Supports both individual files and directories (applies all .yaml/.yml files in directory)
func (p *Plugin) applyKubectlFiles(release *types.Release) error {
	// Skip if dry-run or diff-run
	if p.Dryrun || p.Diffrun {
		return nil
	}

	// Skip if no kubectl files are specified
	if len(release.KubectlFiles) == 0 {
		return nil
	}

	log.Println("Applying additional kubectl files for:", release.Name)

	for _, path := range release.KubectlFiles {
		// Check if path is a file or directory
		fileInfo, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("error accessing path \"%s\": %v", path, err)
		}

		var filesToApply []string
		if fileInfo.IsDir() {
			// If it's a directory, get all YAML files in it
			log.Printf("Processing directory: %s", path)
			files, err := p.getYAMLFilesFromDir(path)
			if err != nil {
				return fmt.Errorf("error reading directory \"%s\": %v", path, err)
			}
			filesToApply = files
		} else {
			// If it's a file, apply it directly
			filesToApply = []string{path}
		}

		// Apply each file
		for _, file := range filesToApply {
			log.Printf("Applying kubectl file: %s", file)
			
			cb := commandbuilder.CommandBuilder{Name: constants.KubectlBin}
			cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "apply"})
			cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "filename", Value: file})
			
			// Don't force namespace - let the manifest define its own namespace
			// This allows resources to be created in their specified namespaces
			
			if p.KubeContext != "" {
				cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "context", Value: p.KubeContext})
			}

			if err := cb.Run(); err != nil {
				return fmt.Errorf("error applying kubectl file \"%s\": %v", file, err)
			}
			
			log.Printf("Successfully applied kubectl file: %s", file)
		}
	}

	return nil
}

// getYAMLFilesFromDir returns all .yaml and .yml files from a directory
// Excludes kustomization.yaml and Kustomization.yaml files
func (p *Plugin) getYAMLFilesFromDir(dirPath string) ([]string, error) {
	var yamlFiles []string
	
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		fileName := file.Name()
		
		// Skip kustomization files
		if fileName == "kustomization.yaml" || fileName == "Kustomization.yaml" || 
		   fileName == "kustomization.yml" || fileName == "Kustomization.yml" {
			log.Printf("Skipping kustomization file: %s", fileName)
			continue
		}
		
		if strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml") {
			fullPath := dirPath + "/" + fileName
			yamlFiles = append(yamlFiles, fullPath)
		}
	}

	if len(yamlFiles) == 0 {
		log.Printf("WARNING: No YAML files found in directory: %s", dirPath)
	} else {
		log.Printf("Found %d YAML file(s) in directory: %s", len(yamlFiles), dirPath)
	}

	return yamlFiles, nil
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
	var err error
	byteData := []byte{}

	if p.KubeConfig != "" {
		p.KubeConfigFile = kubeConfig + "-" + p.KubeContext
		log.Println("Creating Kubernetes configfile" + p.KubeConfigFile)
		if p.KubeConfigBase64 {
			log.Println("configfile is base64 encoded")
			byteData, err = base64.StdEncoding.DecodeString(p.KubeConfig)
			if err != nil {
				log.Fatalf("err %v", err)
			}

		} else {
			log.Println("configfile is not encoded")
			byteData = []byte(p.KubeConfig)
		}

		if err := ioutil.WriteFile(p.KubeConfigFile, byteData, 0600); err != nil {
			return fmt.Errorf("error creating kube config file: %v", err)
		}
		log.Println("setting KUBECONFIG environment variable to:  " + p.KubeConfigFile)
		err := os.Setenv("KUBECONFIG", p.KubeConfigFile)
		if err != nil {
			log.Fatalf("err %v", err)
		}
	}

	// Providing a Kubernetes config context is mostly used for Drone support.
	// If not provided, the current context from Kubernetes config is used.
	if p.KubeContext != "" {
		log.Println("Setting Kubernetes context")
		cb := commandbuilder.CommandBuilder{Name: kubectlBin}
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "config"})
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: "use-context"})
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeRaw, Value: p.KubeContext})
		cb.Add(commandbuilder.Arg{Type: commandbuilder.ArgTypeLongParam, Name: "kubeconfig", Value: p.KubeConfigFile})
		cmd := cb.Command()
		if err := utils.Run(cmd, false); err != nil {
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
