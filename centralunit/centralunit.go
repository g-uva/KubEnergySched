package main

import (
	"fmt"
	"time"
	"encoding/json"
	"os"
	"kube-scheduler/pkg/core"
)



func LoadClustersFromFile(path string) ([]core.RemoteCluster, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var clusters []core.RemoteCluster
	err = json.NewDecoder(file).Decode(&clusters)
	return clusters, err
}

func main() {
	fmt.Println("Loading clusters from file...")
	clustersRaw, err := LoadClustersFromFile("/config/clusters.json")
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