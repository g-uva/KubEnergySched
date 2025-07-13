package main

import (
	"encoding/csv"
	"kube-scheduler/ecsched"
	"log"
	"os"
	"strconv"
	"time"
	"path/filepath"
	"io"
	"fmt"
)

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

	var workloads []ecsched.Workload
	start := time.Now()
	for i, row := range rows {
		if i == 0 {
			continue
		}
		ts, _ := strconv.ParseInt(row[0], 10, 64)
		power, _ := strconv.ParseFloat(row[4], 64)

		submit := start.Add(time.Duration(ts) * time.Microsecond)
		cpu := power * 4.0
		mem := power * 8192.0

		workloads = append(workloads, ecsched.Workload{
			ID:         "w" + strconv.Itoa(i),
			SubmitTime: submit,
			Duration:   5 * time.Minute,
			CPU:        cpu,
			Memory:     mem,
		})
	}
	return workloads
}

func main() {
	// Prepare results directory
	ts := time.Now().Unix()
	dir := fmt.Sprintf("results/%d_results", ts)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Failed to create results dir: %v", err)
	}

	// Redirect logs to both stdout and run.log
	logFilePath := filepath.Join(dir, "run.log")
	lf, err := os.Create(logFilePath)
	if err != nil {
		log.Fatalf("Failed to create log file: %v", err)
	}
	defer lf.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, lf))

	// Load nodes
	nodes := []*ecsched.Node{
		{Name: "n1", TotalCPU: 16.0, TotalMemory: 32000},
		{Name: "n2", TotalCPU: 16.0, TotalMemory: 32000},
	}

	sim := ecsched.NewScheduler(nodes)
	wls := loadWorkloads("powertrace/data/powertrace.csv")

	for _, w := range wls {
		sim.AddWorkload(w)
	}

	sim.Run()

	// Write CSV schedule log
	csvPath := filepath.Join(dir, "schedule_log.csv")
	out, err := os.Create(csvPath)
	if err != nil {
		log.Fatalf("Failed to create CSV: %v", err)
	}
	defer out.Close()

	out.WriteString("job_id,node,submit,start,end\n")
	for _, line := range sim.Logs {
		out.WriteString(line + "\n")
	}

	log.Printf("All outputs saved under %s", dir)
}

