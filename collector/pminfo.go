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
	"strconv"
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

	pminfoPowerSupplyStatusMetricTemplate = metricTemplate{
		desc:         pminfoPowerSupplyStatusDesc,
		valueType:    prometheus.GaugeValue,
		valueCreator: convertPowerSupplyStatusValue,
	}

	pminfoPowerConsumptionMetricTemplate = metricTemplate{
		desc:         pminfoPowerConsumptionDesc,
		valueType:    prometheus.GaugeValue,
		valueCreator: convertPowerConsumptionValue,
	}

	pminfoPowerSupplyStatusDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "pminfo", "power_supply_status"),
		"Power supply status (0=OK, 1=OFF, 2=Failure)",
		[]string{"target", "module"},
		nil,
	)

	pminfoPowerConsumptionDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "pminfo", "power_consumption_watts"),
		"Current power consumption measured in watts",
		[]string{"target", "module"},
		nil,
	)
)

type PminfoCollector struct {
	targets  []string
	user     string
	password string
}

func init() {
	validatePminfoRegex()

	pminfoMetricTemplates["Status"] = pminfoPowerSupplyStatusMetricTemplate
	pminfoMetricTemplates["Input Power"] = pminfoPowerConsumptionMetricTemplate
}

func NewPminfoCollector(targets []string, user string, password string) prometheus.Collector {
	return &PminfoCollector{targets, user, password}
}

func (c *PminfoCollector) Collect(ch chan<- prometheus.Metric) {

	for _, target := range c.targets {

		log.Debug("Collecting pminfo module data from target: ", target)

		pminfoData, err := util.ExecuteCommandWithSudo(CmdSmcIpmiTool, target, c.user, c.password, "pminfo")

		if err != nil {
			log.Fatal(err)
		}

		for _, metric := range c.createMetrics(target, *pminfoData) {
			ch <- metric
		}
	}

}

func (c *PminfoCollector) Describe(ch chan<- *prometheus.Desc) {
}

func (c *PminfoCollector) createMetrics(target string, data string) []prometheus.Metric {

	slice := make([]prometheus.Metric, 0, 20)

	matchedModules := pminfoModuleRegex.FindAllStringSubmatch(data, -1)

	for _, module := range matchedModules {

		number := module[pminfoModuleNumberIndex]
		items := module[pminfoModuleItemsIndex]

		// Create itemMap for fast O(1) lookup
		itemMap := make(map[string]string)

		for _, item := range pminfoItemRegex.FindAllStringSubmatch(items, -1) {

			name := strings.TrimSpace(item[pminfoItemNameIndex])
			value := strings.TrimSpace(item[pminfoItemValueIndex])

			itemMap[name] = value
		}

		for metricName, metricTemplate := range pminfoMetricTemplates {

			value, found := itemMap[metricName]

			if found {
				slice = append(
					slice,
					prometheus.MustNewConstMetric(
						metricTemplate.desc,
						metricTemplate.valueType,
						metricTemplate.valueCreator(value),
						target, number,
					))
			} else {
				log.Panicln("Metric not found: ", metricName)
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

// TODO: error found during parsing...? -> return (float64, error)
// Valid values: OK, OFF and what else?
func convertPowerSupplyStatusValue(value string) float64 {
	if strings.Contains(value, "OK") {
		return 0
	} else if strings.Contains(value, "OFF") {
		return 1
	} else {
		return 2
	}
}

// TODO: error found during parsing...? -> return (float64, error)
func convertPowerConsumptionValue(value string) float64 {
	slice := strings.Split(value, " ")

	if len(slice) != 2 {
		log.Panicln("Length of Input Power item is invalid: ", value)
	}

	if slice[1] != "W" {
		log.Panicln("Unit in Input Power item is not W: ", value)
	}

	powerConsumption, err := strconv.ParseFloat(slice[0], 10)

	if err != nil {
		log.Panicln(err)
	}

	return powerConsumption
}
