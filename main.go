package main

import (
	"fmt"
	"log"
	"os"

	"github.com/target/impeller/types"
	"github.com/target/impeller/utils"
	"github.com/target/impeller/utils/report"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "addon-manager"
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "cluster-config-path",
			Usage:  "Path to the cluster config",
			EnvVar: "CLUSTER_CONFIG,PLUGIN_CLUSTER_CONFIG,PARAMETER_CLUSTER_CONFIG",
		},
		cli.StringSliceFlag{
			Name:   "value-files",
			Usage:  "Helm value override files",
			EnvVar: "VALUE_FILES,PLUGIN_VALUE_FILES,PARAMETER_VALUE_FILES",
		},
		cli.StringFlag{
			Name:   "kube-config",
			Usage:  "Kubernetes configuration file",
			EnvVar: "KUBE_CONFIG,PLUGIN_KUBE_CONFIG,PARAMETER_KUBE_CONFIG",
		},
		cli.StringFlag{
			Name:   "kube-context",
			Usage:  "Kubernetes configuration context to use",
			EnvVar: "KUBE_CONTEXT,PLUGIN_KUBE_CONTEXT,PARAMETER_KUBE_CONTEXT",
		},
		cli.BoolFlag{
			Name:   "dry-run",
			Usage:  "Enables a dry_run deployment",
			EnvVar: "DRY_RUN,PLUGIN_DRY_RUN,PARAMETER_DRY_RUN",
		},
		cli.BoolFlag{
			Name:   "diff-run",
			Usage:  "compares upgrade changes deployment",
			EnvVar: "DIFF_RUN,PLUGIN_DIFF_RUN,PARAMETER_DIFF_RUN",
		},
		cli.BoolFlag{
			Name:   "audit",
			Usage:  "create audit report",
			EnvVar: "AUDIT_RUN,PLUGIN_AUDIT_RUN,PARAMETER_AUDIT_RUN",
		},
		cli.StringFlag{
			Name:   "audit-file",
			Usage:  "audit report file name",
			EnvVar: "AUDIT_FILE_NAME,PLUGIN_AUDIT_FILE_NAME,PARAMETER_AUDIT_FILE_NAME",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx *cli.Context) error {
	var clusterConfig types.ClusterConfig
	var clist report.Clusters
	var auditReportFileName string
	var err error

	if ctx.String("cluster-config-path") == "" {
		return fmt.Errorf("Cluster config path not set.")
	}
	if os.Getenv("DRONE") == "true" {
		if ctx.String("kube-config") == "" {
			return fmt.Errorf("Kube config not set.")
		}
		if ctx.String("kube-context") == "" {
			return fmt.Errorf("Kube context not set.")
		}
	}
	if ctx.Bool("audit") {
		if ctx.String("audit-file") == "" {
			auditReportFileName = "./auditreport.csv"
		} else {
			auditReportFileName = ctx.String("audit-file")
		}
		clist, err = utils.ListClusters(ctx.String("cluster-config-path"))
		if err != nil {
			return fmt.Errorf("Error reading cluster config: %v", err)
		}
	} else {
		clusterConfig, err = utils.ReadClusterConfig(ctx.String("cluster-config-path"))
		if err != nil {
			return fmt.Errorf("Error reading cluster config: %v", err)
		}
	}

	plugin := Plugin{
		ClusterConfig:     clusterConfig,
		ClusterConfigPath: ctx.String("cluster-config-path"),
		ClustersList:      clist,
		ValueFiles:        ctx.StringSlice("value-files"),
		KubeConfig:        ctx.String("kube-config"),
		KubeContext:       ctx.String("kube-context"),
		Dryrun:            ctx.Bool("dry-run"),
		Diffrun:           ctx.Bool("diff-run"),
		Audit:             ctx.Bool("audit"),
		AuditFile:         auditReportFileName,
	}

	return plugin.Exec()
}
