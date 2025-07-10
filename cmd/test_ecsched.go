package main

import (
	"encoding/csv"
	"kube-scheduler/ecsched"
	"log"
	"os"
	"strconv"
	"time"
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
	nodes := []*ecsched.SimulatedNode{
		{Name: "n1"},
		{Name: "n2"},
	}

	sim := ecsched.NewScheduler(nodes)
	wls := loadWorkloads("powertrace/data/powertrace.csv")

	for _, w := range wls {
		sim.AddWorkload(w)
	}
	
	sim.Run()

	out, _ := os.Create("ecsched/results/schedule_log.csv")
	defer out.Close()

	out.WriteString("job_id,node,submit,start,end\n")
	for _, line := range sim.Logs {
		out.WriteString(line + "\n")
	}
	log.Println("Log saved to results/schedule_log.csv")
}
