package prometheus_metrics

import (
	"net/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Creating the Prometheus metrics for the benchmark
var (
	decisionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "benchmark_decision_total",
			Help: "Total number of scheduling decisions made by the benchmark",
		},
		[]string{"strategy", "cluster"},
	)
    energyGauge = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "benchmark_estimated_energy",
            Help: "Estimated energy cost for scheduling decisions.",
        },
        []string{"strategy", "cluster"},
    )
)

func init() {
	prometheus.MustRegister(decisionCounter)
	prometheus.MustRegister(energyGauge)
}

