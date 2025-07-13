package main

import (
	"encoding/csv"
	"fmt"
	"kube-scheduler/ecsched"
	"log"
	"os"
	"strconv"
	"time"
)

func _loadWorkloads(path string) []ecsched.Workload {
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

func _main() {
	// Create 100 homogeneous nodes:
	var nodes []*ecsched.SimulatedNode
	for i := 1; i <= 100; i++ {
		name := fmt.Sprintf("n%d", i)
		nodes = append(nodes, &ecsched.SimulatedNode{
			Name:            name,
			AvailableCPU:    16.0,
			AvailableMemory: 32768.0,
		})
	}

	sim := ecsched.NewScheduler(nodes, 0)
	wls := loadWorkloads("powertrace/data/powertrace.csv")

	for _, w := range wls {
		sim.AddWorkload(w)
	}

	// Run the discrete-event simulation / scheduler:
	sim.Run()

	// Save both Logs and CSV into a timestamped subfolder:
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	dir := fmt.Sprintf("ecsched/results/%s_results", ts)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	// 1) write out the schedule_log.csv
	csvOut, err := os.Create(dir + "/schedule_log.csv")
	if err != nil {
		log.Fatalf("open csv: %v", err)
	}
	defer csvOut.Close()

	csvOut.WriteString("job_id,node,submit,start,end\n")
	for _, line := range sim.Logs {
		csvOut.WriteString(line + "\n")
	}

	// 2) write the raw debug log
	logOut, err := os.Create(dir + "/run.log")
	if err != nil {
		log.Fatalf("open log: %v", err)
	}
	defer logOut.Close()
	// // assume sim.DebugLines holds the debug output if I've been collecting it; 
	// // otherwise just replay sim.Logs with timestamps:
	// for _, entry := range sim.Logs {
	// 	logOut.WriteString(entry + "\n")
	// }

	log.Printf("Results written to %s\n", dir)
}
