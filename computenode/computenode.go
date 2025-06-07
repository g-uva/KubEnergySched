package main

import (
	"fmt"
	"log"
	"math/rand"
	httpMetrics "net/http"
	"os"
	// "time"
	"encoding/json"

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

// func simulateJobs() {
// 	for {
// 		sleep := time.Duration(rand.Intn(7)+3) * time.Second
// 		time.Sleep(sleep)

// 		jobCount.Inc()
// 		cpu := 30 + rand.Float64()*60 // 30% to 90%
// 		cpuUsage.Set(cpu)

// 		fmt.Printf("Executed fake job, CPU: %.2f%%\n", cpu)
// 	}
// }

type JobRequest struct {
	ID             string  `json:"id"`
	CPURequirement int     `json:"cpu"`
	EnergyPriority float64 `json:"priority"`
}

func handleJobSubmit(w httpMetrics.ResponseWriter, r *httpMetrics.Request) {
	var job JobRequest
	err := json.NewDecoder(r.Body).Decode(&job)
	if err != nil {
		httpMetrics.Error(w, "Invalid job format", httpMetrics.StatusBadRequest)
		return
	}

	jobCount.Inc()
	cpu := 30 + rand.Float64()*60 // simulate CPU impact
	cpuUsage.Set(cpu)

	fmt.Printf("Job submitted: %+v | CPU now %.2f%%\n", job, cpu)
	w.WriteHeader(httpMetrics.StatusOK)
}

func main() {
	httpMetrics.Handle("/metrics", promhttp.Handler())
	httpMetrics.HandleFunc("/submit", handleJobSubmit)

	portMetrics := "2112"
	if p := os.Getenv("PORT"); p != "" {
		portMetrics = p
	}

	portJob := "9999"
	if p := os.Getenv("JOB_PORT"); p != "" {
		portJob = p
	}

	go func() {
		fmt.Printf("Serving metrics on :%s\n", portMetrics)
		log.Fatal(httpMetrics.ListenAndServe(":"+portMetrics, nil))
	}()

	fmt.Printf("Serving job submission on :%s\n", portJob)
	log.Fatal(httpMetrics.ListenAndServe(":"+portJob, nil))
}
