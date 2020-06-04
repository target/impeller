package report

import (
	"fmt"
	"os"
	"text/template"
)

type Report struct {
	ReportFile   string
	ReportHeader string
	ReportLines  map[ReportKey]ReportDetail
}

// Report
type ReportKey struct {
	Name      string
	Cluster   string
	Namespace string
}

type ReportDetail struct {
	Version      string
	ChartPath    string
	ChartsSource string
	Overrides    string
	ValueFiles   string
}

func NewReport() Report {

	return Report{
		ReportFile:   "auditreport.csv",
		ReportHeader: "Name,Cluster,Namespace,Version,ChartPath,ChartsSource,ValueFiles",
		ReportLines:  make(map[ReportKey]ReportDetail),
	}
}

func (rpt *Report) Add(reportkey ReportKey, detail ReportDetail) {
	rpt.ReportLines[reportkey] = detail
}

func (rpt *Report) Write(fName string) error {
	type reportline struct {
		Name         string
		Cluster      string
		Namespace    string
		Version      string
		ChartPath    string
		ChartsSource string
		ValueFiles   string
	}
	fd, err := os.Create(fName)
	if err != nil {
		return err
	}
	defer fd.Close()

	// Report header
	fmt.Fprintln(fd, rpt.ReportHeader)
	for key, line := range rpt.ReportLines {
		t, err := template.New("Report").Parse("{{.Name}},{{.Cluster}},{{.Namespace}},{{.Version}},{{.ChartPath}},{{.ChartsSource}},{{.ValueFiles}}\n")
		if err != nil {
			panic(err)
		}
		item := reportline{
			Name:         key.Name,
			Cluster:      key.Cluster,
			Namespace:    key.Namespace,
			Version:      line.Version,
			ChartPath:    line.ChartPath,
			ChartsSource: line.ChartsSource,
			ValueFiles:   line.ValueFiles,
		}
		err = t.Execute(fd, item)
		if err != nil {
			panic(err)
		}

	}
	fd.Sync()

	return nil
}

type Clusters struct {
	ClusterList map[string]bool
}

func NewClusters() Clusters {
	return Clusters{
		ClusterList: make(map[string]bool),
	}
}

func (cl *Clusters) Add(cluster string) {
	if _, ok := cl.ClusterList[cluster]; !ok {
		cl.ClusterList[cluster] = true
	}
}
