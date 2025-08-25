package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kube-scheduler/models/ecsched"
	"kube-scheduler/models/cisched"
	"kube-scheduler/models/k8sched"
	"kube-scheduler/pkg/generator"
	"kube-scheduler/pkg/loader"
	"kube-scheduler/pkg/core"
)

// parseFloatSlice converts a comma-separated list of floats into a slice
func parseFloatSlice(s string) []float64 {
	parts := strings.Split(s, ",")
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			log.Fatalf("invalid float in slice %q: %v", p, err)
		}
		out = append(out, v)
	}
	return out
}

// parseIntSlice converts a comma-separated list of ints into a slice
func parseIntSlice(s string) []int {
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			log.Fatalf("invalid int in slice %q: %v", p, err)
		}
		out = append(out, v)
	}
	return out
}

func main() {
	var nodesCSV, wlCSV, ciWeightsFlag, batchSizesFlag string
	flag.StringVar(&nodesCSV, "nodes-csv", "", "path to nodes CSV (auto-generate if empty)")
	flag.StringVar(&wlCSV, "wl-csv", "", "path to workloads CSV (auto-generate if empty)")
	flag.StringVar(&ciWeightsFlag, "ci-weights", "0.05,0.1,0.2,0.4", "comma-separated CI-base weights")
	flag.StringVar(&batchSizesFlag, "batch-sizes", "50,100,200", "comma-separated MCFP batch sizes")
	flag.Parse()

	// Auto-generate node and workload CSVs if not provided
	if nodesCSV == "" {
		nodesCSV = "config/nodes.csv"
		if err := generator.GenerateNodes(nodesCSV); err != nil {
			log.Fatalf("node generation failed: %v", err)
		}
	}
	if wlCSV == "" {
		wlCSV = "config/workloads.csv"
		if err := generator.GenerateWorkloads(wlCSV, time.Now().Unix()); err != nil {
			log.Fatalf("workload generation failed: %v", err)
		}
	}

	ciWeights := parseFloatSlice(ciWeightsFlag)
	batchSizes := parseIntSlice(batchSizesFlag)

	// Load workloads once
	wls := loader.LoadWorkloadsFromCSV(wlCSV)

	// Prepare top-level results directory and subfolder for this run
	ts := time.Now().Unix()
	topDir := "results"
	runDir := filepath.Join(topDir, fmt.Sprintf("%d_results", ts))
	if err := os.MkdirAll(runDir, 0755); err != nil {
		log.Fatalf("failed to create run results dir: %v", err)
	}

	// Summary CSV for the sweep
	summaryPath := filepath.Join(topDir, fmt.Sprintf("%d_ci_sweep_summary.csv", ts))
	summaryFile, err := os.Create(summaryPath)
	if err != nil {
		log.Fatalf("failed to create summary CSV: %v", err)
	}
	defer summaryFile.Close()
	summaryWriter := csv.NewWriter(summaryFile)
	defer summaryWriter.Flush()

	// Write summary header
	summaryWriter.Write([]string{
		"ci_weight", "batch_size", "scheduler",
		"avg_wait_s", "avg_runtime_s", "total_ci_cost", "avg_solve_ms",
	})

	// Sweep configurations
	for _, ciW := range ciWeights {
		for _, bs := range batchSizes {
			// Define scheduler specs
			specs := []struct {
				name string
				run  func([]core.Workload) ([]ecsched.LogEntry, float64)
			}{
				// {"baseline", func(w []core.Workload) ([]ecsched.LogEntry, float64) {
				// 	nodes := loader.LoadNodesFromCSV(nodesCSV)
				// 	s := ecsched.NewScheduler(nodes)
				// 	s.ScheduleBatchSize = bs
				// 	s.CIBaseWeight = 0.0
				// 	for _, j := range w {
				// 		s.AddWorkload(j)
				// 	}
				// 	start := time.Now()
				// 	s.Run()
				// 	return s.Logs, float64(time.Since(start).Milliseconds())
				// }},

				{"ci_aware", func(w []core.Workload) ([]ecsched.LogEntry, float64) {
					nodes := loader.LoadNodesFromCSV(nodesCSV)
					s := cisched.NewCIScheduler(nodes)
					s.SetScheduleBatchSize(bs)
					// s.SetCIBaseWeight(ciW)
					for _, j := range w {
						s.AddWorkload(j)
					}
					start := time.Now()
					s.Run()
					return s.Logs(), float64(time.Since(start).Milliseconds())
				}},

				{"k8", func(w []core.Workload) ([]ecsched.LogEntry, float64) {
					nodes := loader.LoadNodesFromCSV(nodesCSV)
					s := k8sched.NewK8Simulator(nodes)
					s.SetScheduleBatchSize(bs)
					// s.SetCIBaseWeight(ciW)
					for _, j := range w {
						s.AddWorkload(j)
					}
					start := time.Now()
					s.Run()
					return s.Logs(), float64(time.Since(start).Milliseconds())
				}},
			}

			// Run each scheduler and record metrics
			for _, spec := range specs {
				logs, solveMs := spec.run(wls)

				// Aggregate summary metrics
				var sumWait, sumRun, sumCI float64
				for _, e := range logs {
					wait := float64(e.WaitMS) / 1000.0
					runDur := e.End.Sub(e.Start).Seconds()
					sumWait += wait
					sumRun += runDur
					sumCI += e.CICost
				}
				n := float64(len(logs))

				// Write summary row
				summaryWriter.Write([]string{
					fmt.Sprintf("%g", ciW),
					fmt.Sprintf("%d", bs),
					spec.name,
					fmt.Sprintf("%.3f", sumWait/n),
					fmt.Sprintf("%.3f", sumRun/n),
					fmt.Sprintf("%.3f", sumCI),
					fmt.Sprintf("%.3f", solveMs/n),
				})

				// Write per-run job-level CSV
				batchFile := filepath.Join(runDir,
					fmt.Sprintf("%d_%s_%.2f_%d_results.csv", ts, spec.name, ciW, bs),
				)
				bf, err := os.Create(batchFile)
				if err != nil {
					log.Fatalf("failed to create batch file %s: %v", batchFile, err)
				}
				runWriter := csv.NewWriter(bf)
				// header with CI cost
				runWriter.Write([]string{"job_id","sched","node","submit","start","end","wait_ms","ci_cost"})
				for _, e := range logs {
					runWriter.Write([]string{
						e.JobID,
						spec.name,
						e.Node,
						e.Submit.Format(time.RFC3339Nano),
						e.Start.Format(time.RFC3339Nano),
						e.End.Format(time.RFC3339Nano),
						fmt.Sprint(e.WaitMS),
						fmt.Sprintf("%.3f", e.CICost),
					})
				}
				runWriter.Flush()
				bf.Close()
				log.Printf("Wrote batch results: %s (jobs=%d)", batchFile, len(logs))

			}
		}
	}

	log.Printf("CI sweep complete; summary in %s; batch results in %s", summaryPath, runDir)
}
