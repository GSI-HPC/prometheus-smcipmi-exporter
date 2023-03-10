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

import "github.com/prometheus/client_golang/prometheus"

const (
	Namespace      = "smcipmi"
	CmdSmcIpmiTool = "SMCIPMITool"
)

// Function signature for NewCollector...
type NewCollectorHandle func(string, string, string) prometheus.Collector

type metricTemplate struct {
	desc         *prometheus.Desc
	valueType    prometheus.ValueType
	valueCreator func(string) float64
}

func createErrorMetric(collector string, target string) prometheus.Metric {
	return prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "collector", "error"),
			"Only set if an error has occurred in a collector",
			[]string{"name", "target"},
			nil,
		),
		prometheus.GaugeValue,
		1,
		collector, target,
	)
}
