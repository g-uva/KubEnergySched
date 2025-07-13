package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"kube-scheduler/ecsched"
	"kube-scheduler/cisched"
	"kube-scheduler/k8sched"
)

// nodes returns the set of cluster nodes
func nodes() []*ecsched.SimulatedNode {
	return []*ecsched.SimulatedNode{
		ecsched.NewNode("n1", 16, 32000, 300),
		ecsched.NewNode("n2", 16, 32000, 300),
	}
}

func loadWorkloads(path string) []ecsched.Workload {
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("Failed to open CSV: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV: %v", err)
	}

	var wls []ecsched.Workload
	start := time.Now()
	for i, r := range rows {
		if i == 0 {
			continue
		}
		ts, _ := strconv.ParseInt(r[0], 10, 64)
		power, _ := strconv.ParseFloat(r[4], 64)

		submit := start.Add(time.Duration(ts) * time.Microsecond)
		wls = append(wls, ecsched.Workload{
			ID:         fmt.Sprintf("w%d", i),
			SubmitTime: submit,
			Duration:   5 * time.Minute,
			CPU:        power * 4,
			Memory:     power * 8192,
		})
	}
	return wls
}

func main() {
	wls := loadWorkloads("powertrace/data/powertrace.csv")
	base := fmt.Sprintf("results/%d_results", time.Now().Unix())
	if err := os.MkdirAll(base, 0755); err != nil {
		log.Fatal(err)
	}

	types := []struct {
		name string
		run  func([]ecsched.Workload) []ecsched.LogEntry
	}{
		{"ecsched_baseline", func(w []ecsched.Workload) []ecsched.LogEntry {
			s := ecsched.NewScheduler(nodes())
			for _, j := range w {
				s.AddWorkload(j)
			}
			s.Run()
			return s.Logs
		}},
		{"ecsched_ci_aware", func(w []ecsched.Workload) []ecsched.LogEntry {
			s := cisched.NewCIScheduler(nodes())
			for _, j := range w { s.AddWorkload(j) }
			s.Run()
			return s.Logs()
		}},
		{"k8_heuristic", func(w []ecsched.Workload) []ecsched.LogEntry {
			s := k8sched.NewK8Simulator(nodes())
			for _, j := range w { s.AddWorkload(j) }
			s.Run()
			return s.Logs()
		}},
	}

	for _, t := range types {
		ents := t.run(wls)
		f, err := os.Create(filepath.Join(base, t.name+".csv"))
		if err != nil {
			log.Fatalf("open output: %v", err)
		}
		w := csv.NewWriter(f)
		w.Write([]string{"job_id","sched","node","submit","start","end","wait_ms"})
		for _, e := range ents {
			wait := e.Start.Sub(e.Submit).Milliseconds()
			w.Write([]string{e.JobID, t.name, e.Node, e.Submit.Format(time.RFC3339Nano), e.Start.Format(time.RFC3339Nano), e.End.Format(time.RFC3339Nano), fmt.Sprint(wait)})
		}
		w.Flush()
		f.Close()
		log.Printf("Wrote %s (%d)", t.name+".csv", len(ents))
	}
}