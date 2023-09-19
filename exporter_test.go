// Copyright 2023 Gabriele Iannetti <g.iannetti@gsi.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"flag"
	"os"
	"prometheus-smcipmi-exporter/collector"
	"prometheus-smcipmi-exporter/util"
	"testing"

	log "github.com/sirupsen/logrus"
)

const (
	defaultPminfoFile = "pminfo.txt"
)

var (
	pminfoFile *string
)

// TODO: create local test for exported metrics via HTTP...
func TestParsePminfoModule(t *testing.T) {

	var collector collector.PminfoCollector

	pminfoData := util.MustReadFile(pminfoFile)

	metrics := collector.CreateMetrics(pminfoData)

	for _, m := range metrics {
		log.Debug(m.Desc().String())
	}

	if len(metrics) == 0 {
		t.Error("No pminfo metrics recieved")
	}

	if len(metrics) != 12 {
		t.Error("Incomplete count of pminfo metrics recieved")
	}

}

func TestMain(m *testing.M) {

	pminfoFile = flag.String("pminfoFile", defaultPminfoFile, "A file with pminfo output generated from SMCIPMITool to be processed")
	logLevel := flag.String("log", defaultLogLevel, "Sets log level - ERROR, WARNING, INFO, DEBUG or TRACE")

	flag.Parse()

	initLogging(*logLevel)

	os.Exit(m.Run())
}
