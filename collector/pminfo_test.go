package collector

import (
	"prometheus-smcipmi-exporter/util"
	"testing"
)

func TestParsePminfoModule(t *testing.T, pminfoFile string) {

	var c PminfoCollector

	pminfoData := util.MustReadFile(&pminfoFile)

	metrics := c.parsePminfoModules(pminfoData)

	if len(metrics) == 0 {
		t.Error("No pminfo metrics recieved")
	}
}
