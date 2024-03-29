// -*- coding: utf-8 -*-
//
// © Copyright 2023 GSI Helmholtzzentrum für Schwerionenforschung
//
// This software is distributed under
// the terms of the GNU General Public Licence version 3 (GPL Version 3),
// copied verbatim in the file "LICENCE".

package collector

import (
	"fmt"
	"prometheus-smcipmi-exporter/util"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	log "github.com/sirupsen/logrus"
)

const (
	PminfoPsuStateOK     = 0.0
	PminfoPsuStateOff    = 1.0
	PminfoPsuStateError  = 2.0
	PminfoPsuStateFaulty = 3.0
)

var (
	pminfoModuleRegex       = regexp.MustCompile(`(?ms:(?:\[Module (?P<number>\d+)\])(?P<items>.*?)(?:^\s?$|\z))`)
	pminfoModuleNumberIndex = pminfoModuleRegex.SubexpIndex("number")
	pminfoModuleItemsIndex  = pminfoModuleRegex.SubexpIndex("items")

	pminfoItemRegex      = regexp.MustCompile(`(?m:(?P<name>(?:\s*[\w/(/)]+\s?)+)\s*\|\s*(?P<value>.*))`)
	pminfoItemNameIndex  = pminfoItemRegex.SubexpIndex("name")
	pminfoItemValueIndex = pminfoItemRegex.SubexpIndex("value")

	pminfoPowerConsumptionRegex      = regexp.MustCompile(`^(?P<value>\d{1,3}) W$`)
	pminfoPowerConsumptionValueIndex = pminfoPowerConsumptionRegex.SubexpIndex("value")

	pminfoMetricTemplates = make(map[string]metricTemplate)

	pminfoPowerSupplyStatusMetricTemplate = metricTemplate{
		desc:         pminfoPowerSupplyStatusDesc,
		valueType:    prometheus.GaugeValue,
		valueCreator: ConvertPowerSupplyStatusValue,
	}

	pminfoPowerConsumptionMetricTemplate = metricTemplate{
		desc:         pminfoPowerConsumptionDesc,
		valueType:    prometheus.GaugeValue,
		valueCreator: ConvertPowerConsumptionValue,
	}

	pminfoPowerSupplyStatusDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "pminfo", "power_supply_status"),
		"Power supply status (0=OK, 1=OFF, 2=Failure, 3=Faulty)",
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
	target   string
	user     string
	password string
}

func init() {
	validatePminfoRegex()

	pminfoMetricTemplates["Status"] = pminfoPowerSupplyStatusMetricTemplate
	pminfoMetricTemplates["Input Power"] = pminfoPowerConsumptionMetricTemplate
}

func NewPminfoCollector(target string, user string, password string) prometheus.Collector {
	return &PminfoCollector{target, user, password}
}

func (c *PminfoCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collecting pminfo module data from target: ", c.target)

	pminfoData, err := util.ExecuteCommandWithSudo(
		CmdSmcIpmiTool, c.target, c.user, c.password, "pminfo")

	if err != nil {
		log.Error(err)
		ch <- createErrorMetric("pminfo", c.target)
		return
	}

	metrics, err := c.CreateMetrics(*pminfoData)

	if err != nil {
		log.Error(err)
		ch <- createErrorMetric("pminfo", c.target)
		return
	}

	for _, metric := range metrics {
		ch <- metric
	}
}

func (c *PminfoCollector) Describe(ch chan<- *prometheus.Desc) {
}

func (c *PminfoCollector) CreateMetrics(data string) ([]prometheus.Metric, error) {
	slice := make([]prometheus.Metric, 0, 20)

	matchedModules := pminfoModuleRegex.FindAllStringSubmatch(data, -1)

	if matchedModules == nil {
		return nil, fmt.Errorf("pminfoModuleRegex missmatch on data:\n%s", data)
	}

	for _, module := range matchedModules {

		number := module[pminfoModuleNumberIndex]
		items := module[pminfoModuleItemsIndex]

		// Create itemMap for O(1) lookup
		itemMap := make(map[string]string)

		for _, item := range pminfoItemRegex.FindAllStringSubmatch(items, -1) {

			name := strings.TrimSpace(item[pminfoItemNameIndex])
			value := strings.TrimSpace(item[pminfoItemValueIndex])

			itemMap[name] = value
		}

		for metricName, metricTemplate := range pminfoMetricTemplates {

			var value string
			var found bool

			// SMCIPMITool might return field `Input Power (DC)`.
			// In the webinterface `AC Input Power` is displayed instead.
			//
			// If more fields have varying names displayed, the processing must be changed
			// e.g. without map lookup and checking if substring is in the fields list.
			if metricName == "Input Power" {
				value, found = itemMap[metricName]
				if !found {
					value, found = itemMap["Input Power (DC)"]
				}
			} else {
				value, found = itemMap[metricName]
			}

			if found {
				val, err := metricTemplate.valueCreator(value)

				if err != nil {
					return nil, err
				} else {
					slice = append(slice,
						prometheus.MustNewConstMetric(
							metricTemplate.desc,
							metricTemplate.valueType,
							val,
							c.target, number, // labelValues
						))
				}
			} else {
				return nil, fmt.Errorf(
					"Metric not found: %s\nInData:\n%s\n", metricName, data)
			}
		}
	}
	return slice, nil
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
	if pminfoPowerConsumptionValueIndex == -1 {
		panic("Index value not found in pminfoPowerConsumptionRegex")
	}
}

func ConvertPowerSupplyStatusValue(value string) (float64, error) {

	if strings.Contains(value, "OK") {
		return PminfoPsuStateOK, nil
	}

	if strings.Contains(value, "FAULT") {
		return PminfoPsuStateError, nil
	}

	trimmedValue := strings.TrimSpace(value)

	if strings.HasPrefix(trimmedValue, "[UNIT IS OFF]") ||
		trimmedValue == "(00h)" {
		return PminfoPsuStateOff, nil
	}

	if strings.HasPrefix(trimmedValue, "[Over Current Fault]") {
		return PminfoPsuStateFaulty, nil
	}

	return -1, fmt.Errorf("Unknown power supply status found: %s", value)
}

func ConvertPowerConsumptionValue(value string) (float64, error) {

	matched := pminfoPowerConsumptionRegex.FindStringSubmatch(value)

	if matched == nil {
		return -1, fmt.Errorf("Regex validation of power consumption failed for value: %s", value)
	}

	powerConsumption, err := strconv.ParseFloat(
		matched[pminfoPowerConsumptionValueIndex], 10)

	if err != nil {
		return -1, err
	}

	return powerConsumption, nil
}
