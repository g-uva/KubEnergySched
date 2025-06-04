package metrics

import (
    benchmark "kube-scheduler/benchmark"
	"fmt"
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

func (ba *benchmark.BenchmarkAdapter) UpdateMetrics() {
    for _, r := range ba.Results {
        decisionCounter.WithLabelValues(r.StrategyName, r.SelectedCluster).Inc()
        energyGauge.WithLabelValues(r.StrategyName, r.SelectedCluster).Set(r.EstimatedCost)
    }
}

func StartPrometheusServer() {
    http.Handle("/metrics", promhttp.Handler())
    go func() {
        fmt.Println("[Prometheus] Starting metrics server on :2112")
        if err := http.ListenAndServe(":2112", nil); err != nil {
            fmt.Println("Error starting Prometheus server:", err)
        }
    }()
}
