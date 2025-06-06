package computenode

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

/*
This is a temporary fake lightweight node. Just to test the deployment.
*/

// Fake metrics
var (
	jobCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "compute_node_jobs_total",
			Help: "Total number of simulated jobs processed by this compute node.",
		},
	)
	cpuUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "compute_node_cpu_usage",
			Help: "Simulated CPU usage percentage.",
		},
	)
)

func init() {
	prometheus.MustRegister(jobCount)
	prometheus.MustRegister(cpuUsage)
}

func simulateJobs() {
	for {
		sleep := time.Duration(rand.Intn(7)+3) * time.Second
		time.Sleep(sleep)

		jobCount.Inc()
		cpu := 30 + rand.Float64()*60 // 30% to 90%
		cpuUsage.Set(cpu)

		fmt.Printf("Executed fake job, CPU: %.2f%%\n", cpu)
	}
}

func main() {
	go simulateJobs()

	http.Handle("/metrics", promhttp.Handler())
	port := "2112"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	fmt.Printf("Serving metrics on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
