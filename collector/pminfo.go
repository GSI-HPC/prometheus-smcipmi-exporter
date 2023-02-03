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

package collector

import (
	"prometheus-smcipmi-exporter/util"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	log "github.com/sirupsen/logrus"
)

var (
	pminfoModuleRegex       = regexp.MustCompile(`(?ms:(?:\[SlaveAddress = [\d\w]+\] \[Module (?P<number>\d+)\])(?P<items>.*?)(?:^\s?$|\z))`)
	pminfoModuleNumberIndex = pminfoModuleRegex.SubexpIndex("number")
	pminfoModuleItemsIndex  = pminfoModuleRegex.SubexpIndex("items")

	pminfoItemRegex      = regexp.MustCompile(`(?m:(?P<name>(?:\s*[\w]+\s?)+)\s*\|\s*(?P<value>.*))`)
	pminfoItemNameIndex  = pminfoItemRegex.SubexpIndex("name")
	pminfoItemValueIndex = pminfoItemRegex.SubexpIndex("value")

	pminfoMetricTemplates = make(map[string]metricTemplate)

	pminfoStatusMetricTemplate = metricTemplate{
		desc:         pminfoPowerSupplyStatusDesc,
		valueType:    prometheus.GaugeValue,
		valueCreator: convertPowerSupplyStatusValue,
	}

	pminfoPowerSupplyStatusDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "pminfo", "power_supply_status"),
		"Power supply status (0=OK, 1=OFF, 2=Failure)",
		[]string{"target", "module"},
		nil,
	)
)

type PminfoCollector struct {
	target   string
	user     string
	password string
}

func init() {
	validatePminfoRegex()

	pminfoMetricTemplates["Status"] = pminfoStatusMetricTemplate
}

func NewPminfoCollector(target string, user string, password string) prometheus.Collector {
	return &PminfoCollector{target, user, password}
}

func (c *PminfoCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collecting pminfo data from target: ", c.target)

	pminfoData, err := util.ExecuteCommand(CmdSmcIpmiTool, c.target, c.user, c.password, "pminfo")

	if err != nil {
		log.Fatal(err)
	}

	metrics := c.ParsePminfoModules(*pminfoData)

	for _, metric := range metrics {
		ch <- metric
	}
}

func (c *PminfoCollector) Describe(ch chan<- *prometheus.Desc) {
}

func (c *PminfoCollector) ParsePminfoModules(data string) []prometheus.Metric {
	slice := make([]prometheus.Metric, 0, 20)

	matchedModules := pminfoModuleRegex.FindAllStringSubmatch(data, -1)

	for _, module := range matchedModules {

		number := module[pminfoModuleNumberIndex]

		items := module[pminfoModuleItemsIndex]

		log.Debug("Module number:", number)

		for _, item := range pminfoItemRegex.FindAllStringSubmatch(items, -1) {

			name := strings.TrimSpace(item[pminfoItemNameIndex])
			value := strings.TrimSpace(item[pminfoItemValueIndex])

			metricTemplate, foundMetric := pminfoMetricTemplates[name]

			if foundMetric {
				slice = append(
					slice,
					prometheus.MustNewConstMetric(
						metricTemplate.desc,
						metricTemplate.valueType,
						metricTemplate.valueCreator(value),
						c.target, number,
					))
			}
		}
	}
	return slice
}

func validatePminfoRegex() {
	if pminfoModuleNumberIndex == -1 {
		panic("Index number not found in pminfoModuleRegex")
	}
	if pminfoModuleItemsIndex == -1 {
		panic("Index items not found in pminfoModuleRegex")
	}
	if pminfoItemNameIndex == -1 {
		panic("Index name not found in pminfoItemRegex")
	}
	if pminfoItemValueIndex == -1 {
		panic("Index value not found in pminfoItemRegex")
	}
}

func convertPowerSupplyStatusValue(value string) float64 {
	if strings.Contains(value, "OK") {
		return 0
	} else if strings.Contains(value, "OFF") {
		return 1
	} else {
		return 2
	}
}
