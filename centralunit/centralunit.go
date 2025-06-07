package main

import (
	"fmt"
	"time"
	"kube-scheduler/pkg/core"
)


func main() {
	fmt.Println("Loading clusters from file...")
	clustersRaw, err := core.LoadClustersFromFile("/config/clusters.json")
	if err != nil {
		fmt.Println("Error loading clusters:", err)
		return
	}
	fmt.Printf("Loaded %d clusters from file\n", len(clustersRaw))

	// Wrap into []core.Cluster interface
	var clusters []core.Cluster
	for _, rc := range clustersRaw {
		clusters = append(clusters, rc)
	}

	unit := core.CentralUnit{
		Clusters: clusters,
		Strategy: &core.RoundRobin{},
	}

	workloads := []core.Workload{
		{ID: "job1", CPURequirement: 4, EnergyPriority: 0.7},
		{ID: "job2", CPURequirement: 2, EnergyPriority: 0.4},
	}

	unit.Dispatch(workloads)

	// To prevent it from exiting
	for {
		time.Sleep(time.Hour)
	}
}