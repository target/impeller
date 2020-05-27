package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewReport(t *testing.T) {
	rep := NewReport()
	assert.Equal(t, "auditreport.csv", rep.ReportFile)
}

func TestReport_Write(t *testing.T) {
	rep := NewReport()
	rep.Write("test.csv")
}
func TestNewClusters(t *testing.T) {

}
