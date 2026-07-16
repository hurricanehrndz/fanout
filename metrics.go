// Copyright (c) 2020 Doc.ai and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fanout

import (
	"github.com/coredns/coredns/plugin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	metricLabelTo    = "to"
	requestCountHelp = "Counter of requests made per upstream."
)

// Variables declared for monitoring.
var (
	RequestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: pluginName,
		Name:      "request_count_total",
		Help:      requestCountHelp,
	}, []string{metricLabelTo})
	RcodeCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: pluginName,
		Name:      "response_rcode_count_total",
		Help:      requestCountHelp,
	}, []string{"rcode", metricLabelTo})
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: pluginName,
		Name:      "request_duration_seconds",
		Buckets:   plugin.TimeBuckets,
		Help:      "Histogram of the time each request took.",
	}, []string{metricLabelTo})
)
