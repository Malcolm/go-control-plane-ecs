package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	endpointsDiscovered = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "xds_endpoints_discovered",
		Help: "The current number of endpoints discovered from ECS.",
	})
	snapshotUpdates = promauto.NewCounter(prometheus.CounterOpts{
		Name: "xds_snapshot_updates_total",
		Help: "The total number of snapshot updates.",
	})
	awsAPIErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "xds_aws_api_errors_total",
		Help: "The total number of errors from the AWS API.",
	})
)
